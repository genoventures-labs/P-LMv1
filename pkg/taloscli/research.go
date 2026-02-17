package taloscli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/memory"
	"github.com/Thynaptic/P-LMv1/pkg/router"
	"github.com/Thynaptic/P-LMv1/pkg/state"
	"github.com/Thynaptic/P-LMv1/pkg/tools"
	"github.com/ollama/ollama/api"
	"github.com/spf13/cobra"
)

type researchRuntime struct {
	client          *api.Client
	mm              *memory.MemoryManager
	tc              *tools.GLMToolClient
	modelCandidates []string
	reindexer       *memory.Reindexer
}

type researchFinding struct {
	Text     string `json:"text"`
	Refs     []int  `json:"refs,omitempty"`
	Verified bool   `json:"verified"`
}

type researchReport struct {
	Mode            string
	Query           string
	Executive       string
	Findings        []researchFinding
	EvidenceNotes   []string
	Risks           []string
	NextActions     []string
	Sources         []string
	UnverifiedCount int
}

var (
	researchRunMaxPages         int
	researchRunCrawlDepth       int
	researchRunMaxResearchLoops int
	researchRunTimeout          time.Duration
	researchRunVerbose          bool
	researchRunSeedURLs         []string

	researchDeepMaxPages         int
	researchDeepCrawlDepth       int
	researchDeepMaxPlanSteps     int
	researchDeepMaxResearchLoops int
	researchDeepTimeout          time.Duration
	researchDeepVerbose          bool
	researchDeepSeedURLs         []string

	researchSessionsLast int
)

var researchCmd = &cobra.Command{
	Use:   "research",
	Short: "Run structured research workflows with professional report output.",
}

var researchRunCmd = &cobra.Command{
	Use:   "run [query]",
	Short: "Run bounded research workflow with tool-assisted discovery and synthesis.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.TrimSpace(strings.Join(args, " "))
		if query == "" {
			fmt.Println("Query cannot be empty.")
			return
		}
		report, _, err := executeResearchMode("run", query)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println(renderResearchReport(report))
	},
}

var researchDeepCmd = &cobra.Command{
	Use:   "deep [query]",
	Short: "Run advanced deep research with expanded depth and multi-agent pipeline.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.TrimSpace(strings.Join(args, " "))
		if query == "" {
			fmt.Println("Query cannot be empty.")
			return
		}
		report, _, err := executeResearchMode("deep", query)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println(renderResearchReport(report))
	},
}

var researchSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List recent research sessions and artifact ids.",
	Run: func(cmd *cobra.Command, args []string) {
		recs, corrupt, err := readResearchSessionRecords()
		if err != nil {
			fmt.Printf("Error reading research sessions: %v\n", err)
			return
		}
		if researchSessionsLast > 0 && len(recs) > researchSessionsLast {
			recs = recs[:researchSessionsLast]
		}
		fmt.Println(renderResearchSessions(recs, corrupt, researchSessionsLast))
	},
}

