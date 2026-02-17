package taloscli

import "testing"

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
