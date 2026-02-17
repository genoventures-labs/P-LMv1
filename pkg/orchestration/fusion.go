package orchestration

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/cognition"
	"github.com/Thynaptic/P-LMv1/pkg/memory"
	"github.com/Thynaptic/P-LMv1/pkg/state"
	"github.com/ollama/ollama/api"
)

const (
	fusionTimeout = 25 * time.Second
)

// TechnicalPerspective describes one coherent worldview extracted from evidence.
type TechnicalPerspective struct {
	Name       string
	Summary    string
	Weight     float64
	Sources    []string
	Claims     []string
	Importance float64
}

// PerspectiveConflict describes a contradiction between worldviews.
type PerspectiveConflict struct {
	Issue    string
	Left     string
	Right    string
	Severity float64
}

// ConflictResolution contains weighted winner and rationale.
type ConflictResolution struct {
	Issue     string
	Winner    string
	Rationale string
}

// FusionResult captures staged worldview fusion output.
type FusionResult struct {
	Perspectives   []TechnicalPerspective
	Conflicts      []PerspectiveConflict
	Resolutions    []ConflictResolution
	Epistemic      []cognition.DebateOutcome
	FusedWorldview string
	ConflictIndex  float64
}

// FusionEngine performs multi-stage worldview fusion.
type FusionEngine struct {
	Memory *memory.MemoryManager
	Client *api.Client
}

type worldviewBucket struct {
	name    string
	lines   []string
	sources map[string]bool
	weight  float64
	claims  []string
}

// FuseWorldviews executes extraction -> conflict detection -> weighted resolution -> unified synthesis.
func (f *FusionEngine) FuseWorldviews(query string, research string, modelCandidates []string) (FusionResult, error) {
	if f == nil || f.Memory == nil {
		return FusionResult{}, fmt.Errorf("fusion engine requires memory manager")
	}
	segs, err := f.Memory.RetrieveKnowledgeSegments(query, 48)
	if err != nil {
		return FusionResult{}, err
	}
	segs = FilterEvidenceForFusion(query, research, segs, 14)

	perspectives := ExtractWorldviews(query, research, segs)
	conflicts := detectConflicts(perspectives)
	resolutions := resolveConflictsWeighted(conflicts, perspectives)
	fused := synthesizeUnifiedWorldview(query, perspectives, conflicts, resolutions)
	epOutcomes, _ := cognition.RunEpistemicSelfAlignment(query, research, segs, f.Memory)
	if len(epOutcomes) > 0 {
		fused = strings.TrimSpace(fused) + "\n\nEpistemic Belief Shifts\n"
		for _, o := range epOutcomes {
			fused += fmt.Sprintf("- Winner %s over %s: %s\n", o.WinnerID, o.LoserID, o.Rationale)
			if strings.TrimSpace(o.VerificationNote) != "" {
				fused += fmt.Sprintf("  Verification: %s\n", o.VerificationNote)
			}
		}
	}
	if llm := f.llmFuse(query, perspectives, conflicts, resolutions, modelCandidates); strings.TrimSpace(llm) != "" {
		fused = llm
		if len(epOutcomes) > 0 {
			fused = strings.TrimSpace(fused) + "\n\nEpistemic Belief Shifts\n"
			for _, o := range epOutcomes {
				fused += fmt.Sprintf("- Winner %s over %s: %s\n", o.WinnerID, o.LoserID, o.Rationale)
				if strings.TrimSpace(o.VerificationNote) != "" {
					fused += fmt.Sprintf("  Verification: %s\n", o.VerificationNote)
				}
			}
		}
	}

	return FusionResult{
		Perspectives:   perspectives,
		Conflicts:      conflicts,
		Resolutions:    resolutions,
		Epistemic:      epOutcomes,
		FusedWorldview: fused,
		ConflictIndex:  conflictIndex(conflicts),
	}, nil
}

