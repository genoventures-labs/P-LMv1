package taloscli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type pipelineStep struct {
	Raw  string
	Args []string
}

type pipelineState struct {
	LastResearchSessionID string
}

var pipelineCmd = &cobra.Command{
	Use:   "pipeline <chain>",
	Short: "Execute an allowlisted TALOS command chain.",
	Long:  "Executes a strict, allowlisted pipeline such as research -> learn with structured handoff.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain := strings.TrimSpace(strings.Join(args, " "))
		if chain == "" {
			return fmt.Errorf("pipeline chain cannot be empty")
		}
		return runPipelineChain(chain)
	},
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
}

func runPipelineChain(chain string) error {
	steps, err := parsePipelineChain(chain)
	if err != nil {
		return err
	}
	if err := validatePipelineSteps(steps); err != nil {
		return err
	}
	state := &pipelineState{}
	for i, step := range steps {
		if err := executePipelineStep(step, state); err != nil {
			return fmt.Errorf("pipeline failed at step %d (%s): %w", i+1, step.Raw, err)
		}
	}
	fmt.Printf("PIPELINE RESULT\n\nSTATUS\n  SUCCESS\n\nSTEPS\n  executed: %d\n", len(steps))
	return nil
}

func parsePipelineChain(chain string) ([]pipelineStep, error) {
	segments, err := splitPipelineSegments(chain)
	if err != nil {
		return nil, err
	}
	steps := make([]pipelineStep, 0, len(segments))
	for _, seg := range segments {
		argv, err := splitShellTokens(seg)
		if err != nil {
			return nil, fmt.Errorf("invalid step %q: %w", seg, err)
		}
		if len(argv) == 0 {
			continue
		}
		steps = append(steps, pipelineStep{Raw: strings.TrimSpace(seg), Args: argv})
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("no pipeline steps found")
	}
	return steps, nil
}

func splitPipelineSegments(chain string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	for _, r := range chain {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
			cur.WriteRune(r)
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			cur.WriteRune(r)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			cur.WriteRune(r)
		case ';', '|':
			if inSingle || inDouble {
				cur.WriteRune(r)
				continue
			}
			piece := strings.TrimSpace(cur.String())
			if piece != "" {
				out = append(out, piece)
			}
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if escaped || inSingle || inDouble {
		return nil, errors.New("unterminated quote or escape in chain")
	}
	if piece := strings.TrimSpace(cur.String()); piece != "" {
		out = append(out, piece)
	}
	return out, nil
}

func splitShellTokens(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		args = append(args, cur.String())
		cur.Reset()
	}
	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '\'':
			if inDouble {
				cur.WriteRune(r)
			} else {
				inSingle = !inSingle
			}
		case '"':
			if inSingle {
				cur.WriteRune(r)
			} else {
				inDouble = !inDouble
			}
		case ' ', '\t', '\n':
			if inSingle || inDouble {
				cur.WriteRune(r)
			} else {
				flush()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if escaped || inSingle || inDouble {
		return nil, errors.New("unterminated quote or escape")
	}
	flush()
	return args, nil
}

func normalizePipelineAction(step pipelineStep) string {
	if len(step.Args) == 0 {
		return ""
	}
	if strings.EqualFold(step.Args[0], "research") {
		if len(step.Args) > 1 {
			if strings.EqualFold(step.Args[1], "run") {
				return "research/run"
			}
			if strings.EqualFold(step.Args[1], "deep") {
				return "research/deep"
			}
		}
		return "research/run"
	}
	if strings.EqualFold(step.Args[0], "learn") {
		for i := 1; i < len(step.Args); i++ {
			tok := step.Args[i]
			if strings.EqualFold(tok, "--from-research") || strings.HasPrefix(strings.ToLower(tok), "--from-research=") {
				return "learn/from-research"
			}
		}
		return "learn/other"
	}
	return strings.ToLower(step.Args[0])
}

func validatePipelineSteps(steps []pipelineStep) error {
	if len(steps) < 2 {
		return fmt.Errorf("pipeline must contain at least 2 steps")
	}
	for i := 0; i < len(steps)-1; i++ {
		from := normalizePipelineAction(steps[i])
		to := normalizePipelineAction(steps[i+1])
		if (from == "research/run" || from == "research/deep") && to == "learn/from-research" {
			continue
		}
		return fmt.Errorf("invalid chain transition: %s -> %s (allowed: research run|deep -> learn --from-research)", from, to)
	}
	return nil
}

func executePipelineStep(step pipelineStep, st *pipelineState) error {
	action := normalizePipelineAction(step)
	switch action {
	case "research/run":
		mode := "run"
		idx := 1
		if len(step.Args) > 1 && (strings.EqualFold(step.Args[1], "run") || strings.EqualFold(step.Args[1], "deep")) {
			mode = strings.ToLower(step.Args[1])
			idx = 2
		}
		query := strings.TrimSpace(strings.Join(step.Args[idx:], " "))
		if query == "" {
			return fmt.Errorf("research query cannot be empty")
		}
		report, sessionID, err := executeResearchMode(mode, query)
		if err != nil {
			return err
		}
		_ = report
		st.LastResearchSessionID = sessionID
		return nil
	case "research/deep":
		query := ""
		if len(step.Args) > 2 {
			query = strings.TrimSpace(strings.Join(step.Args[2:], " "))
		}
		if query == "" {
			return fmt.Errorf("research query cannot be empty")
		}
		report, sessionID, err := executeResearchMode("deep", query)
		if err != nil {
			return err
		}
		_ = report
		st.LastResearchSessionID = sessionID
		return nil
	case "learn/from-research":
		artifactID := "latest"
		for i := 1; i < len(step.Args); i++ {
			tok := step.Args[i]
			if strings.HasPrefix(strings.ToLower(tok), "--from-research=") {
				artifactID = strings.TrimSpace(strings.SplitN(tok, "=", 2)[1])
			}
			if strings.EqualFold(tok, "--from-research") {
				if i+1 < len(step.Args) && !strings.HasPrefix(step.Args[i+1], "--") {
					artifactID = strings.TrimSpace(step.Args[i+1])
				}
			}
		}
		if strings.EqualFold(artifactID, "latest") && strings.TrimSpace(st.LastResearchSessionID) != "" {
			artifactID = st.LastResearchSessionID
		}
		return executeLearnFromResearch(artifactID, true, true)
	default:
		return fmt.Errorf("unsupported pipeline step: %s", step.Raw)
	}
}
