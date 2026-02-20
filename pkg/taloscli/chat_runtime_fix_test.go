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
