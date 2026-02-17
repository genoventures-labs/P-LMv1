package taloscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/cognition"
	"github.com/Thynaptic/P-LMv1/pkg/memory"
	"github.com/Thynaptic/P-LMv1/pkg/router"
	"github.com/Thynaptic/P-LMv1/pkg/state"
	"github.com/Thynaptic/P-LMv1/pkg/tools"
	"github.com/ollama/ollama/api"
	"github.com/spf13/cobra"
)

var (
	maMaxPlanSteps     int
	maMaxResearchLoops int
	maVerbose          bool
)

const (
	maDefaultFirstTokenTimeout = 75 * time.Second
	maDefaultChatTimeout       = 180 * time.Second
	maFinalLatencyBudgetMS     = 45000
)

var (
	maFirstTokenTimeout = durationFromEnv("PLM_AGENT_FIRST_TOKEN_TIMEOUT", maDefaultFirstTokenTimeout)
	maChatTimeout       = durationFromEnv("PLM_AGENT_CHAT_TIMEOUT", maDefaultChatTimeout)
)

var multiAgentCmd = &cobra.Command{
	Use:   "multi-agent [query]",
	Short: "Run an extended multi-agent pipeline (planner -> researcher -> verifier -> synthesizer).",
	Long:  `Runs a Perplexity-style multi-agent pipeline with tool arbitration, source extraction, verification, and synthesis.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please provide a query. Example: talos multi-agent \"What changed in X this week?\"")
			return
		}
		query := strings.TrimSpace(strings.Join(args, " "))
		if query == "" {
			fmt.Println("Query cannot be empty.")
			return
		}

		r, err := router.NewRouter()
		if err != nil {
			fmt.Printf("Error initializing router: %v\n", err)
			return
		}
		initialModel := r.ResolveModel(router.ResolveRequest{
			Query:  query,
			Stage:  "plan",
			Models: r.Models,
		})
		modelCandidates := buildModelCandidates(r.Models, initialModel)
		if len(modelCandidates) == 0 {
			fmt.Println("No model candidates available.")
			return
		}

		mm, err := memory.NewMemoryManager()
		if err != nil {
			fmt.Printf("Error initializing memory manager: %v\n", err)
			return
		}
		sm, err := state.NewManager()
		if err == nil {
			sm.SetPrimaryGoal(query)
			_ = sm.Save()
		}
		reindexer := memory.NewReindexer(mm, sm)
		reindexer.Start()
		defer reindexer.Stop()

		tc, err := tools.NewGLMToolClient()
		if err != nil {
			fmt.Printf("Warning: tool client unavailable: %v\n", err)
			fmt.Println("Proceeding without external tools.")
		}

		client, err := api.ClientFromEnvironment()
		if err != nil {
			fmt.Printf("Error creating Ollama client: %v\n", err)
			return
		}

		if err := mm.AddMessage("user", query); err != nil {
			fmt.Printf("Warning: Failed to store user query in memory: %v\n", err)
		}

		answer, refs, err := runMultiAgentPipeline(client, mm, tc, modelCandidates, query)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Println(answer)
		if len(refs) > 0 {
			fmt.Println("\nSources:")
			for i, r := range refs {
				fmt.Printf("[%d] %s\n", i+1, r)
			}
		}

		if err := mm.AddMessage("assistant", answer); err != nil {
			fmt.Printf("Warning: Failed to store assistant answer in memory: %v\n", err)
		}
	},
}

type evidenceRecord struct {
	Step      string
	Summary   string
	ToolLogs  []string
	SourceURL []string
}

func runMultiAgentPipeline(
	client *api.Client,
	mm *memory.MemoryManager,
	tc *tools.GLMToolClient,
	modelCandidates []string,
	query string,
) (string, []string, error) {
	knowledgeCtx, _ := mm.RetrieveKnowledge(query, 4)
	historyCtx, _ := mm.RetrieveContext(query, 3)

	plannerInput := "User query:\n" + query
	if len(knowledgeCtx) > 0 {
		plannerInput += "\n\nRelevant knowledge:\n- " + strings.Join(knowledgeCtx, "\n- ")
	}
	if len(historyCtx) > 0 {
		plannerInput += "\n\nRelevant history:\n- " + strings.Join(historyCtx, "\n- ")
	}

	plannerSystem := `You are the Planner agent.
Break the user task into concrete research steps.
Return JSON only in this format:
{"steps":["step 1","step 2","step 3"]}`

	plannerResp, _, err := runAgentLoop(client, tc, modelCandidates, plannerSystem, plannerInput, 1, false, query, "plan", 0)
	if err != nil {
		return "", nil, fmt.Errorf("planner failed: %w", err)
	}
	steps := parsePlanSteps(plannerResp, query, maMaxPlanSteps)
	if len(steps) == 0 {
		steps = []string{query}
	}

	if maVerbose {
		fmt.Printf("DEBUG: Planner created %d step(s).\n", len(steps))
		for i, s := range steps {
			fmt.Printf("DEBUG:   %d) %s\n", i+1, s)
		}
	}

	var evidence []evidenceRecord
	allSources := make(map[string]bool)

	researchSystem := `You are the Researcher agent.
You may call tools to gather evidence.
Prefer web_search for discovery, fetch_url for page content, http_request for APIs, vector_retrieve for semantic retrieval.
When done, provide a concise factual summary with source domains if available.`

	for i, step := range steps {
		if maVerbose {
			fmt.Printf("DEBUG: Research step %d/%d\n", i+1, len(steps))
		}
		researchResp, toolLogs, runErr := runAgentLoop(
			client, tc, modelCandidates,
			researchSystem,
			"Research step:\n"+step+"\n\nOriginal query:\n"+query,
			maMaxResearchLoops,
			true,
			query,
			"research",
			0,
		)
		if runErr != nil {
			researchResp = "Research step failed: " + runErr.Error()
		}

		srcs := uniqueStrings(extractURLs(strings.Join(toolLogs, "\n") + "\n" + researchResp))
		for _, s := range srcs {
			allSources[s] = true
		}

		evidence = append(evidence, evidenceRecord{
			Step:      step,
			Summary:   sanitizeModelOutput(researchResp),
			ToolLogs:  toolLogs,
			SourceURL: srcs,
		})
	}

	verifierSystem := `You are the Verifier agent.
Check consistency of collected evidence, identify conflicts/gaps, and produce concise verification notes.
Return plain text.`
	symbolicAudit := buildSymbolicSiblingAudit(query, evidence, knowledgeCtx)
	verifierInput := "Original query:\n" + query +
		"\n\nEvidence:\n" + formatEvidenceForVerifier(evidence) +
		"\n\nSymbolic consistency audit:\n" + symbolicAudit
	verifierResp, _, err := runAgentLoop(client, tc, modelCandidates, verifierSystem, verifierInput, 1, false, query, "verify", 0)
	if err != nil {
		verifierResp = "Verifier unavailable: " + err.Error()
	}

	sourceList := mapKeysSorted(allSources)
	synthSystem := `You are the Synthesizer agent.
Write the final answer using evidence and verifier notes.
Use concise, high-signal prose.
If sources are provided, cite with [n] markers that map to the numbered source list.`
	synthInput := "Query:\n" + query +
		"\n\nEvidence:\n" + formatEvidenceForSynth(evidence, sourceList) +
		"\n\nSymbolic consistency audit:\n" + symbolicAudit +
		"\n\nVerifier notes:\n" + verifierResp +
		"\n\nProvide final answer now."

	finalResp, _, err := runAgentLoop(client, tc, modelCandidates, synthSystem, synthInput, 1, false, query, "final", maFinalLatencyBudgetMS)
	if err != nil {
		return fallbackSynthesisFromEvidence(query, evidence, sourceList), sourceList, nil
	}

	finalAnswer := sanitizeModelOutput(finalResp)
	if strings.TrimSpace(finalAnswer) == "" {
		finalAnswer = "I couldn't synthesize a final answer from the current evidence."
	}
	return finalAnswer, sourceList, nil
}

func runAgentLoop(
	client *api.Client,
	tc *tools.GLMToolClient,
	modelCandidates []string,
	systemPrompt string,
	userPrompt string,
	maxLoops int,
	allowTools bool,
	taskQuery string,
	stage string,
	maxLatencyMS int,
) (string, []string, error) {
	resolvedCandidates := modelCandidates
	if resolved, err := router.ResolveRemote(router.ResolveRequest{
		Query:        taskQuery,
		Stage:        stage,
		MaxLatencyMS: maxLatencyMS,
		Models:       modelCandidates,
	}); err == nil && strings.TrimSpace(resolved) != "" {
		resolvedCandidates = buildModelCandidates(modelCandidates, resolved)
	}

	messages := []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	var toolLogs []string

	for i := 0; i < maxInt(maxLoops, 1); i++ {
		resp, modelUsed, err := callLLMWithFallback(client, resolvedCandidates, messages)
		if err != nil {
			return "", toolLogs, err
		}
		if maVerbose {
			fmt.Printf("DEBUG: Agent call %d used model %s\n", i+1, modelUsed)
		}

		clean := sanitizeModelOutput(resp)
		if !allowTools || tc == nil {
			return clean, toolLogs, nil
		}

		toolCalls, hasTools := parseToolCalls(resp)
		if !hasTools {
			return clean, toolLogs, nil
		}
		if len(toolCalls) > maxToolCallsPerTurn {
			toolCalls = toolCalls[:maxToolCallsPerTurn]
		}

		messages = append(messages, api.Message{Role: "assistant", Content: resp})

		for _, raw := range toolCalls {
			call, note := arbitrateToolCall(raw, taskQuery)
			if maVerbose && note != "" {
				fmt.Printf("DEBUG: Tool arbitration: %s\n", note)
			}
			out, err := executeToolCall(tc, call)
			if err != nil {
				out = "Error executing tool: " + err.Error()
			}
			toolLogs = append(toolLogs, out)
			msg := "Tool result (" + call.Tool + "): " + truncateForModel(out) + "\nContinue."
			messages = append(messages, api.Message{Role: "user", Content: msg})
		}
	}

	return "Reached agent loop limit without final response.", toolLogs, nil
}

func callLLMWithFallback(client *api.Client, modelCandidates []string, messages []api.Message) (string, string, error) {
	if len(modelCandidates) == 0 {
		return "", "", fmt.Errorf("no model candidates available")
	}

	var lastErr error
	for _, modelName := range modelCandidates {
		resp, err := callLLMOnce(client, modelName, messages)
		if err == nil {
			return resp, modelName, nil
		}
		lastErr = err
		if maVerbose {
			fmt.Printf("DEBUG: Model %s failed: %v\n", modelName, err)
		}
		if !isRetryableAgentError(err) {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("all model candidates failed: %w", lastErr)
}

func isRetryableAgentError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if isTransientLLMError(err) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "first-token timeout")
}

func callLLMOnce(client *api.Client, modelName string, messages []api.Message) (string, error) {
	prompt := ""
	if len(messages) > 0 {
		prompt = messages[len(messages)-1].Content
	}
	opts, entropy := state.ResolveEntropyOptions(prompt)
	req := &api.ChatRequest{
		Model:    modelName,
		Options:  opts,
		Messages: messages,
	}
	if maVerbose {
		fmt.Printf("DEBUG: Entropy mode=%s temperature=%.2f top_p=%.2f\n", entropy.Mode, entropy.Temperature, entropy.TopP)
	}

	totalCtx, totalCancel := context.WithTimeout(context.Background(), maChatTimeout)
	defer totalCancel()
	ctx, cancel := context.WithCancel(totalCtx)
	defer cancel()

	var out strings.Builder
	var sawFirstToken atomic.Bool
	var firstTokenTimeout atomic.Bool

	timer := time.AfterFunc(maFirstTokenTimeout, func() {
		if !sawFirstToken.Load() {
			firstTokenTimeout.Store(true)
			cancel()
		}
	})
	defer timer.Stop()

	err := client.Chat(ctx, req, func(resp api.ChatResponse) error {
		if !sawFirstToken.Load() {
			sawFirstToken.Store(true)
			timer.Stop()
		}
		out.WriteString(resp.Message.Content)
		return nil
	})
	if err != nil {
		if firstTokenTimeout.Load() && errors.Is(ctx.Err(), context.Canceled) {
			return "", fmt.Errorf("first-token timeout after %s", maFirstTokenTimeout)
		}
		return "", err
	}

	return out.String(), nil
}

func parsePlanSteps(raw, fallback string, maxSteps int) []string {
	type plan struct {
		Steps []string `json:"steps"`
	}

	trimmed := strings.TrimSpace(stripMarkdownCodeFences(stripReasoningSections(raw)))
	var p plan
	if err := json.Unmarshal([]byte(trimmed), &p); err == nil && len(p.Steps) > 0 {
		return clampSteps(p.Steps, maxSteps)
	}

	lines := strings.Split(trimmed, "\n")
	var steps []string
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
		if line != "" {
			steps = append(steps, line)
		}
	}
	steps = clampSteps(steps, maxSteps)
	if len(steps) > 0 {
		return steps
	}
	return clampSteps([]string{fallback}, maxSteps)
}

func clampSteps(steps []string, maxSteps int) []string {
	var out []string
	seen := make(map[string]bool)
	for _, s := range steps {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) >= maxInt(maxSteps, 1) {
			break
		}
	}
	return out
}

func formatEvidenceForVerifier(ev []evidenceRecord) string {
	var b strings.Builder
	for i, e := range ev {
		fmt.Fprintf(&b, "%d) Step: %s\nSummary: %s\n", i+1, e.Step, e.Summary)
		if len(e.SourceURL) > 0 {
			fmt.Fprintf(&b, "Sources: %s\n", strings.Join(e.SourceURL, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatEvidenceForSynth(ev []evidenceRecord, sources []string) string {
	index := make(map[string]int)
	for i, s := range sources {
		index[s] = i + 1
	}

	var b strings.Builder
	for i, e := range ev {
		fmt.Fprintf(&b, "%d) %s\n", i+1, e.Summary)
		if len(e.SourceURL) > 0 {
			var refs []string
			for _, s := range e.SourceURL {
				if n, ok := index[s]; ok {
					refs = append(refs, fmt.Sprintf("[%d]", n))
				}
			}
			if len(refs) > 0 {
				fmt.Fprintf(&b, "Refs: %s\n", strings.Join(uniqueStrings(refs), " "))
			}
		}
	}

	if len(sources) > 0 {
		b.WriteString("\nNumbered sources:\n")
		for i, s := range sources {
			fmt.Fprintf(&b, "[%d] %s\n", i+1, s)
		}
	}
	return b.String()
}

func extractURLs(s string) []string {
	re := regexp.MustCompile(`https?://[^\s"\\]+`)
	return uniqueStrings(re.FindAllString(s, -1))
}

