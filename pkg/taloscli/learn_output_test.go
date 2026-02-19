package taloscli

import (
	"strings"
	"testing"

	"github.com/Thynaptic/P-LMv1/pkg/rag"
)

func TestRenderRemoteLearnSummaryFriendlyIncludesSourceAndProgress(t *testing.T) {
	out := renderRemoteLearnSummaryFriendly(
		[]string{"https://example.com/a", "https://example.com/b"},
		rag.RemoteIndexStats{ItemsFetched: 12, ItemsIndexed: 7, ChunksIndexed: 21},
	)
	for _, token := range []string{
		"LEARN REPORT",
		"STATUS",
		"SUCCESS",
		"https://example.com/a, https://example.com/b",
		"fetched: 12 | indexed: 7",
		"chunks_indexed: 21",
	} {
		if !strings.Contains(out, token) {
			t.Fatalf("expected token %q in output: %s", token, out)
		}
	}
}

func TestRenderDirectoryLearnSummaryFriendlyIncludesSourceAndProgress(t *testing.T) {
	out := renderDirectoryLearnSummaryFriendly("/tmp/docs", rag.IndexStats{
		FilesScanned:  8,
		FilesIndexed:  5,
		ChunksIndexed: 14,
	})
	for _, token := range []string{
		"LEARN REPORT",
		"STATUS",
		"SUCCESS",
		"/tmp/docs",
		"scanned_files: 8 | indexed_files: 5",
		"chunks_indexed: 14",
	} {
		if !strings.Contains(out, token) {
			t.Fatalf("expected token %q in output: %s", token, out)
		}
	}
}

func TestRenderInlineLearnSummaryFriendlyIncludesSource(t *testing.T) {
	out := renderInlineLearnSummaryFriendly(true, "/tmp/notes.md")
	for _, token := range []string{
		"LEARN REPORT",
		"STATUS",
		"SUCCESS",
		"file: /tmp/notes.md",
		"indexed: 1/1",
	} {
		if !strings.Contains(out, token) {
			t.Fatalf("expected token %q in output: %s", token, out)
		}
	}
}

func TestLearnCommandHasVerboseFlag(t *testing.T) {
	f := learnCmd.Flags().Lookup("verbose")
	if f == nil {
		t.Fatal("expected --verbose flag to be registered on learn command")
	}
}