// ExtractWorldviews identifies at least two technical perspectives from research + evidence.
func ExtractWorldviews(query, research string, segs []memory.KnowledgeSegment) []TechnicalPerspective {
	segs = FilterEvidenceForFusion(query, research, segs, 12)

	legacy := worldviewBucket{name: "Legacy System Reality", sources: make(map[string]bool)}
	desired := worldviewBucket{name: "Desired State Spec", sources: make(map[string]bool)}

	addTo := func(b *worldviewBucket, line string, seg *memory.KnowledgeSegment) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		b.lines = append(b.lines, line)
		b.claims = append(b.claims, line)
		if seg != nil {
			src := strings.TrimSpace(seg.Metadata["source_path"])
			if src != "" {
				b.sources[src] = true
			}
			b.weight += sourceImportance(*seg)
		}
	}

	for _, seg := range segs {
		content := strings.TrimSpace(seg.Content)
		if content == "" {
			continue
		}
		c := strings.ToLower(content)
		switch {
		case hasAny(c, "legacy", "existing", "current", "today", "v1", "old", "as-is"):
			addTo(&legacy, truncateLine(content, 220), &seg)
		case hasAny(c, "desired", "target", "should", "future", "v2", "planned", "to-be", "goal"):
			addTo(&desired, truncateLine(content, 220), &seg)
		default:
			if sourceLooksRuntime(seg.Metadata) {
				addTo(&legacy, truncateLine(content, 220), &seg)
			} else {
				addTo(&desired, truncateLine(content, 220), &seg)
			}
		}
	}

	if strings.TrimSpace(research) != "" {
		qLower := strings.ToLower(strings.TrimSpace(query))
		for _, line := range splitLines(research) {
			l := strings.ToLower(line)
			if looksLikeArtifact(l, "") {
				continue
			}
			rel := lexicalOverlapLocal(qLower, l)
			planLike := hasAny(l,
				"mission plan", "[active]", "[pending]", "[blocked]", "[completed]",
				"define scope", "collect context", "execute implementation", "validate outcome",
				"phase", "task", "dependency", "verify", "analysis", "parser", "binary", "protocol", "chunked",
			)
			if rel < 0.04 && !planLike {
				continue
			}
			switch {
			case hasAny(l, "currently", "today", "existing", "observed", "runtime", "[active]", "current"):
				addTo(&legacy, line, nil)
			case hasAny(l, "should", "recommended", "target", "desired", "proposed", "[pending]", "[blocked]"):
				addTo(&desired, line, nil)
			case planLike && rel >= 0.04:
				// In project-plan outputs, treat concrete active/executed steps as reality; pending path as target.
				if hasAny(l, "active", "completed", "current", "observed") {
					addTo(&legacy, line, nil)
				} else {
					addTo(&desired, line, nil)
				}
			}
		}
	}

	// Ensure minimum two perspectives.
	if len(legacy.lines) == 0 {
		legacy.lines = append(legacy.lines, "Current-state signals were sparse; inferred baseline from available runtime/config evidence.")
	}
	if len(desired.lines) == 0 {
		desired.lines = append(desired.lines, "Desired-state specification was sparse; inferred target from stated project goals and recommendations.")
	}

	p := []TechnicalPerspective{
		toPerspective(legacy),
		toPerspective(desired),
	}
	normalizePerspectiveWeights(p)
	return p
}

func detectConflicts(perspectives []TechnicalPerspective) []PerspectiveConflict {
	if len(perspectives) < 2 {
		return nil
	}
	var out []PerspectiveConflict
	a := perspectives[0]
	b := perspectives[1]
	for _, ca := range a.Claims {
		for _, cb := range b.Claims {
			sev := contradictionHeuristic(ca, cb)
			if sev >= 0.55 {
				out = append(out, PerspectiveConflict{
					Issue:    summarizeIssue(ca, cb),
					Left:     ca,
					Right:    cb,
					Severity: sev,
				})
			}
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Severity > out[j].Severity })
	return out
}

