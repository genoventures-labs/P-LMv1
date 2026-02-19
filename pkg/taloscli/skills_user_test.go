package taloscli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsMigrateCommandDryRunAndApply(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	if err := os.MkdirAll(filepath.Join(".skills", "permanent"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyIndex := `{"skills":[{"skill_id":"legacy_skill","name":"legacy","intent":"legacy intent","root_dir":".skills/permanent/legacy_skill","source_path":".skills/permanent/legacy_skill/skill.go","manifest_path":".skills/permanent/legacy_skill/skill_manifest.json","enabled":true}]}`
	if err := os.WriteFile(filepath.Join(".skills", "permanent", "index.json"), []byte(legacyIndex), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"skills", "migrate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("skills migrate dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "DRY-RUN") {
		t.Fatalf("expected dry-run output, got: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SetArgs([]string{"skills", "migrate", "--apply"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("skills migrate apply failed: %v", err)
	}
	if !strings.Contains(out.String(), "APPLIED") {
		t.Fatalf("expected applied output, got: %s", out.String())
	}
}
