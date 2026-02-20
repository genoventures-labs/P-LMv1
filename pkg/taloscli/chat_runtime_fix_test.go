package taloscli

import (
	"strings"
	"testing"
)

func TestIsTrivialPrompt(t *testing.T) {
	if !isTrivialPrompt("Online?") {
		t.Fatal("expected short ping prompt to be trivial")
	}
	if isTrivialPrompt("Research and compare vector databases") {
		t.Fatal("expected research prompt to be non-trivial")
	}
}

func TestParseToolCallsMarkdownWrapper(t *testing.T) {
	in := "### Tool Call: web_search\n\n{\"tool\":\"web_search\",\"args\":{\"query\":\"Bitcoin digital asset\"}}"
	calls, ok := parseToolCalls(in)
	if !ok || len(calls) != 1 {
		t.Fatalf("expected one parsed tool call, got ok=%v calls=%v", ok, calls)
	}
	if calls[0].Tool != "web_search" {
		t.Fatalf("expected web_search, got %s", calls[0].Tool)
	}
}

func TestParseToolCallsVisualHandshakeTool(t *testing.T) {
	in := `{"tool":"analyze_visual_target","args":{"consent":true,"intent":"analyze_ui_element","target":{"x":10,"y":20,"width":100,"height":60,"label":"Save"}}}`
	calls, ok := parseToolCalls(in)
	if !ok || len(calls) != 1 {
		t.Fatalf("expected one parsed tool call, got ok=%v calls=%v", ok, calls)
	}
	if calls[0].Tool != "analyze_visual_target" {
		t.Fatalf("expected analyze_visual_target, got %s", calls[0].Tool)
	}
}

func TestParseToolCallsMultimodalTool(t *testing.T) {
	in := `{"tool":"multimodal_tool","args":{"query":"trace sasswall drift","target":{"label":"error banner","x":20,"y":30,"width":120,"height":70}}}`
	calls, ok := parseToolCalls(in)
	if !ok || len(calls) != 1 {
		t.Fatalf("expected one parsed multimodal call, got ok=%v calls=%v", ok, calls)
	}
	if calls[0].Tool != "multimodal_tool" {
		t.Fatalf("expected multimodal_tool, got %s", calls[0].Tool)
	}
}

func TestParseToolCallsDocSearchTool(t *testing.T) {
	in := `{"tool":"doc_search","args":{"query":"Sasswall","dir":"./docs"}}`
	calls, ok := parseToolCalls(in)
	if !ok || len(calls) != 1 {
		t.Fatalf("expected one parsed call, got ok=%v calls=%v", ok, calls)
	}
	if calls[0].Tool != "doc_search" {
		t.Fatalf("expected doc_search, got %s", calls[0].Tool)
	}
}

func TestTruncateContextEntriesTrimsPerEntryAndTotal(t *testing.T) {
	in := []string{
		"  " + strings.Repeat("a", 20) + "  ",
		strings.Repeat("b", 20),
	}
	out := truncateContextEntries(in, 10, 40)
	if len(out) != 2 {
		t.Fatalf("expected 2 truncated entries, got %d (%v)", len(out), out)
	}
	if out[0] != strings.Repeat("a", 10)+" ..." {
		t.Fatalf("unexpected first entry: %q", out[0])
	}
	if out[1] != strings.Repeat("b", 10)+" ..." {
		t.Fatalf("unexpected second entry: %q", out[1])
	}
}

func TestTruncateContextEntriesRespectsTotalCap(t *testing.T) {
	in := []string{
		strings.Repeat("x", 15),
		strings.Repeat("y", 15),
	}
	out := truncateContextEntries(in, 15, 15)
	if len(out) != 1 {
		t.Fatalf("expected total cap to keep one entry, got %d", len(out))
	}
}

func TestSystemPromptForTurnUsesMinimalForTopLevelMinimalMode(t *testing.T) {
	got := systemPromptForTurn(cognitionBudget{Mode: "minimal"}, 0)
	if got != minimalSystemPrompt {
		t.Fatal("expected minimal prompt for top-level minimal cognition mode")
	}
}

func TestSystemPromptForTurnUsesFullForRecursiveTurns(t *testing.T) {
	got := systemPromptForTurn(cognitionBudget{Mode: "minimal"}, 1)
	if got != systemPrompt {
		t.Fatal("expected full prompt for recursive turns")
	}
}

func TestPrimaryUserRequestExtractsGoalLockRequest(t *testing.T) {
	in := "Keep focus on active objective [abc]. Request: Summarize last learning run"
	got := primaryUserRequest(in)
	if got != "Summarize last learning run" {
		t.Fatalf("unexpected extracted request: %q", got)
	}
}

func TestShouldBypassIntentCorrectionForShortOperationalPrompt(t *testing.T) {
	if !shouldBypassIntentCorrection("Summarize last learning run") {
		t.Fatal("expected short operational prompt to bypass intent correction")
	}
}

func TestShouldBypassIntentCorrectionFalseForComplexPrompt(t *testing.T) {
	if shouldBypassIntentCorrection("Research and compare vector databases with tradeoff analysis") {
		t.Fatal("expected complex prompt to keep intent correction enabled")
	}
}

func TestHasVisibleToken(t *testing.T) {
	if hasVisibleToken("   ") {
		t.Fatal("expected whitespace-only content to not count as visible token")
	}
	if !hasVisibleToken("hello") {
		t.Fatal("expected non-empty content to count as visible token")
	}
}

