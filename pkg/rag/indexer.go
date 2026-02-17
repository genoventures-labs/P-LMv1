package rag

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Thynaptic/P-LMv1/pkg/memory"
)

// IndexOptions configures bulk document indexing behavior.
type IndexOptions struct {
	Recursive     bool
	MaxChunkChars int
	ChunkOverlap  int
	Extensions    map[string]bool
	AllTypes      bool
	OnFileEvent   func(FileEvent)
}

// IndexStats summarizes an indexing run.
type IndexStats struct {
	FilesScanned       int
	FilesIndexed       int
	ChunksIndexed      int
	SkippedUnsupported int
	SkippedBinary      int
	ParseErrors        int
	IndexErrors        int
	WalkErrors         int
}

// FileEvent reports per-file indexing progress and outcomes.
type FileEvent struct {
	Path         string
	RelPath      string
	Outcome      string
	Chunks       int
	Error        string
	FilesScanned int
	FilesIndexed int
}

// DefaultIndexOptions returns pragmatic defaults for document indexing.
func DefaultIndexOptions() IndexOptions {
	return IndexOptions{
		Recursive:     true,
		MaxChunkChars: 1200,
		ChunkOverlap:  200,
		Extensions: map[string]bool{
			".pdf":      true,
			".md":       true,
			".markdown": true,
			".txt":      true,
			".rst":      true,
			".adoc":     true,
			".json":     true,
			".yaml":     true,
			".yml":      true,
			".csv":      true,
			".tsv":      true,
			".html":     true,
			".htm":      true,
		},
	}
}

// ParseExtensionsCSV converts ".md,.pdf" style input to extension map.
func ParseExtensionsCSV(input string) map[string]bool {
	exts := make(map[string]bool)
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		if !strings.HasPrefix(part, ".") {
			part = "." + part
		}
		exts[part] = true
	}
	return exts
}

// ExtensionsToCSV serializes extension map into deterministic CSV.
func ExtensionsToCSV(exts map[string]bool) string {
	if len(exts) == 0 {
		return ""
	}
	order := []string{
		".pdf", ".md", ".markdown", ".txt", ".rst", ".adoc",
		".json", ".yaml", ".yml", ".csv", ".tsv", ".html", ".htm",
	}
	var out []string
	seen := make(map[string]bool)
	for _, ext := range order {
		if exts[ext] {
			out = append(out, ext)
			seen[ext] = true
		}
	}
	for ext := range exts {
		if !seen[ext] {
			out = append(out, ext)
		}
	}
	return strings.Join(out, ",")
}

