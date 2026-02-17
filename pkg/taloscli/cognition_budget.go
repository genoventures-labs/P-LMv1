package taloscli

import (
	"strings"

	"github.com/Thynaptic/P-LMv1/pkg/state"
)

type cognitionBudget struct {
	Mode             string
	ComplexityScore  int
	UseThoughtGraph  bool
	UseTreeOfThought bool
	UseMCTS          bool
	HistoryTopK      int
	KnowledgeTopK    int
	MaxLinearModels  int
}

func normalizeCognitionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "minimal", "min", "light":
		return "minimal"
	case "balanced", "standard", "normal":
		return "balanced"
	case "deep", "full":
		return "deep"
	default:
		return "auto"
	}
}

func planCognitionBudget(taskQuery string, sm *state.Manager, modeOverride string) cognitionBudget {
	mode := normalizeCognitionMode(modeOverride)
	if mode == "auto" {
		mode = chooseAutoCognitionMode(taskQuery, sm)
	}
	switch mode {
	case "minimal":
		return cognitionBudget{
			Mode:             mode,
			ComplexityScore:  estimateTaskComplexity(taskQuery, sm),
			UseThoughtGraph:  false,
			UseTreeOfThought: false,
			UseMCTS:          false,
			HistoryTopK:      1,
			KnowledgeTopK:    1,
			MaxLinearModels:  2,
		}
	case "deep":
		return cognitionBudget{
			Mode:             mode,
			ComplexityScore:  estimateTaskComplexity(taskQuery, sm),
			UseThoughtGraph:  true,
			UseTreeOfThought: true,
			UseMCTS:          true,
			HistoryTopK:      4,
			KnowledgeTopK:    4,
			MaxLinearModels:  4,
		}
	default:
		return cognitionBudget{
			Mode:             "balanced",
			ComplexityScore:  estimateTaskComplexity(taskQuery, sm),
			UseThoughtGraph:  false,
			UseTreeOfThought: true,
			UseMCTS:          false,
			HistoryTopK:      2,
			KnowledgeTopK:    2,
			MaxLinearModels:  3,
		}
	}
}

func chooseAutoCognitionMode(taskQuery string, sm *state.Manager) string {
	score := estimateTaskComplexity(taskQuery, sm)
	if score <= 1 {
		return "minimal"
	}
	if score <= 4 {
		return "balanced"
	}
	return "deep"
}

func estimateTaskComplexity(taskQuery string, sm *state.Manager) int {
	q := strings.ToLower(strings.TrimSpace(taskQuery))
	if q == "" {
		return 0
	}
	if isTrivialPrompt(q) {
		return 0
	}

	score := 0
	if len(q) > 60 {
		score++
	}
	if len(q) > 140 {
		score++
	}
	if len(q) > 260 {
		score++
	}
	if strings.Count(q, " and ") >= 1 {
		score++
	}
	if strings.Count(q, ",") >= 2 {
		score++
	}

	for _, marker := range []string{
		"analyze", "analysis", "compare", "tradeoff", "design", "architecture",
		"plan", "strategy", "research", "investigate", "verify", "cross-check",
		"multi-step", "benchmark", "orchestration", "cognition", "delegate",
	} {
		if strings.Contains(q, marker) {
			score += 2
		}
	}

	if sm != nil {
		s := sm.GetSnapshot()
		if s.AnalyticalMode > 0.7 {
			score++
		}
	}
	return score
}
