package taloscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSyntheticTextJSONLLines(t *testing.T) {
	lines := buildSyntheticTextJSONLLines("alpha beta gamma delta", "inline", 10)
	if len(lines) < 2 {
		t.Fatalf("expected chunking into multiple lines, got %d", len(lines))
	}
	for _, line := range lines {
		for _, token := range []string{`"type":"synthetic_text"`, `"source":"inline"`, `"target_text"`} {
			if !strings.Contains(line, token) {
				t.Fatalf("expected token %q in line: %s", token, line)
			}
		}
	}
}

func TestExecuteLearnSyntheticTextWritesOutput(t *testing.T) {
	prevOut := learnSyntheticTextOut
	prevFile := learnFile
	prevSessions := learnSessionsPath
	learnFile = ""
	learnSessionsPath = filepath.Join(t.TempDir(), "learn_sessions.jsonl")
	learnSyntheticTextOut = filepath.Join(t.TempDir(), "synthetic.jsonl")
	t.Cleanup(func() {
		learnSyntheticTextOut = prevOut
		learnFile = prevFile
		learnSessionsPath = prevSessions
	})
	if err := executeLearnSyntheticText([]string{"synthetic sample text for generator"}); err != nil {
		t.Fatalf("executeLearnSyntheticText returned error: %v", err)
	}
	data, err := os.ReadFile(learnSyntheticTextOut)
	if err != nil {
		t.Fatalf("read synthetic output: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `"type":"synthetic_text"`) {
		t.Fatalf("expected synthetic artifact records, got: %s", out)
	}
}
