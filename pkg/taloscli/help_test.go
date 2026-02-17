package taloscli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpTemplate(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected help command to succeed, got error: %v", err)
	}

	rendered := out.String()
	if rendered == "" {
		t.Fatal("expected help output to be non-empty")
	}

	requiredSections := []string{
		"TALOS CLI",
		"USAGE",
		"AVAILABLE COMMANDS",
		"MORE INFO",
	}
	for _, section := range requiredSections {
		if !strings.Contains(rendered, section) {
			t.Fatalf("expected help output to contain section %q", section)
		}
	}
}
