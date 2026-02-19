package taloscli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExplainKnownCapability(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"explain", "doctor"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected explain doctor to succeed, got error: %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "CAPABILITY") || !strings.Contains(rendered, "doctor") {
		t.Fatalf("expected explain output to include capability section, got: %s", rendered)
	}
	if !strings.Contains(rendered, "USAGE") || !strings.Contains(rendered, "FUNCTIONALITY") {
		t.Fatalf("expected explain output sections in response, got: %s", rendered)
	}
}

func TestExplainAliasResolution(t *testing.T) {
	doc, ok := lookupCapabilityDoc("toolserver")
	if !ok {
		t.Fatal("expected alias 'toolserver' to resolve")
	}
	if doc.Name != "tools admin" {
		t.Fatalf("expected alias to resolve tools admin, got %q", doc.Name)
	}
}

func TestExplainRebootAliasResolution(t *testing.T) {
	doc, ok := lookupCapabilityDoc("reboot")
	if !ok {
		t.Fatal("expected alias 'reboot' to resolve")
	}
	if doc.Name != "monitor" {
		t.Fatalf("expected alias to resolve monitor, got %q", doc.Name)
	}
}

func TestExplainUnknownCapability(t *testing.T) {
	out := &bytes.Buffer{}
	writeExplainNotFound(out, "does-not-exist")
	rendered := out.String()
	if !strings.Contains(rendered, "Unknown capability") {
		t.Fatalf("expected unknown capability error, got: %s", rendered)
	}
	if !strings.Contains(rendered, "AVAILABLE CAPABILITIES") {
		t.Fatalf("expected available capabilities list, got: %s", rendered)
	}
}

func TestExplainMemoryCapabilityIncludesTuningVars(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"explain", "memory"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected explain memory to succeed, got error: %v", err)
	}
	rendered := out.String()
	required := []string{
		"CAPABILITY",
		"memory",
		"TALOS_MEMORY_RETRIEVAL_MODE",
		"TALOS_RETRIEVAL_CANDIDATE_WEIGHTS",
		"TALOS_RETRIEVAL_DYNAMIC_WEIGHTS",
		"TALOS_RETRIEVAL_KNOWLEDGE_WEIGHTS",
		"TALOS_RETRIEVAL_SEGMENT_WEIGHTS",
		"TALOS_MAR_ENABLED",
		"TALOS_MAR_CANDIDATE_LIMIT",
		"TALOS_MAR_MAX_ANCHORS",
		"TALOS_MAR_WEIGHTS",
		"TALOS_MAR_MIN_SCORE",
		"TALOS_MAR_TOPOLOGY_BOOST",
		"TALOS_MAR_CACHE_TTL_SEC",
		"TALOS_MAR_CACHE_MAX",
		"TALOS_MAR_STATUS_ENABLED",
		"TALOS_TOPOLOGY_ENABLED",
		"TALOS_TOPOLOGY_WEIGHTS",
		"TALOS_DOC_ROUTING_WEIGHTS",
		"TALOS_DOC_ROUTING_TIMEOUT_MS",
		"TALOS_LONGFORM_ENABLED",
		"TALOS_LONGFORM_MAX_PASSES",
		"TALOS_STYLE_V2_ENABLED",
		"TALOS_STYLE_MODEL_ENABLED",
		"TALOS_STYLE_MODEL_TIMEOUT_MS",
		"TALOS_STYLE_CACHE_TTL_SEC",
		"TALOS_STYLE_CACHE_MAX",
		"TALOS_SYMBOLIC_SUPERVISION_ENABLED",
		"TALOS_SYMBOLIC_ENFORCEMENT_MODE",
		"TALOS_SYMBOLIC_MAX_CORRECTIONS",
		"TALOS_SYMBOLIC_TIMEOUT_MS",
		"TALOS_SYMBOLIC_CACHE_TTL_SEC",
		"TALOS_SYMBOLIC_CACHE_MAX",
		"TALOS_OVERLAY_POLICY_ENABLED",
		"TALOS_OVERLAY_DEFAULT_MODE",
		"TALOS_OVERLAY_MAX_BOXES",
		"TALOS_OVERLAY_MIN_CONFIDENCE",
		"TALOS_OVERLAY_HIGH_RISK_AUTO_THRESHOLD",
		"TALOS_OVERLAY_SESSION_CONSENT_TTL_SEC",
		"TALOS_OVERLAY_TAXONOMY_STRICT",
		"TALOS_REFLECTION_V2_ENABLED",
		"TALOS_REFLECTION_ENFORCEMENT_MODE",
		"TALOS_REFLECTION_WARN_THRESHOLD",
		"TALOS_REFLECTION_STEER_THRESHOLD",
		"TALOS_REFLECTION_VETO_THRESHOLD",
		"TALOS_REFLECTION_TIMEOUT_MS",
		"TALOS_REFLECTION_CITATION_GATE",
		"TALOS_REFLECTION_STATUS_ENABLED",
		"TALOS_REFLECTION_LOG_PATH",
		"TALOS_REFLECTION_CACHE_TTL_SEC",
		"TALOS_REFLECTION_CACHE_MAX",
		".memory/worldview_truth.json",
		".memory/worldview_truth_shifts.jsonl",
		"talos monitor status",
	}
	for _, token := range required {
		if !strings.Contains(rendered, token) {
			t.Fatalf("expected explain memory output to contain %q, got: %s", token, rendered)
		}
	}
}

