package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/memory"
	"github.com/Thynaptic/P-LMv1/pkg/state"
	"github.com/ollama/ollama/api"
)

const (
	mapTimeout    = 20 * time.Second
	reduceTimeout = 30 * time.Second
	maxDocGroups  = 6
	maxChunksPer  = 4
)

var (
	ipPattern     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	entityPattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9_.:/-]{2,}\b`)
)

// DocumentOrchestrator coordinates hierarchical reading across indexed documents.
type DocumentOrchestrator struct {
	Memory *memory.MemoryManager
	Client *api.Client
}

// DocGroup contains routed chunks from one source document.
type DocGroup struct {
	SourcePath string
	Segments   []memory.KnowledgeSegment
}

// MapSummary is the stage-1 summary for a routed document group.
type MapSummary struct {
	SourcePath string
	Summary    string
	Entities   []string
}

// CrossLink captures a shared entity between two sources.
type CrossLink struct {
	Entity   string
	SourceA  string
	SourceB  string
	Strength int
}

// DocumentSynthesis is the full orchestration output.
type DocumentSynthesis struct {
	StructuredAnswer string
	MapSummaries     []MapSummary
	CrossLinks       []CrossLink
}

// Orchestrate runs multi-document routing, map-reduce synthesis, and cross-linking.
func (d *DocumentOrchestrator) Orchestrate(query string, modelCandidates []string, topK int) (DocumentSynthesis, error) {
	if d == nil || d.Memory == nil {
		return DocumentSynthesis{}, fmt.Errorf("document orchestrator requires memory manager")
	}
	if strings.TrimSpace(query) == "" {
		return DocumentSynthesis{}, fmt.Errorf("query is empty")
	}
	if topK <= 0 {
		topK = 24
	}

	groups, err := d.routeRelevantSegments(query, topK)
	if err != nil {
		return DocumentSynthesis{}, err
	}
	if len(groups) == 0 {
		return DocumentSynthesis{}, fmt.Errorf("no relevant document segments found")
	}

	maps := d.mapSummaries(query, groups, modelCandidates)
	links := crossLinkSummaries(maps)
	answer := d.reduceSummaries(query, maps, links, modelCandidates)

	return DocumentSynthesis{
		StructuredAnswer: strings.TrimSpace(answer),
		MapSummaries:     maps,
		CrossLinks:       links,
	}, nil
}

func (d *DocumentOrchestrator) routeRelevantSegments(query string, topK int) ([]DocGroup, error) {
	segs, err := d.Memory.RetrieveKnowledgeSegments(query, topK)
	if err != nil {
		return nil, err
	}
	if len(segs) == 0 {
		return nil, nil
	}

	bySource := make(map[string][]memory.KnowledgeSegment)
	for _, s := range segs {
		source := strings.TrimSpace(s.Metadata["source_path"])
		if source == "" {
			source = "knowledge_base:unscoped"
		}
		bySource[source] = append(bySource[source], s)
	}

	type sourceScore struct {
		Source string
		Score  float64
	}
	var ranking []sourceScore
	for source, chunks := range bySource {
		score := 0.0
		for _, c := range chunks {
			score += c.Similarity
		}
		ranking = append(ranking, sourceScore{Source: source, Score: score})
	}
	sort.Slice(ranking, func(i, j int) bool { return ranking[i].Score > ranking[j].Score })
	if len(ranking) > maxDocGroups {
		ranking = ranking[:maxDocGroups]
	}

	var groups []DocGroup
	for _, r := range ranking {
		chunks := bySource[r.Source]
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].Similarity > chunks[j].Similarity })
		if len(chunks) > maxChunksPer {
			chunks = chunks[:maxChunksPer]
		}
		groups = append(groups, DocGroup{
			SourcePath: r.Source,
			Segments:   chunks,
		})
	}
	return groups, nil
}

func (d *DocumentOrchestrator) mapSummaries(query string, groups []DocGroup, modelCandidates []string) []MapSummary {
	out := make([]MapSummary, 0, len(groups))
	for _, g := range groups {
		summary := d.mapSingleSource(query, g, modelCandidates)
		entities := extractEntities(summary)
		if len(entities) == 0 {
			entities = extractEntities(joinSegments(g.Segments))
		}
		out = append(out, MapSummary{
			SourcePath: g.SourcePath,
			Summary:    summary,
			Entities:   entities,
		})
	}
	return out
}

func (d *DocumentOrchestrator) mapSingleSource(query string, g DocGroup, modelCandidates []string) string {
	contextText := joinSegments(g.Segments)
	if strings.TrimSpace(contextText) == "" {
		return "No extractable content for source."
	}

	fallback := "Source " + g.SourcePath + ":\n" + truncate(contextText, 420)
	if d.Client == nil || len(modelCandidates) == 0 {
		return fallback
	}

	system := `You are a document mapper.
Summarize the provided source chunk set into 4-6 concise bullets.
Focus on architecture, responsibilities, interfaces, and constraints.
Do not include chain-of-thought.`
	user := "Query:\n" + query + "\n\nSource: " + g.SourcePath + "\n\nChunks:\n" + contextText

	resp, err := callLLMWithCandidates(d.Client, modelCandidates, []api.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, mapTimeout)
	if err != nil {
		return fallback
	}
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return fallback
	}
	return resp
}

func (d *DocumentOrchestrator) reduceSummaries(query string, summaries []MapSummary, links []CrossLink, modelCandidates []string) string {
	if len(summaries) == 0 {
		return "No relevant documents found."
	}

	var b strings.Builder
	b.WriteString("Mapped Sources:\n")
	for i, s := range summaries {
		fmt.Fprintf(&b, "%d) %s\n%s\n\n", i+1, s.SourcePath, strings.TrimSpace(s.Summary))
	}
	if len(links) > 0 {
		b.WriteString("Cross-links:\n")
		for _, l := range links {
			fmt.Fprintf(&b, "- %s: %s <-> %s\n", l.Entity, l.SourceA, l.SourceB)
		}
	}
	payload := b.String()

	fallback := fallbackReduce(query, summaries, links)
	if d.Client == nil || len(modelCandidates) == 0 {
		return fallback
	}

	system := `You are a reduce-stage synthesizer for hierarchical document reasoning.
