package taloscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureAndRemovePathExportBlock(t *testing.T) {
	tmp := t.TempDir()
	rc := filepath.Join(tmp, ".bashrc")
	binDir := "/tmp/talos-bin"

	updated, err := ensurePathExportBlock(rc, binDir)
	if err != nil {
		t.Fatalf("ensurePathExportBlock failed: %v", err)
	}
	if !updated {
		t.Fatal("expected first ensurePathExportBlock call to update file")
	}

	content, err := os.ReadFile(rc)
	if err != nil {
		t.Fatalf("read profile failed: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, pathBlockStart) || !strings.Contains(text, pathBlockEnd) {
		t.Fatal("expected TALOS PATH block to be present")
	}
	if !strings.Contains(text, binDir) {
		t.Fatalf("expected TALOS PATH block to include bin directory %q", binDir)
	}

	updated, err = ensurePathExportBlock(rc, binDir)
	if err != nil {
		t.Fatalf("second ensurePathExportBlock failed: %v", err)
	}
	if updated {
		t.Fatal("expected second ensurePathExportBlock call to be no-op")
	}

	removed, err := removePathExportBlock(rc)
	if err != nil {
		t.Fatalf("removePathExportBlock failed: %v", err)
	}
	if !removed {
		t.Fatal("expected removePathExportBlock to remove block")
	}

	content, err = os.ReadFile(rc)
	if err != nil {
		t.Fatalf("read profile after remove failed: %v", err)
	}
	text = string(content)
	if strings.Contains(text, pathBlockStart) || strings.Contains(text, pathBlockEnd) {
		t.Fatal("expected TALOS PATH block to be removed")
	}

	removed, err = removePathExportBlock(rc)
	if err != nil {
		t.Fatalf("second removePathExportBlock failed: %v", err)
	}
	if removed {
		t.Fatal("expected second removePathExportBlock call to be no-op")
	}
}