func TestPlanCognitionBudgetMinimalDisablesAnchoredContext(t *testing.T) {
	b := planCognitionBudget("Summarize last learning run", nil, "minimal")
	if b.HistoryTopK != 0 || b.KnowledgeTopK != 0 {
		t.Fatalf("expected minimal mode to disable anchored context, got h=%d k=%d", b.HistoryTopK, b.KnowledgeTopK)
	}
}

func TestShouldStreamTopLevelMinimal(t *testing.T) {
	if !shouldStreamTopLevelMinimal(cognitionBudget{Mode: "minimal"}, 0) {
		t.Fatal("expected top-level minimal mode to stream output")
	}
	if shouldStreamTopLevelMinimal(cognitionBudget{Mode: "balanced"}, 0) {
		t.Fatal("expected non-minimal mode not to force streaming")
	}
}

func TestChatCommandHasVerboseFlag(t *testing.T) {
	f := chatCmd.Flags().Lookup("verbose")
	if f == nil {
		t.Fatal("expected --verbose flag on chat command")
	}
}

func TestChatCommandHasDomainFlag(t *testing.T) {
	f := chatCmd.Flags().Lookup("domain")
	if f == nil {
		t.Fatal("expected --domain flag on chat command")
	}
}

func TestResolveChatDomainPrefersFlagThenEnv(t *testing.T) {
	prev := chatDomain
	defer func() { chatDomain = prev }()
	t.Setenv("PLM_CHAT_DOMAIN", "ops")
	chatDomain = ""
	if got := resolveChatDomain(); got != "ops" {
		t.Fatalf("expected env fallback domain ops, got %q", got)
	}
	chatDomain = "talos-runtime"
	if got := resolveChatDomain(); got != "talos-runtime" {
		t.Fatalf("expected flag domain talos-runtime, got %q", got)
	}
}

func TestSystemPromptsEnforceInsufficientKnowledgeResponse(t *testing.T) {
	required := `I don't have enough knowledge to answer that reliably.`
	if !strings.Contains(systemPrompt, required) {
		t.Fatal("expected primary system prompt to include strict insufficient-knowledge response policy")
	}
	if !strings.Contains(minimalSystemPrompt, required) {
		t.Fatal("expected minimal system prompt to include strict insufficient-knowledge response policy")
	}
}

func TestMaybeHandleBuiltInChatCommandScoutStatus(t *testing.T) {
	handled, out := maybeHandleBuiltInChatCommand("scout status")
	if !handled {
		t.Fatal("expected scout status built-in command to be handled")
	}
	for _, token := range []string{"SCOUT STATUS", "briefs_24h:", "recommendation:"} {
		if !strings.Contains(out, token) {
			t.Fatalf("expected token %q in output: %s", token, out)
		}
	}
}

func TestNormalizeInsufficientKnowledgeResponseCollapsesMixedOutput(t *testing.T) {
	in := "Some speculative answer.\n\n" + insufficientKnowledgeResponse
	got := normalizeInsufficientKnowledgeResponse(in)
	if got != insufficientKnowledgeResponse {
		t.Fatalf("expected strict insufficient-knowledge response, got %q", got)
	}
}

func TestExtractGroundedLearningSubject(t *testing.T) {
	got := extractGroundedLearningSubject("What have you learned about Anthropic, from recent training?")
	if got != "anthropic" {
		t.Fatalf("expected extracted subject anthropic, got %q", got)
	}
}

func TestHasGroundedLearningEvidence(t *testing.T) {
	knowledge := []string{"Anthropic announced updates in recent notes."}
	if !hasGroundedLearningEvidence("anthropic", knowledge) {
		t.Fatal("expected grounded evidence to match subject in knowledge context")
	}
	if hasGroundedLearningEvidence("openai", knowledge) {
		t.Fatal("expected no grounded evidence for unmatched subject")
	}
}

func TestShouldBlockUngroundedFromKnowledge(t *testing.T) {
	prev := chatZeroTrustGating
	chatZeroTrustGating = true
	defer func() { chatZeroTrustGating = prev }()

	if !shouldBlockUngroundedFromKnowledge("What's Thynaptic?", nil) {
		t.Fatal("expected zero-trust gate to block when no knowledge exists")
	}
	if shouldBlockUngroundedFromKnowledge("What's Thynaptic?", []string{"Thynaptic is a local-first runtime."}) {
		t.Fatal("expected zero-trust gate to allow when knowledge exists")
	}
	if !shouldBlockUngroundedFromKnowledge("What's Thynaptic?", []string{"alignment_audit session_state"}) {
		t.Fatal("expected gate to block when namespace knowledge is irrelevant to query")
	}
	if !shouldBlockUngroundedFromKnowledge("What have you learned about Anthropic, from recent training?", []string{"General AI note"}) {
		t.Fatal("expected subject-aware gate to block when knowledge misses the requested subject")
	}
}

func TestExtractGroundingKeywords(t *testing.T) {
	got := extractGroundingKeywords("What is GLM in TALOS runtime?")
	if len(got) == 0 {
		t.Fatal("expected non-empty grounding keywords")
	}
	if got[0] != "talos" && got[0] != "runtime" {
		// keyword order is deterministic by query order; ensure stop words were removed.
		t.Fatalf("unexpected first grounding keyword: %q", got[0])
	}
}
