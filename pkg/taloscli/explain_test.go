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