func resolveConflictsWeighted(conflicts []PerspectiveConflict, perspectives []TechnicalPerspective) []ConflictResolution {
	if len(conflicts) == 0 || len(perspectives) < 2 {
		return nil
	}
	leftW := perspectives[0].Weight
	rightW := perspectives[1].Weight
	var out []ConflictResolution

	for _, c := range conflicts {
		winner := perspectives[0].Name
		rationale := fmt.Sprintf("%s prioritized (weight %.2f vs %.2f)", perspectives[0].Name, leftW, rightW)
		if rightW > leftW {
			winner = perspectives[1].Name
			rationale = fmt.Sprintf("%s prioritized (weight %.2f vs %.2f)", perspectives[1].Name, rightW, leftW)
		}
		// Runtime/log evidence gets explicit precedence if present in conflict text.
		lc := strings.ToLower(c.Left + " " + c.Right)
		if hasAny(lc, "log", "runtime", "live", "observed", "vps") {
			if hasAny(strings.ToLower(perspectives[0].Summary), "runtime", "log", "observed", "vps") {
				winner = perspectives[0].Name
				rationale = "Runtime/live evidence overrides stale specification."
			} else if hasAny(strings.ToLower(perspectives[1].Summary), "runtime", "log", "observed", "vps") {
				winner = perspectives[1].Name
				rationale = "Runtime/live evidence overrides stale specification."
			}
		}

		out = append(out, ConflictResolution{
			Issue:     c.Issue,
			Winner:    winner,
			Rationale: rationale,
		})
	}
	return out
}

func synthesizeUnifiedWorldview(query string, perspectives []TechnicalPerspective, conflicts []PerspectiveConflict, resolutions []ConflictResolution) string {
	var b strings.Builder
	b.WriteString("Fused Worldview\n")
	b.WriteString("Query: " + strings.TrimSpace(query) + "\n\n")

	b.WriteString("Stage 1 - Conflict Detection\n")
	if len(conflicts) == 0 {
		b.WriteString("- No major worldview contradictions detected.\n")
	} else {
		for _, c := range conflicts {
			b.WriteString(fmt.Sprintf("- %s (severity %.2f)\n", c.Issue, c.Severity))
		}
	}

	b.WriteString("\nStage 2 - Weighted Resolution\n")
	if len(resolutions) == 0 {
		b.WriteString("- No explicit conflict resolution needed.\n")
	} else {
		for _, r := range resolutions {
			b.WriteString(fmt.Sprintf("- %s -> %s (%s)\n", r.Issue, r.Winner, r.Rationale))
		}
	}

	b.WriteString("\nStage 3 - Unified Synthesis\n")
	for _, p := range perspectives {
		b.WriteString(fmt.Sprintf("- %s (weight %.2f): %s\n", p.Name, p.Weight, p.Summary))
	}
	return strings.TrimSpace(b.String())
}

func (f *FusionEngine) llmFuse(
	query string,
	perspectives []TechnicalPerspective,
	conflicts []PerspectiveConflict,
	resolutions []ConflictResolution,
	modelCandidates []string,
) string {
	if f.Client == nil || len(modelCandidates) == 0 {
		return ""
	}
	var p strings.Builder
	p.WriteString("Query:\n" + query + "\n\n")
	p.WriteString("Perspectives:\n")
	for _, x := range perspectives {
		p.WriteString(fmt.Sprintf("- %s (weight %.2f): %s\n", x.Name, x.Weight, x.Summary))
	}
	p.WriteString("\nConflicts:\n")
	for _, c := range conflicts {
		p.WriteString(fmt.Sprintf("- %s (%.2f)\n", c.Issue, c.Severity))
	}
	p.WriteString("\nResolutions:\n")
	for _, r := range resolutions {
		p.WriteString(fmt.Sprintf("- %s -> %s (%s)\n", r.Issue, r.Winner, r.Rationale))
	}

	system := `You are a worldview fusion engine.
Perform three explicit stages:
1) Conflict Detection
2) Weighted Resolution
3) Unified Synthesis
Use source weighting faithfully. Keep concise and technical.`
	messages := []api.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: p.String()},
	}
	for _, model := range modelCandidates {
		opts, _ := state.ResolveEntropyOptions(p.String())
		req := &api.ChatRequest{
			Model:    model,
			Options:  opts,
			Messages: messages,
		}
		ctx, cancel := context.WithTimeout(context.Background(), fusionTimeout)
		var out strings.Builder
		err := f.Client.Chat(ctx, req, func(resp api.ChatResponse) error {
			out.WriteString(resp.Message.Content)
			return nil
		})
		cancel()
		if err == nil && strings.TrimSpace(out.String()) != "" {
			return strings.TrimSpace(out.String())
		}
	}
	return ""
}