func truncateForModel(s string) string {
	if len(s) <= maxToolResultChars {
		return s
	}
	return s[:maxToolResultChars] + "\n...(truncated for context size)"
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func mapKeysSorted(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func fallbackSynthesisFromEvidence(query string, ev []evidenceRecord, sources []string) string {
	var first string
	for _, e := range ev {
		if strings.TrimSpace(e.Summary) != "" {
			first = strings.TrimSpace(e.Summary)
			break
		}
	}
	if first == "" {
		first = "I could not fully synthesize the final answer due model latency, but research steps completed."
	}
	answer := "Query: " + query + "\n\nBest available answer:\n" + first
	if len(sources) > 0 {
		answer += "\n\nSources:\n"
		for i, s := range sources {
			answer += fmt.Sprintf("[%d] %s\n", i+1, s)
		}
	}
	return answer
}

func buildSymbolicSiblingAudit(query string, evidence []evidenceRecord, knowledgeCtx []string) string {
	_ = query
	var findings []string

	for i := 0; i < len(evidence); i++ {
		for j := i + 1; j < len(evidence); j++ {
			a := strings.TrimSpace(evidence[i].Summary)
			b := strings.TrimSpace(evidence[j].Summary)
			if a == "" || b == "" {
				continue
			}
			score := cognition.DetectContradiction(a, b)
			if score >= 0.7 {
				findings = append(findings, fmt.Sprintf(
					"- Potential contradiction (%.2f) between step %d and step %d",
					score, i+1, j+1,
				))
			}
		}
	}

	for i, ev := range evidence {
		if strings.TrimSpace(ev.Summary) == "" {
			continue
		}
		score := cognition.EvaluateLogic(ev.Summary, knowledgeCtx)
		if score >= 0.7 {
			findings = append(findings, fmt.Sprintf(
				"- Step %d appears to conflict with known context (%.2f)",
				i+1, score,
			))
		}
	}

	if len(findings) == 0 {
		return "No strong symbolic contradictions detected across sibling candidates."
	}
	return "Detected potential contradictions:\n" + strings.Join(findings, "\n")
}

func init() {
	multiAgentCmd.Flags().IntVar(&maMaxPlanSteps, "max-plan-steps", 4, "Maximum planner decomposition steps")
	multiAgentCmd.Flags().IntVar(&maMaxResearchLoops, "max-research-loops", 3, "Maximum researcher tool loops per step")
	multiAgentCmd.Flags().BoolVar(&maVerbose, "verbose", false, "Enable verbose pipeline logs")
	rootCmd.AddCommand(multiAgentCmd)
}
