package cognition

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

const (
	defaultPruneThreshold = 0.30
	defaultUCB1C          = 1.25
)

// ThoughtNode is a branch-capable node used in MCTS thought search.
type ThoughtNode struct {
	ID          string
	Answer      string
	Depth       int
	Parent      *ThoughtNode
	Children    []*ThoughtNode
	Visits      int
	ValueSum    float64
	Score       float64
	Confidence  float64
	Pruned      bool
	PruneReason string
}

// MCTSConfig controls selection/expansion/pruning behavior.
type MCTSConfig struct {
	Iterations     int
	BranchFactor   int
	RolloutDepth   int
	UCB1C          float64
	PruneThreshold float64
	Seed           int64
}

// MCTSEvaluation captures weighted score components for a branch.
type MCTSEvaluation struct {
	Confidence float64
	Candidate  string
	Reason     string
}

// MCTSCallbacks provides external model/tool hooks used by search.
type MCTSCallbacks struct {
	ProposeBranches func(ctx context.Context, currentAnswer string, branchCount int) ([]string, error)
	EvaluatePath    func(ctx context.Context, candidate string) (MCTSEvaluation, error)
	AdversarialEval func(ctx context.Context, candidate string) (MCTSEvaluation, error)
}

// MCTSEngine runs branch search and backpropagation to choose a winning path.
type MCTSEngine struct {
	Config    MCTSConfig
	Callbacks MCTSCallbacks
}

// Search executes MCTS over potential logic paths and returns the best synthesized answer.
func (e *MCTSEngine) Search(ctx context.Context, draftAnswer string) (string, bool, *ThoughtNode, error) {
	if strings.TrimSpace(draftAnswer) == "" {
		draftAnswer = "No draft answer yet."
	}
	cfg := e.normalizedConfig()
	if e.Callbacks.ProposeBranches == nil || e.Callbacks.EvaluatePath == nil || e.Callbacks.AdversarialEval == nil {
		return "", false, nil, fmt.Errorf("mcts engine requires ProposeBranches, EvaluatePath, and AdversarialEval callbacks")
	}

	rng := rand.New(rand.NewSource(cfg.Seed))
	root := &ThoughtNode{ID: "root", Answer: strings.TrimSpace(draftAnswer), Depth: 0}

	bestAnswer := strings.TrimSpace(draftAnswer)
	bestScore := 0.0

	for i := 0; i < cfg.Iterations; i++ {
		if ctx.Err() != nil {
			break
		}

		selected := e.selectNode(root, cfg, rng)
		if selected == nil {
			break
		}
		if selected.Depth < cfg.RolloutDepth {
			e.expandNode(ctx, selected, cfg)
			if len(selected.Children) > 0 {
				choice := pickUnprunedChild(selected.Children, rng)
				if choice != nil {
					selected = choice
				}
			}
		}

		score, refined, ok := e.evaluateNode(ctx, selected)
		if !ok {
			continue
		}
		e.backpropagate(selected, score)

		candidate := strings.TrimSpace(refined)
		if candidate == "" {
			candidate = strings.TrimSpace(selected.Answer)
		}
		if score > bestScore && candidate != "" {
			bestScore = score
			bestAnswer = candidate
		}
	}

	if strings.TrimSpace(bestAnswer) == "" {
		return "", false, root, nil
	}
	return strings.TrimSpace(bestAnswer), true, root, nil
}

func (e *MCTSEngine) normalizedConfig() MCTSConfig {
	cfg := e.Config
	if cfg.Iterations <= 0 {
		cfg.Iterations = 5
	}
	if cfg.BranchFactor <= 0 {
		cfg.BranchFactor = 3
	}
	if cfg.BranchFactor < 3 {
		cfg.BranchFactor = 3
	}
	if cfg.BranchFactor > 5 {
		cfg.BranchFactor = 5
	}
	if cfg.RolloutDepth <= 0 {
		cfg.RolloutDepth = 2
	}
	if cfg.UCB1C <= 0 {
		cfg.UCB1C = defaultUCB1C
	}
	if cfg.PruneThreshold <= 0 {
		cfg.PruneThreshold = defaultPruneThreshold
	}
	if cfg.Seed == 0 {
		cfg.Seed = time.Now().UnixNano()
	}
	return cfg
}