Produce a structured response with sections:
1) Architecture Overview
2) Key Components
3) Data/Control Flow
4) Risks or Open Questions
5) Linked Evidence
Be concise and high-signal.`
	user := "User Query:\n" + query + "\n\nInputs:\n" + payload

	resp, err := callLLMWithCandidates(d.Client, modelCandidates, []api.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, reduceTimeout)
	if err != nil {
		return fallback
	}
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return fallback
	}
	return resp
}

func crossLinkSummaries(summaries []MapSummary) []CrossLink {
	type sourceEntity struct {
		Source string
	}

	entitySources := make(map[string][]sourceEntity)
	for _, s := range summaries {
		for _, e := range s.Entities {
			entitySources[e] = append(entitySources[e], sourceEntity{Source: s.SourcePath})
		}
	}

	var out []CrossLink
	seen := make(map[string]bool)
	for entity, refs := range entitySources {
		if len(refs) < 2 {
			continue
		}
		for i := 0; i < len(refs); i++ {
			for j := i + 1; j < len(refs); j++ {
				a, b := refs[i].Source, refs[j].Source
				if a == b {
					continue
				}
				key := entity + "||" + minStr(a, b) + "||" + maxStr(a, b)
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, CrossLink{
					Entity:   entity,
					SourceA:  a,
					SourceB:  b,
					Strength: 1,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Entity == out[j].Entity {
			if out[i].SourceA == out[j].SourceA {
				return out[i].SourceB < out[j].SourceB
			}
			return out[i].SourceA < out[j].SourceA
		}
		return out[i].Entity < out[j].Entity
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func extractEntities(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	entities := make(map[string]bool)

	for _, ip := range ipPattern.FindAllString(text, -1) {
		entities[ip] = true
	}
	for _, ent := range entityPattern.FindAllString(text, -1) {
		n := strings.TrimSpace(ent)
		l := strings.ToLower(n)
		if len(n) < 3 {
			continue
		}
		if l == "http" || l == "https" || l == "json" {
			continue
		}
		entities[n] = true
	}

	var out []string
	for e := range entities {
		out = append(out, e)
	}
	sort.Strings(out)
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func joinSegments(segs []memory.KnowledgeSegment) string {
	var b strings.Builder
	for i, s := range segs {
		fmt.Fprintf(&b, "[chunk %d | sim %.2f | source %s", i+1, s.Similarity, s.Metadata["source_path"])
		if idx := strings.TrimSpace(s.Metadata["chunk_index"]); idx != "" {
			if total := strings.TrimSpace(s.Metadata["chunk_total"]); total != "" {
				fmt.Fprintf(&b, " | %s/%s", idx, total)
			}
		}
		b.WriteString("]\n")
		b.WriteString(strings.TrimSpace(s.Content))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

func fallbackReduce(query string, summaries []MapSummary, links []CrossLink) string {
	var b strings.Builder
	b.WriteString("Architecture Overview\n")
	b.WriteString("- Query: " + strings.TrimSpace(query) + "\n")
	b.WriteString("- Sources analyzed: " + strconv.Itoa(len(summaries)) + "\n\n")

	b.WriteString("Key Components\n")
	for _, s := range summaries {
		b.WriteString("- " + s.SourcePath + ": " + truncate(strings.ReplaceAll(strings.TrimSpace(s.Summary), "\n", " "), 180) + "\n")
	}

	if len(links) > 0 {
		b.WriteString("\nLinked Evidence\n")
		for _, l := range links {
			b.WriteString("- " + l.Entity + " appears in " + l.SourceA + " and " + l.SourceB + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func callLLMWithCandidates(client *api.Client, models []string, messages []api.Message, timeout time.Duration) (string, error) {
	if client == nil || len(models) == 0 {
		return "", fmt.Errorf("missing llm client or model candidates")
	}
	var lastErr error
	for _, model := range models {
		prompt := ""
		if len(messages) > 0 {
			prompt = messages[len(messages)-1].Content
		}
		opts, _ := state.ResolveEntropyOptions(prompt)
		req := &api.ChatRequest{
			Model:    model,
			Options:  opts,
			Messages: messages,
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		var out strings.Builder
		err := client.Chat(ctx, req, func(resp api.ChatResponse) error {
			out.WriteString(resp.Message.Content)
			return nil
		})
		cancel()
		if err == nil {
			return strings.TrimSpace(stripCodeFence(out.String())), nil
		}
		lastErr = err
	}
	return "", lastErr
}

func stripCodeFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") || !strings.HasSuffix(t, "```") {
		return t
	}
	lines := strings.Split(t, "\n")
	if len(lines) < 3 {
		return t
	}
	raw := strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
	if json.Valid([]byte(raw)) {
		return raw
	}
	return raw
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	if n < 4 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func minStr(a, b string) string {
	if a <= b {
		return a
	}
	return b
}

func maxStr(a, b string) string {
	if a >= b {
		return a
	}
	return b
}