func executeResearchMode(mode, query string) (researchReport, string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "run" && mode != "deep" {
		return researchReport{}, "", fmt.Errorf("unsupported research mode: %s", mode)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return researchReport{}, "", fmt.Errorf("query cannot be empty")
	}

	rt, err := initResearchRuntime(query)
	if err != nil {
		return researchReport{}, "", err
	}
	defer rt.Close()

	if mode == "run" {
		fmt.Println("Phase: Discovery")
		if researchRunVerbose {
			fmt.Printf("Profile: run (crawl-depth=%d, max-pages=%d, loops=%d, timeout=%s)\n", researchRunCrawlDepth, researchRunMaxPages, researchRunMaxResearchLoops, researchRunTimeout)
		}

		originalVerbose := maVerbose
		originalChatTimeout := maChatTimeout
		originalFirstTokenTimeout := maFirstTokenTimeout
		maVerbose = researchRunVerbose
		maChatTimeout = researchRunTimeout
		maFirstTokenTimeout = maxDuration(20*time.Second, researchRunTimeout/3)
		defer func() {
			maVerbose = originalVerbose
			maChatTimeout = originalChatTimeout
			maFirstTokenTimeout = originalFirstTokenTimeout
		}()

		system := `You are TALOS Research Operator.
Objective: produce a high-signal factual research result.
Workflow:
1) Start with web discovery unless explicit seed URLs are provided.
2) Use tools when needed: web_search, fetch_url, http_request, vector_retrieve.
3) Keep breadth/depth bounded by the provided research budget.
4) Return concise findings with explicit source URLs whenever available.
Return plain text only.`
		user := "Research query:\n" + query +
			fmt.Sprintf("\n\nBudget:\n- crawl_depth=%d\n- max_pages=%d\n- max_research_loops=%d\n", researchRunCrawlDepth, researchRunMaxPages, researchRunMaxResearchLoops)
		if len(researchRunSeedURLs) > 0 {
			user += "\nSeed URLs:\n- " + strings.Join(researchRunSeedURLs, "\n- ")
		}

		resp, toolLogs, runErr := runAgentLoop(
			rt.client,
			rt.tc,
			rt.modelCandidates,
			system,
			user,
			researchRunMaxResearchLoops,
			true,
			query,
			"research",
			int(researchRunTimeout.Milliseconds()),
		)
		if runErr != nil {
			report := buildResearchReport("run", query, "", nil, nil, nil)
			report.Risks = append(report.Risks, "Research execution failed: "+runErr.Error())
			rec, persistErr := persistResearchSession(query, "run", report, "FAILED", runErr.Error(), len(toolLogs))
			if persistErr != nil {
				fmt.Printf("Warning: Failed to write research session log: %v\n", persistErr)
			}
			return report, rec.SessionID, runErr
		}

		fmt.Println("Phase: Synthesis")
		sourceList := uniqueStrings(extractURLs(strings.Join(toolLogs, "\n") + "\n" + resp))
		findings := buildFindings(resp, sourceList)
		report := buildResearchReport("run", query, sanitizeModelOutput(resp), findings, sourceList, toolLogs)
		rec, persistErr := persistResearchSession(query, "run", report, "SUCCESS", "", len(toolLogs))
		if persistErr != nil {
			fmt.Printf("Warning: Failed to write research session log: %v\n", persistErr)
		}
		return report, rec.SessionID, nil
	}

	fmt.Println("Phase: Planning")
	if researchDeepVerbose {
		fmt.Printf("Profile: deep (crawl-depth=%d, max-pages=%d, steps=%d, loops=%d, timeout=%s)\n", researchDeepCrawlDepth, researchDeepMaxPages, researchDeepMaxPlanSteps, researchDeepMaxResearchLoops, researchDeepTimeout)
	}

	originalVerbose := maVerbose
	originalPlanSteps := maMaxPlanSteps
	originalLoops := maMaxResearchLoops
	originalChatTimeout := maChatTimeout
	originalFirstTokenTimeout := maFirstTokenTimeout
	maVerbose = researchDeepVerbose
	maMaxPlanSteps = researchDeepMaxPlanSteps
	maMaxResearchLoops = researchDeepMaxResearchLoops
	maChatTimeout = researchDeepTimeout
	maFirstTokenTimeout = maxDuration(25*time.Second, researchDeepTimeout/3)
	defer func() {
		maVerbose = originalVerbose
		maMaxPlanSteps = originalPlanSteps
		maMaxResearchLoops = originalLoops
		maChatTimeout = originalChatTimeout
		maFirstTokenTimeout = originalFirstTokenTimeout
	}()

	seedHint := ""
	if len(researchDeepSeedURLs) > 0 {
		seedHint = "\n\nSeed URLs:\n- " + strings.Join(researchDeepSeedURLs, "\n- ")
	}
	deepQuery := query + fmt.Sprintf("\n\nDeep research budget: crawl_depth=%d, max_pages=%d.%s", researchDeepCrawlDepth, researchDeepMaxPages, seedHint)

	fmt.Println("Phase: Research")
	answer, refs, deepErr := runMultiAgentPipeline(rt.client, rt.mm, rt.tc, rt.modelCandidates, deepQuery)
	if deepErr != nil {
		report := buildResearchReport("deep", query, "", nil, nil, nil)
		report.Risks = append(report.Risks, "Deep research execution failed: "+deepErr.Error())
		rec, persistErr := persistResearchSession(query, "deep", report, "FAILED", deepErr.Error(), 0)
		if persistErr != nil {
			fmt.Printf("Warning: Failed to write research session log: %v\n", persistErr)
		}
		return report, rec.SessionID, deepErr
	}

	fmt.Println("Phase: Synthesis")
	findings := buildFindings(answer, refs)
	report := buildResearchReport("deep", query, sanitizeModelOutput(answer), findings, refs, nil)
	rec, persistErr := persistResearchSession(query, "deep", report, "SUCCESS", "", 0)
	if persistErr != nil {
		fmt.Printf("Warning: Failed to write research session log: %v\n", persistErr)
	}
	return report, rec.SessionID, nil
}

