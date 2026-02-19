package rag

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeHTTPURL(t *testing.T) {
	u, host, err := normalizeHTTPURL("https://example.com/a?x=1#frag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "example.com" {
		t.Fatalf("expected host example.com, got %q", host)
	}
	if strings.Contains(u, "#frag") {
		t.Fatalf("expected fragment removed, got %q", u)
	}
}

func TestNormalizeHTTPURLRejectsUnsupportedScheme(t *testing.T) {
	if _, _, err := normalizeHTTPURL("file:///tmp/a.txt"); err == nil {
		t.Fatal("expected unsupported scheme error")
	}
}

func TestNormalizeHTTPURLRejectsInvalidDomain(t *testing.T) {
	if _, _, err := normalizeHTTPURL("https://-bad-domain-.com/path"); err == nil {
		t.Fatal("expected invalid domain error")
	}
}

func TestExtractLinks(t *testing.T) {
	base := "https://example.com/docs/index.html"
	html := `<a href="/a">A</a><a href="https://example.com/b">B</a><a href="mailto:test@example.com">M</a>`
	links := extractLinks(base, []byte(html))
	if len(links) != 2 {
		t.Fatalf("expected 2 HTTP links, got %d: %#v", len(links), links)
	}
	if links[0] != "https://example.com/a" && links[1] != "https://example.com/a" {
		t.Fatalf("expected resolved relative link, got %#v", links)
	}
}

func TestParseHFFirstRows(t *testing.T) {
	body := []byte(`{
		"rows":[
			{"row":{"text":"hello","label":1}},
			{"row":{"text":"world","label":0}}
		]
	}`)
	rows, err := parseHFFirstRows(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if !strings.Contains(rows[0], "text: hello") {
		t.Fatalf("unexpected flattened row: %q", rows[0])
	}
}

func TestIsDomainAllowed(t *testing.T) {
	seeds := map[string]bool{"example.com": true}
	if !isDomainAllowed("example.com", seeds, nil, true) {
		t.Fatal("expected seed domain allowed")
	}
	if isDomainAllowed("other.com", seeds, nil, true) {
		t.Fatal("expected non-seed domain blocked when crawling")
	}
	allow := map[string]bool{"other.com": true}
	if !isDomainAllowed("other.com", seeds, allow, true) {
		t.Fatal("expected explicit allowlist domain to pass")
	}
}

func TestPreflightURLReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := srv.Client()
	opts := DefaultRemoteIndexOptions()
	if err := preflightURLReachable(client, srv.URL+"/doc", opts); err != nil {
		t.Fatalf("expected preflight success, got error: %v", err)
	}
}

func TestExtractHFErrorMessageJSON(t *testing.T) {
	msg := extractHFErrorMessage([]byte(`{"error":"The split train does not exist."}`))
	if !strings.Contains(msg, "split train") {
		t.Fatalf("expected parsed error message, got %q", msg)
	}
}

func TestFetchHFSplitHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/splits" {
			_, _ = w.Write([]byte(`{"splits":[{"config":"default","split":"validation"},{"config":"default","split":"test"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	opts := DefaultRemoteIndexOptions()
	hint, err := fetchHFSplitHint(srv.Client(), srv.URL, "demo/ds", opts)
	if err != nil {
		t.Fatalf("expected split hint success, got error: %v", err)
	}
	if !strings.Contains(hint, "default") || !strings.Contains(hint, "validation") {
		t.Fatalf("unexpected hint: %q", hint)
	}
}

func TestNormalizeRemoteOptionsChunkTitleDefaults(t *testing.T) {
	opts := normalizeRemoteOptions(RemoteIndexOptions{})
	if !opts.GenerateChunkTitles {
		t.Fatal("expected GenerateChunkTitles enabled by default")
	}
	if opts.ChunkTitleMaxChars <= 0 {
		t.Fatalf("expected positive ChunkTitleMaxChars, got %d", opts.ChunkTitleMaxChars)
	}
}