func (e *MCTSEngine) selectNode(root *ThoughtNode, cfg MCTSConfig, rng *rand.Rand) *ThoughtNode {
	node := root
	for node != nil && node.Depth < cfg.RolloutDepth {
		children := unprunedChildren(node.Children)
		if len(children) == 0 {
			return node
		}
		var unvisited []*ThoughtNode
		for _, c := range children {
			if c.Visits == 0 {
				unvisited = append(unvisited, c)
			}
		}
		if len(unvisited) > 0 {
			return unvisited[rng.Intn(len(unvisited))]
		}
		best := children[0]
		bestUCB := ucb(best, maxIntLocal(node.Visits, 1), cfg.UCB1C)
		for _, c := range children[1:] {
			u := ucb(c, maxIntLocal(node.Visits, 1), cfg.UCB1C)
			if u > bestUCB {
				best = c
				bestUCB = u
			}
		}
		node = best
	}
	return node
}

func (e *MCTSEngine) expandNode(ctx context.Context, node *ThoughtNode, cfg MCTSConfig) {
	if node == nil || node.Pruned || node.Depth >= cfg.RolloutDepth {
		return
	}
	if len(node.Children) > 0 {
		return
	}
	branches, err := e.Callbacks.ProposeBranches(ctx, node.Answer, cfg.BranchFactor)
	if err != nil || len(branches) == 0 {
		return
	}
	for i, b := range branches {
		ans := strings.TrimSpace(b)
		if ans == "" {
			continue
		}
		id := fmt.Sprintf("%s.%d", node.IDOrDefault(), i+1)
		node.Children = append(node.Children, &ThoughtNode{
			ID:     id,
			Answer: ans,
			Depth:  node.Depth + 1,
			Parent: node,
		})
	}
}

func (e *MCTSEngine) evaluateNode(ctx context.Context, node *ThoughtNode) (float64, string, bool) {
	if node == nil || node.Pruned {
		return 0, "", false
	}
	base, err := e.Callbacks.EvaluatePath(ctx, node.Answer)
	if err != nil {
		return 0, "", false
	}
	adv, err := e.Callbacks.AdversarialEval(ctx, node.Answer)
	if err != nil {
		return 0, "", false
	}

	baseConf := clamp01Local(base.Confidence)
	advConf := clamp01Local(adv.Confidence)
	weighted := clamp01Local((baseConf * 0.55) + (advConf * 0.45))

	node.Score = weighted
	node.Confidence = weighted
	if weighted < e.normalizedConfig().PruneThreshold {
		node.Pruned = true
		node.PruneReason = fmt.Sprintf("kill-switch: branch score %.2f < %.2f", weighted, e.normalizedConfig().PruneThreshold)
	}

	candidate := strings.TrimSpace(base.Candidate)
	if candidate == "" {
		candidate = strings.TrimSpace(adv.Candidate)
	}
	if candidate == "" {
		candidate = strings.TrimSpace(node.Answer)
	}
	return weighted, candidate, true
}

func (e *MCTSEngine) backpropagate(node *ThoughtNode, score float64) {
	for n := node; n != nil; n = n.Parent {
		n.Visits++
		n.ValueSum += score
	}
}

func (n *ThoughtNode) IDOrDefault() string {
	if n == nil || strings.TrimSpace(n.ID) == "" {
		return "node"
	}
	return strings.TrimSpace(n.ID)
}

func (n *ThoughtNode) AverageValue() float64 {
	if n == nil || n.Visits == 0 {
		return 0
	}
	return n.ValueSum / float64(n.Visits)
}

func unprunedChildren(in []*ThoughtNode) []*ThoughtNode {
	out := make([]*ThoughtNode, 0, len(in))
	for _, c := range in {
		if c == nil || c.Pruned {
			continue
		}
		out = append(out, c)
	}
	return out
}

func pickUnprunedChild(children []*ThoughtNode, rng *rand.Rand) *ThoughtNode {
	cands := unprunedChildren(children)
	if len(cands) == 0 {
		return nil
	}
	return cands[rng.Intn(len(cands))]
}

func ucb(node *ThoughtNode, parentVisits int, c float64) float64 {
	if node == nil || node.Pruned {
		return math.Inf(-1)
	}
	if node.Visits == 0 {
		return math.Inf(1)
	}
	avg := node.ValueSum / float64(node.Visits)
	explore := c * math.Sqrt(math.Log(float64(maxIntLocal(parentVisits, 1)))/float64(node.Visits))
	return avg + explore
}

func clamp01Local(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func maxIntLocal(a, b int) int {
	if a > b {
		return a
	}
	return b
}
