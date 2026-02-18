package taloscli

import (
	"strings"
	"testing"
)

func TestRenderLearnDryRunPlanNoInput(t *testing.T) {
	learnFile = ""
	learnDir = ""
	learnFromResearch = ""
	learnURLs = nil
	learnURLFile = ""
	learnHFDatasets = nil
	if _, err := renderLearnDryRunPlan(nil); err == nil {
		t.Fatal("expected error for missing learn input")
	}
}

func TestRenderResearchDryRunPlan(t *testing.T) {
	out := renderResearchDryRunPlan("run", "test query", researchExecutionContext{})
	if !strings.Contains(out, "TALOS RESEARCH DRY RUN") || !strings.Contains(out, "skipped: true") {
		t.Fatalf("unexpected dry-run output: %s", out)
	}
}