// IndexDirectory indexes supported files under a directory into knowledge memory.
func IndexDirectory(mm *memory.MemoryManager, dir string, opts IndexOptions) (IndexStats, error) {
	if opts.MaxChunkChars <= 0 {
		opts.MaxChunkChars = 1200
	}
	if opts.ChunkOverlap < 0 {
		opts.ChunkOverlap = 0
	}
	if opts.ChunkOverlap >= opts.MaxChunkChars {
		opts.ChunkOverlap = opts.MaxChunkChars / 4
	}
	if len(opts.Extensions) == 0 {
		opts.Extensions = DefaultIndexOptions().Extensions
	}
	if opts.AllTypes {
		opts.Extensions = nil
	}

	var stats IndexStats
	absRoot, err := filepath.Abs(dir)
	if err != nil {
		return stats, fmt.Errorf("failed to resolve directory path: %w", err)
	}

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			stats.WalkErrors++
			emitFileEvent(opts, FileEvent{
				Path:         path,
				RelPath:      relPath(absRoot, path),
				Outcome:      "walk-error",
				Error:        walkErr.Error(),
				FilesScanned: stats.FilesScanned,
				FilesIndexed: stats.FilesIndexed,
			})
			return nil
		}
		if d.IsDir() {
			if !opts.Recursive && path != absRoot {
				return filepath.SkipDir
			}
			return nil
		}

		stats.FilesScanned++
		ext := strings.ToLower(filepath.Ext(path))
		if !opts.AllTypes && !opts.Extensions[ext] {
			stats.SkippedUnsupported++
			emitFileEvent(opts, FileEvent{
				Path:         path,
				RelPath:      relPath(absRoot, path),
				Outcome:      "skipped-unsupported",
				FilesScanned: stats.FilesScanned,
				FilesIndexed: stats.FilesIndexed,
			})
			return nil
		}

		content, parseErr := readDocument(path, ext)
		if parseErr != nil {
			if strings.Contains(parseErr.Error(), "binary file") {
				stats.SkippedBinary++
				emitFileEvent(opts, FileEvent{
					Path:         path,
					RelPath:      relPath(absRoot, path),
					Outcome:      "skipped-binary",
					Error:        parseErr.Error(),
					FilesScanned: stats.FilesScanned,
					FilesIndexed: stats.FilesIndexed,
				})
			} else {
				stats.ParseErrors++
				emitFileEvent(opts, FileEvent{
					Path:         path,
					RelPath:      relPath(absRoot, path),
					Outcome:      "parse-error",
					Error:        parseErr.Error(),
					FilesScanned: stats.FilesScanned,
					FilesIndexed: stats.FilesIndexed,
				})
			}
			return nil
		}
		content = normalizeText(content)
		if strings.TrimSpace(content) == "" {
			emitFileEvent(opts, FileEvent{
				Path:         path,
				RelPath:      relPath(absRoot, path),
				Outcome:      "empty",
				FilesScanned: stats.FilesScanned,
				FilesIndexed: stats.FilesIndexed,
			})
			return nil
		}

		chunks := chunkText(content, opts.MaxChunkChars, opts.ChunkOverlap)
		if len(chunks) == 0 {
			emitFileEvent(opts, FileEvent{
				Path:         path,
				RelPath:      relPath(absRoot, path),
				Outcome:      "empty",
				FilesScanned: stats.FilesScanned,
				FilesIndexed: stats.FilesIndexed,
			})
			return nil
		}

		relPath := relPath(absRoot, path)

		indexFailed := false
		for i, chunk := range chunks {
			metadata := map[string]string{
				"type":        "knowledge",
				"source_type": "file",
				"source_path": relPath,
				"source_ext":  ext,
				"chunk_index": strconv.Itoa(i + 1),
				"chunk_total": strconv.Itoa(len(chunks)),
				"indexed_at":  time.Now().UTC().Format(time.RFC3339),
			}
			if err := mm.AddKnowledge(chunk, metadata); err != nil {
				stats.IndexErrors++
				indexFailed = true
				emitFileEvent(opts, FileEvent{
					Path:         path,
					RelPath:      relPath,
					Outcome:      "index-error",
					Error:        fmt.Sprintf("failed to index chunk %d: %v", i+1, err),
					FilesScanned: stats.FilesScanned,
					FilesIndexed: stats.FilesIndexed,
				})
				break
			}
		}
		if indexFailed {
			return nil
		}

		stats.FilesIndexed++
		stats.ChunksIndexed += len(chunks)
		emitFileEvent(opts, FileEvent{
			Path:         path,
			RelPath:      relPath,
			Outcome:      "indexed",
			Chunks:       len(chunks),
			FilesScanned: stats.FilesScanned,
			FilesIndexed: stats.FilesIndexed,
		})
		return nil
	})
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

func emitFileEvent(opts IndexOptions, event FileEvent) {
	if opts.OnFileEvent != nil {
		opts.OnFileEvent(event)
	}
}

func readDocument(path, ext string) (string, error) {
	if ext == ".pdf" {
		return readPDF(path)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	if isLikelyBinary(data) {
		return "", fmt.Errorf("binary file")
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("non-utf8 text file")
	}
	return string(data), nil
}

func readPDF(path string) (string, error) {
	_, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("pdf indexing requires 'pdftotext' to be installed")
	}
	cmd := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %v: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func isLikelyBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if bytes.Contains(sample, []byte{0}) {
		return true
	}
	return false
}

func normalizeText(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func chunkText(content string, maxChars, overlap int) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	runes := []rune(content)
	if len(runes) <= maxChars {
		return []string{content}
	}

	step := maxChars - overlap
	if step <= 0 {
		step = maxChars
	}

	var chunks []string
	for start := 0; start < len(runes); start += step {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}