func toPerspective(b worldviewBucket) TechnicalPerspective {
	var srcs []string
	for s := range b.sources {
		srcs = append(srcs, s)
	}
	sort.Strings(srcs)
	summary := strings.Join(uniqueLines(b.lines, 4), " ")
	return TechnicalPerspective{
		Name:       b.name,
		Summary:    strings.TrimSpace(summary),
		Weight:     b.weight,
		Sources:    srcs,
		Claims:     uniqueLines(b.claims, 8),
		Importance: b.weight,
	}
}

func normalizePerspectiveWeights(p []TechnicalPerspective) {
	total := 0.0
	for i := range p {
		if p[i].Weight <= 0 {
			p[i].Weight = 0.5
		}
		total += p[i].Weight
	}
	if total <= 0 {
		total = float64(len(p))
	}
	for i := range p {
		p[i].Weight = p[i].Weight / total
	}
}

func sourceImportance(seg memory.KnowledgeSegment) float64 {
	meta := seg.Metadata
	imp := parseMetaFloatLocal(meta["base_importance"], 0.5)
	fresh := 0.5
	if t, ok := parseTimeLocal(meta["timestamp"]); ok {
		ageH := time.Since(t).Hours()
		fresh = math.Pow(2, -ageH/(24*7)) // half-life 7 days for source freshness
	}
	score := (imp * 0.6) + (fresh * 0.4)
	if sourceLooksRuntime(meta) {
		score += 0.15
	}
	if score > 1 {
		score = 1
	}
	if score < 0 {
		score = 0
	}
	return score
}

func sourceLooksRuntime(meta map[string]string) bool {
	s := strings.ToLower(strings.TrimSpace(meta["source_path"] + " " + meta["source_type"]))
	return hasAny(s, "log", "runtime", "journal", "vps", "status", "health", "metrics")
}

func contradictionHeuristic(a, b string) float64 {
	la := strings.ToLower(strings.TrimSpace(a))
	lb := strings.ToLower(strings.TrimSpace(b))
	if la == "" || lb == "" {
		return 0
	}
	shared := lexicalOverlapLocal(la, lb)
	negA := hasAny(" "+la+" ", " not ", " no ", " never ", "disabled", "fails", "unavailable")
	negB := hasAny(" "+lb+" ", " not ", " no ", " never ", "enabled", "works", "available", "active")
	if shared >= 0.45 && negA != negB {
		return 0.85
	}
	if shared >= 0.7 {
		return 0.20
	}
	if shared < 0.12 {
		return 0.05
	}
	return 0.40
}

func summarizeIssue(a, b string) string {
	return truncateLine(a, 80) + "  <>  " + truncateLine(b, 80)
}

func conflictIndex(conflicts []PerspectiveConflict) float64 {
	if len(conflicts) == 0 {
		return 0
	}
	sum := 0.0
	for _, c := range conflicts {
		sum += c.Severity
	}
	score := sum / float64(len(conflicts))
	if score > 1 {
		return 1
	}
	return score
}