func TestExplainReflectionCapabilityIncludesTuningPlaybook(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"explain", "reflection"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected explain reflection to succeed, got error: %v", err)
	}
	rendered := out.String()
	required := []string{
		"CAPABILITY",
		"reflection",
		"TALOS_REFLECTION_V2_ENABLED",
		"TALOS_REFLECTION_ENFORCEMENT_MODE",
		"TALOS_REFLECTION_WARN_THRESHOLD",
		"TALOS_REFLECTION_STEER_THRESHOLD",
		"TALOS_REFLECTION_VETO_THRESHOLD",
		"TALOS_REFLECTION_TIMEOUT_MS",
		"TALOS_REFLECTION_CITATION_GATE",
		"TALOS_REFLECTION_STATUS_ENABLED",
		"TALOS_REFLECTION_LOG_PATH",
		"TALOS_REFLECTION_CACHE_TTL_SEC",
		"TALOS_REFLECTION_CACHE_MAX",
		"tiered mode",
		"hard mode",
		"advisory mode",
	}
	for _, token := range required {
		if !strings.Contains(strings.ToLower(rendered), strings.ToLower(token)) {
			t.Fatalf("expected explain reflection output to contain %q, got: %s", token, rendered)
		}
	}
}

func TestExplainMCTSCapabilityIncludesTuningVars(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"explain", "mcts"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected explain mcts to succeed, got error: %v", err)
	}
	rendered := out.String()
	required := []string{
		"CAPABILITY",
		"mcts",
		"TALOS_MCTS_STRATEGY",
		"TALOS_MCTS_MAX_CONCURRENCY",
		"TALOS_MCTS_WIDENING_ALPHA",
		"TALOS_MCTS_WIDENING_K",
		"TALOS_MCTS_PRIOR_WEIGHT",
		"TALOS_MCTS_VIRTUAL_LOSS",
		"TALOS_MCTS_MAX_CHILDREN_PER_NODE",
		"TALOS_MCTS_LEGACY_FALLBACK",
		"TALOS_MCTS_STATUS_ENABLED",
		"TALOS_MCTS_TRACE_PATH",
	}
	for _, token := range required {
		if !strings.Contains(strings.ToLower(rendered), strings.ToLower(token)) {
			t.Fatalf("expected explain mcts output to contain %q, got: %s", token, rendered)
		}
	}
}