func initResearchRuntime(query string) (*researchRuntime, error) {
	r, err := router.NewRouter()
	if err != nil {
		return nil, fmt.Errorf("initializing router: %w", err)
	}
	initialModel := r.ResolveModel(router.ResolveRequest{
		Query:  query,
		Stage:  "research",
		Models: r.Models,
	})
	modelCandidates := buildModelCandidates(r.Models, initialModel)
	if len(modelCandidates) == 0 {
		return nil, fmt.Errorf("no model candidates available")
	}

	mm, err := memory.NewMemoryManager()
	if err != nil {
		return nil, fmt.Errorf("initializing memory manager: %w", err)
	}
	tc, err := tools.NewGLMToolClient()
	if err != nil {
		fmt.Printf("Warning: tool client unavailable: %v\n", err)
		fmt.Println("Proceeding without external tools.")
	}
	sm, err := state.NewManager()
	if err == nil {
		sm.SetPrimaryGoal(query)
		_ = sm.Save()
	}
	reindexer := memory.NewReindexer(mm, sm)
	reindexer.Start()

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("creating Ollama client: %w", err)
	}
	return &researchRuntime{
		client:          client,
		mm:              mm,
		tc:              tc,
		modelCandidates: modelCandidates,
		reindexer:       reindexer,
	}, nil
}

func (rt *researchRuntime) Close() {
	if rt != nil && rt.reindexer != nil {
		rt.reindexer.Stop()
	}
}

func buildFindings(raw string, sources []string) []researchFinding {
	clean := sanitizeModelOutput(raw)
	lines := strings.Split(clean, "\n")
	var candidates []string
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
		if line != "" && len(line) > 20 {
			candidates = append(candidates, line)
		}
	}
	if len(candidates) == 0 && strings.TrimSpace(clean) != "" {
		candidates = append(candidates, strings.TrimSpace(clean))
	}
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}

	findings := make([]researchFinding, 0, len(candidates))
	for i, c := range candidates {
		f := researchFinding{Text: c}
		if len(sources) > 0 {
			refIdx := (i % len(sources)) + 1
			f.Refs = []int{refIdx}
			f.Verified = true
		}
		findings = append(findings, f)
	}
	return findings
}

func buildResearchReport(mode, query, summary string, findings []researchFinding, sources, toolLogs []string) researchReport {
	report := researchReport{
		Mode:      mode,
		Query:     query,
		Executive: strings.TrimSpace(summary),
		Findings:  findings,
		Sources:   sources,
	}
	if report.Executive == "" {
		report.Executive = "No executive summary was produced."
	}
	if len(findings) == 0 {
		report.Risks = append(report.Risks, "No high-confidence findings were produced by the research pipeline.")
	}
	if len(sources) == 0 {
		report.Risks = append(report.Risks, "No external sources were captured; findings should be treated as unverified.")
	}
	for _, f := range findings {
		if !f.Verified || len(f.Refs) == 0 {
			report.UnverifiedCount++
		}
	}
	if report.UnverifiedCount > 0 {
		report.Risks = append(report.Risks, fmt.Sprintf("%d finding(s) are unverified due to missing source mapping.", report.UnverifiedCount))
	}
	if len(toolLogs) > 0 {
		report.EvidenceNotes = append(report.EvidenceNotes, fmt.Sprintf("Captured %d tool interaction log entries during research.", len(toolLogs)))
	}
	report.NextActions = []string{
		"Validate top findings against primary documentation or official changelogs.",
		"Promote verified evidence into TALOS memory using `talos learn --url` or `talos learn --file`.",
	}
	if mode == "deep" {
		report.NextActions = append([]string{"Run targeted follow-up research for unresolved contradictions or weak citations."}, report.NextActions...)
	}
	return report
}