func parseMetaFloatLocal(v string, fallback float64) float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func parseTimeLocal(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func hasAny(s string, markers ...string) bool {
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var out []string
	for _, r := range raw {
		r = strings.TrimSpace(strings.TrimLeft(r, "-*0123456789. "))
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func uniqueLines(in []string, limit int) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func truncateLine(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	if n < 4 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func lexicalOverlapLocal(a, b string) float64 {
	ta := tokenizeLocal(a)
	tb := tokenizeLocal(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(ta))
	for _, t := range ta {
		setA[t] = true
	}
	shared := 0
	for _, t := range tb {
		if setA[t] {
			shared++
		}
	}
	den := len(ta)
	if len(tb) > den {
		den = len(tb)
	}
	return float64(shared) / float64(den)
}

// FilterEvidenceForFusion removes low-trust/low-relevance memory artifacts before worldview synthesis.
func FilterEvidenceForFusion(query, research string, segs []memory.KnowledgeSegment, maxSegments int) []memory.KnowledgeSegment {
	if len(segs) == 0 {
		return nil
	}
	if maxSegments <= 0 {
		maxSegments = 12
	}

	q := strings.ToLower(strings.TrimSpace(query))
	r := strings.ToLower(strings.TrimSpace(truncateLine(research, 1400)))
	type scored struct {
		seg   memory.KnowledgeSegment
		score float64
	}
	scoredSegs := make([]scored, 0, len(segs))

	for _, s := range segs {
		content := strings.TrimSpace(s.Content)
		if content == "" {
			continue
		}
		lc := strings.ToLower(content)
		src := strings.ToLower(strings.TrimSpace(s.Metadata["source_path"] + " " + s.Metadata["source_type"]))
		runtimeLike := sourceLooksRuntime(s.Metadata)

		if looksLikeArtifact(lc, src) {
			// Never let known synthetic/test artifacts dominate technical fusion.
			if !runtimeLike {
				continue
			}
		}

		relevance := lexicalOverlapLocal(q, lc)
		if r != "" {
			relevance = math.Max(relevance, lexicalOverlapLocal(r, lc))
		}
		if hasAny(src, "readme", "guide", "doc", "spec", "log", "runtime", "vps", "config", "kb", "knowledge", "file") {
			relevance += 0.06
		}
		if hasAny(src, "/tmp/", "rag_test", "test", "fixture") {
			relevance -= 0.20
		}
		if hasAny(strings.ToLower(s.Metadata["source_type"]), "strategy", "deliberation", "agency") {
			relevance -= 0.08
		}
		relevance = math.Max(0, math.Min(1, relevance))

		quality := (s.Similarity * 0.35) + (sourceImportance(s) * 0.35) + (relevance * 0.30)
		if runtimeLike {
			quality += 0.08
		}
		if hasAny(q, "binary", "parser", "protocol") && hasAny(lc, "binary", "parser", "protocol", "hex", "offset") {
			quality += 0.10
		}
		if looksLikeArtifact(lc, src) {
			quality -= 0.45
		}

		// Keep very low-score items only when they are live/runtime signals.
		if quality < 0.18 && !runtimeLike {
			continue
		}
		scoredSegs = append(scoredSegs, scored{
			seg:   s,
			score: quality,
		})
	}

	sort.SliceStable(scoredSegs, func(i, j int) bool {
		return scoredSegs[i].score > scoredSegs[j].score
	})
	if len(scoredSegs) == 0 {
		// Fallback: keep top semantic non-artifact items to avoid empty worldview extraction.
		for i := 0; i < len(segs) && i < maxSegments; i++ {
			lc := strings.ToLower(strings.TrimSpace(segs[i].Content))
			src := strings.ToLower(strings.TrimSpace(segs[i].Metadata["source_path"] + " " + segs[i].Metadata["source_type"]))
			if looksLikeArtifact(lc, src) {
				continue
			}
			scoredSegs = append(scoredSegs, scored{seg: segs[i], score: segs[i].Similarity})
		}
	}
	if len(scoredSegs) > maxSegments {
		scoredSegs = scoredSegs[:maxSegments]
	}

	out := make([]memory.KnowledgeSegment, 0, len(scoredSegs))
	for _, x := range scoredSegs {
		out = append(out, x.seg)
	}
	return out
}

func looksLikeArtifact(contentLower, sourceLower string) bool {
	if hasAny(contentLower,
		"secret code", "favorite fruit", "bitcoin is a digital asset", "release checklist",
		"peach-fuzz", "mangosteen",
	) {
		return true
	}
	if hasAny(sourceLower, "/tmp/", "rag_test", "fixture", "dummy", "example-notes") {
		return true
	}
	return false
}

func tokenizeLocal(s string) []string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(",", " ", ".", " ", ":", " ", ";", " ", "(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ", "\"", " ")
	s = repl.Replace(s)
	var out []string
	for _, w := range strings.Fields(s) {
		if len(w) < 3 {
			continue
		}
		out = append(out, w)
	}
	return out
}
