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
