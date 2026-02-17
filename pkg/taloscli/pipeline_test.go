package taloscli

import (
	"strings"
	"testing"
)

func TestParsePipelineChainQuoted(t *testing.T) {
	steps, err := parsePipelineChain(`research run "k8s release" ; learn --from-research latest`)
	if err != nil {
		t.Fatalf("parse chain: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if got := strings.Join(steps[0].Args, "|"); got != "research|run|k8s release" {
		t.Fatalf("unexpected step[0] args: %s", got)
	}
	if got := strings.Join(steps[1].Args, "|"); got != "learn|--from-research|latest" {
		t.Fatalf("unexpected step[1] args: %s", got)
	}
}

func TestValidatePipelineAllowlist(t *testing.T) {
	ok := []pipelineStep{
		{Raw: "research run x", Args: []string{"research", "run", "x"}},
		{Raw: "learn --from-research latest", Args: []string{"learn", "--from-research", "latest"}},
	}
	if err := validatePipelineSteps(ok); err != nil {
		t.Fatalf("expected allowlisted chain, got %v", err)
	}

	bad := []pipelineStep{
		{Raw: "doctor", Args: []string{"doctor"}},
		{Raw: "learn --from-research latest", Args: []string{"learn", "--from-research", "latest"}},
	}
	if err := validatePipelineSteps(bad); err == nil {
		t.Fatal("expected invalid chain error")
	}
}

func TestSplitPipelineSegmentsUnterminatedQuote(t *testing.T) {
	_, err := splitPipelineSegments(`research run "bad ; learn --from-research latest`)
	if err == nil {
		t.Fatal("expected quote parse error")
	}
}
