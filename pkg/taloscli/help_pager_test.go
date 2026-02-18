package taloscli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextCommandShowsPagedHelp(t *testing.T) {
	oldPath := helpStatePath
	helpStatePath = filepath.Join(t.TempDir(), "help_state.json")
	defer func() { helpStatePath = oldPath }()

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help execute failed: %v", err)
	}
	first := out.String()
	if !strings.Contains(first, "talos /next") {
		t.Fatalf("expected first page to include /next hint, got: %s", first)
	}

	out.Reset()
	rootCmd.SetArgs([]string{"next"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("next execute failed: %v", err)
	}
	next := out.String()
	if strings.Contains(next, "No active help page session") {
		t.Fatalf("expected next page progression, got: %s", next)
	}
	if !strings.Contains(next, "TALOS CLI HELP") {
		t.Fatalf("expected help page header on next, got: %s", next)
	}
}