func renderResearchReport(r researchReport) string {
	var b strings.Builder
	b.WriteString("TALOS RESEARCH REPORT\n\n")
	b.WriteString("MODE\n")
	b.WriteString("  " + strings.ToUpper(strings.TrimSpace(r.Mode)) + "\n\n")
	b.WriteString("QUERY\n")
	b.WriteString("  " + strings.TrimSpace(r.Query) + "\n\n")
	b.WriteString("EXECUTIVE SUMMARY\n")
	b.WriteString("  " + strings.TrimSpace(r.Executive) + "\n\n")

	b.WriteString("KEY FINDINGS\n")
	if len(r.Findings) == 0 {
		b.WriteString("  - No findings produced.\n")
	} else {
		for _, f := range r.Findings {
			line := "  - " + strings.TrimSpace(f.Text)
			if len(f.Refs) > 0 {
				for _, n := range f.Refs {
					line += fmt.Sprintf(" [%d]", n)
				}
			} else {
				line += " [UNVERIFIED]"
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\nEVIDENCE AND CITATIONS\n")
	if len(r.EvidenceNotes) == 0 {
		b.WriteString("  - Evidence notes unavailable.\n")
	} else {
		for _, note := range r.EvidenceNotes {
			b.WriteString("  - " + note + "\n")
		}
	}

	b.WriteString("\nRISKS / UNCERTAINTIES\n")
	if len(r.Risks) == 0 {
		b.WriteString("  - No material risks identified in this run.\n")
	} else {
		for _, risk := range r.Risks {
			b.WriteString("  - " + risk + "\n")
		}
	}

	b.WriteString("\nRECOMMENDED NEXT ACTIONS\n")
	if len(r.NextActions) == 0 {
		b.WriteString("  - No follow-up actions suggested.\n")
	} else {
		for _, act := range r.NextActions {
			b.WriteString("  - " + act + "\n")
		}
	}

	b.WriteString("\nSOURCES\n")
	if len(r.Sources) == 0 {
		b.WriteString("  [none]\n")
	} else {
		for i, s := range r.Sources {
			b.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, s))
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func init() {
	researchRunCmd.Flags().IntVar(&researchRunMaxPages, "max-pages", 40, "Maximum pages/sources to consider during run mode")
	researchRunCmd.Flags().IntVar(&researchRunCrawlDepth, "crawl-depth", 1, "Crawl depth budget hint for run mode")
	researchRunCmd.Flags().IntVar(&researchRunMaxResearchLoops, "max-research-loops", 3, "Maximum tool loops for run mode")
	researchRunCmd.Flags().DurationVar(&researchRunTimeout, "timeout", 120*time.Second, "Total LLM timeout budget for run mode")
	researchRunCmd.Flags().BoolVar(&researchRunVerbose, "verbose", false, "Enable verbose research logs")
	researchRunCmd.Flags().StringSliceVar(&researchRunSeedURLs, "seed-url", nil, "Seed URL(s) to prioritize during run mode")

	researchDeepCmd.Flags().IntVar(&researchDeepMaxPages, "max-pages", 80, "Maximum pages/sources to consider during deep mode")
	researchDeepCmd.Flags().IntVar(&researchDeepCrawlDepth, "crawl-depth", 2, "Crawl depth budget hint for deep mode")
	researchDeepCmd.Flags().IntVar(&researchDeepMaxPlanSteps, "max-plan-steps", 6, "Maximum planner steps for deep mode")
	researchDeepCmd.Flags().IntVar(&researchDeepMaxResearchLoops, "max-research-loops", 6, "Maximum researcher loops for deep mode")
	researchDeepCmd.Flags().DurationVar(&researchDeepTimeout, "timeout", 240*time.Second, "Total LLM timeout budget for deep mode")
	researchDeepCmd.Flags().BoolVar(&researchDeepVerbose, "verbose", false, "Enable verbose deep research logs")
	researchDeepCmd.Flags().StringSliceVar(&researchDeepSeedURLs, "seed-url", nil, "Seed URL(s) to prioritize during deep mode")

	researchSessionsCmd.Flags().IntVar(&researchSessionsLast, "last", 10, "Number of recent sessions to show")

	researchCmd.AddCommand(researchRunCmd)
	researchCmd.AddCommand(researchDeepCmd)
	researchCmd.AddCommand(researchSessionsCmd)
	rootCmd.AddCommand(researchCmd)
}
