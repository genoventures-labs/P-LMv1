package taloscli

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/memory"
	"github.com/Thynaptic/P-LMv1/pkg/rag"
	"github.com/spf13/cobra"
)

var learnFile string
var learnDir string
var learnRecursive bool
var learnExtensions string
var learnTypes []string
var learnAllTypes bool
var learnChunkChars int
var learnChunkOverlap int
var learnURLs []string
var learnURLFile string
var learnCrawl bool
var learnCrawlDepth int
var learnMaxPages int
var learnRateLimit float64
var learnAllowedDomains []string
var learnUserAgent string
var learnRemoteTimeout time.Duration
var learnMaxBytes int64
var learnAuthHeaderEnv string
var learnHFDatasets []string
var learnHFConfig string
var learnHFSplit string
var learnHFMaxRecords int
var learnURLSafety bool
var learnURLSafetyTimeout time.Duration
var learnURLSafetyCacheTTL time.Duration
var learnURLSafetyVisibility string
var learnURLSafetyFailOpen bool
var learnFromResearch string
var learnIncludeResearchSources bool
var learnIncludeResearchSummary bool

var learnCmd = &cobra.Command{
	Use:   "learn [text]",
	Short: "Add knowledge to your personal LLM's memory.",
	Long:  `This command allows you to teach your personal LLM new information by adding text, files, directories, URLs, or Hugging Face datasets into its persistent knowledge base.`,
	Run: func(cmd *cobra.Command, args []string) {
		mm, err := memory.NewMemoryManager()
		if err != nil {
			fmt.Printf("Error initializing memory manager: %v\n", err)
			return
		}
		if strings.TrimSpace(learnFromResearch) != "" {
			if err := executeLearnFromResearch(learnFromResearch, learnIncludeResearchSummary, learnIncludeResearchSources); err != nil {
				fmt.Printf("Error learning from research artifact: %v\n", err)
			}
			return
		}

		if learnDir != "" {
			session := newLearnSession("DIRECTORY", learnDir, []string{learnDir}, map[string]string{
				"recursive":     fmt.Sprintf("%t", learnRecursive),
				"chunk_chars":   fmt.Sprintf("%d", learnChunkChars),
				"chunk_overlap": fmt.Sprintf("%d", learnChunkOverlap),
			})
			opts := rag.DefaultIndexOptions()
			opts.Recursive = learnRecursive
			opts.MaxChunkChars = learnChunkChars
			opts.ChunkOverlap = learnChunkOverlap
			opts.AllTypes = learnAllTypes

			extensionInput := strings.TrimSpace(learnExtensions)
			if len(learnTypes) > 0 {
				typeCSV := strings.Join(learnTypes, ",")
				if extensionInput == "" {
					extensionInput = typeCSV
				} else {
					extensionInput += "," + typeCSV
				}
			}

			opts.Extensions = rag.ParseExtensionsCSV(extensionInput)
			if !opts.AllTypes && len(opts.Extensions) == 0 {
				opts.Extensions = rag.DefaultIndexOptions().Extensions
			}
			opts.OnFileEvent = func(event rag.FileEvent) {
				switch event.Outcome {
				case "indexed":
					fmt.Printf("[%d scanned | %d indexed] indexed %s (%d chunks)\n", event.FilesScanned, event.FilesIndexed, event.RelPath, event.Chunks)
				case "skipped-unsupported":
					fmt.Printf("[%d scanned] skipped (type filter) %s\n", event.FilesScanned, event.RelPath)
				case "skipped-binary":
					fmt.Printf("[%d scanned] skipped (binary) %s\n", event.FilesScanned, event.RelPath)
				case "parse-error":
					fmt.Printf("[%d scanned] parse error %s: %s\n", event.FilesScanned, event.RelPath, event.Error)
				case "index-error":
					fmt.Printf("[%d scanned] index error %s: %s\n", event.FilesScanned, event.RelPath, event.Error)
				case "walk-error":
					fmt.Printf("walk error %s: %s\n", event.RelPath, event.Error)
				}
			}

			fmt.Printf("Indexing directory: %s\n", learnDir)
			if opts.AllTypes {
				fmt.Println("Type filter: all file types (text-like files will be indexed; binary files will be skipped)")
			} else {
				fmt.Printf("Type filter: %s\n", rag.ExtensionsToCSV(opts.Extensions))
			}
			stats, err := rag.IndexDirectory(mm, learnDir, opts)
			if err != nil {
				fmt.Printf("Error indexing directory: %v\n", err)
				session.finish("FAILED", "Directory indexing failed.", err.Error(), map[string]int64{
					"files_scanned":  int64(stats.FilesScanned),
					"files_indexed":  int64(stats.FilesIndexed),
					"chunks_indexed": int64(stats.ChunksIndexed),
					"errors":         int64(stats.ParseErrors + stats.IndexErrors + stats.WalkErrors),
				})
				if logErr := appendLearnSessionRecord(session); logErr != nil {
					fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
				}
				return
			}
			session.finish("SUCCESS", "Directory learning completed successfully.", "", map[string]int64{
				"files_scanned":       int64(stats.FilesScanned),
				"files_indexed":       int64(stats.FilesIndexed),
				"chunks_indexed":      int64(stats.ChunksIndexed),
				"skipped_unsupported": int64(stats.SkippedUnsupported),
				"skipped_binary":      int64(stats.SkippedBinary),
				"parse_errors":        int64(stats.ParseErrors),
				"index_errors":        int64(stats.IndexErrors),
				"walk_errors":         int64(stats.WalkErrors),
			})
			if logErr := appendLearnSessionRecord(session); logErr != nil {
				fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
			}
			fmt.Println(renderDirectoryLearnSummary(learnDir, opts, stats))
			return
		}

		if len(learnURLs) > 0 || strings.TrimSpace(learnURLFile) != "" || len(learnHFDatasets) > 0 {
			remoteOpts := rag.DefaultRemoteIndexOptions()
			remoteOpts.MaxChunkChars = learnChunkChars
			remoteOpts.ChunkOverlap = learnChunkOverlap
			remoteOpts.Crawl = learnCrawl
			remoteOpts.CrawlDepth = learnCrawlDepth
			remoteOpts.MaxPages = learnMaxPages
			remoteOpts.RateLimitPerSec = learnRateLimit
			remoteOpts.UserAgent = learnUserAgent
			remoteOpts.Timeout = learnRemoteTimeout
			remoteOpts.MaxBytes = learnMaxBytes
			remoteOpts.HFToken = strings.TrimSpace(os.Getenv("HF_TOKEN"))
			remoteOpts.URLSafetyEnabled = learnURLSafety
			remoteOpts.URLSafetyTimeout = learnURLSafetyTimeout
			remoteOpts.URLSafetyCacheTTL = learnURLSafetyCacheTTL
			remoteOpts.URLSafetyVisibility = learnURLSafetyVisibility
			remoteOpts.URLSafetyFailOpen = learnURLSafetyFailOpen
			remoteOpts.URLSafetyAPIKey = strings.TrimSpace(os.Getenv("URLSCAN_API_KEY"))
			if envName := strings.TrimSpace(learnAuthHeaderEnv); envName != "" {
				remoteOpts.AuthHeader = strings.TrimSpace(os.Getenv(envName))
			}
			if len(learnAllowedDomains) > 0 {
				remoteOpts.AllowedDomains = make(map[string]bool, len(learnAllowedDomains))
				for _, d := range learnAllowedDomains {
					d = strings.ToLower(strings.TrimSpace(d))
					if d != "" {
						remoteOpts.AllowedDomains[d] = true
					}
				}
			}
			remoteOpts.OnEvent = func(ev rag.RemoteEvent) {
				switch ev.Outcome {
				case "indexed", "hf-indexed-record":
					fmt.Printf("[%d fetched | %d indexed] %s: %s\n", ev.ItemsFetched, ev.ItemsIndexed, ev.Outcome, ev.Source)
				case "safety-allowed", "safety-cache-hit", "safety-cache-miss":
					fmt.Printf("[%d fetched | %d indexed] %s: %s (%s)\n", ev.ItemsFetched, ev.ItemsIndexed, ev.Outcome, ev.Source, ev.Detail)
				case "safety-blocked", "safety-error", "preflight-failed", "skipped-domain", "invalid-url", "parse-error", "http-error", "http-status", "hf-http-error", "hf-http-status", "hf-parse-error", "index-error", "hf-index-error":
					fmt.Printf("[%d fetched | %d indexed] %s: %s (%s)\n", ev.ItemsFetched, ev.ItemsIndexed, ev.Outcome, ev.Source, ev.Detail)
				}
			}

			seedURLs, urlsErr := collectURLs(learnURLs, learnURLFile)
			if urlsErr != nil {
				fmt.Printf("Error collecting URLs: %v\n", urlsErr)
				return
			}
			sources := append([]string(nil), seedURLs...)
			for _, ds := range learnHFDatasets {
				ds = strings.TrimSpace(ds)
				if ds != "" {
					sources = append(sources, "hf:"+ds)
				}
			}
			session := newLearnSession("REMOTE", "remote", sources, map[string]string{
				"crawl":                fmt.Sprintf("%t", learnCrawl),
				"crawl_depth":          fmt.Sprintf("%d", learnCrawlDepth),
				"max_pages":            fmt.Sprintf("%d", learnMaxPages),
				"url_safety":           fmt.Sprintf("%t", learnURLSafety),
				"url_safety_fail_open": fmt.Sprintf("%t", learnURLSafetyFailOpen),
			})
			totalStats := rag.RemoteIndexStats{}
			if len(seedURLs) > 0 {
				if remoteOpts.URLSafetyEnabled && strings.TrimSpace(remoteOpts.URLSafetyAPIKey) == "" && !remoteOpts.URLSafetyFailOpen {
					fmt.Println("Error: URL safety is enabled but URLSCAN_API_KEY is not set.")
					fmt.Println("Set URLSCAN_API_KEY or run with --url-safety-fail-open.")
					session.finish("FAILED", "Remote learning aborted before execution.", "URL safety key missing", map[string]int64{
						"items_indexed": 0,
						"errors":        1,
					})
					if logErr := appendLearnSessionRecord(session); logErr != nil {
						fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
					}
					return
				}
				fmt.Printf("Indexing remote URLs: %d seed(s)\n", len(seedURLs))
				if !learnCrawl {
					fmt.Println("Crawl mode: disabled (single URL mode)")
				} else {
					fmt.Printf("Crawl mode: enabled (depth=%d, max-pages=%d, rate=%.2f req/s)\n", learnCrawlDepth, learnMaxPages, learnRateLimit)
				}
				if remoteOpts.URLSafetyEnabled {
					fmt.Printf("URL safety: enabled (visibility=%s, timeout=%s, cache-ttl=%s, fail-open=%t)\n", remoteOpts.URLSafetyVisibility, remoteOpts.URLSafetyTimeout, remoteOpts.URLSafetyCacheTTL, remoteOpts.URLSafetyFailOpen)
				} else {
					fmt.Println("URL safety: disabled")
				}
				urlStats, urlErr := rag.IndexURLs(mm, seedURLs, remoteOpts)
				totalStats = mergeRemoteStats(totalStats, urlStats)
				if urlErr != nil {
					fmt.Printf("URL indexing error: %v\n", urlErr)
				}
			}

			if len(learnHFDatasets) > 0 {
				specs := make([]rag.HFSpec, 0, len(learnHFDatasets))
				for _, ds := range learnHFDatasets {
					ds = strings.TrimSpace(ds)
					if ds == "" {
						continue
					}
					specs = append(specs, rag.HFSpec{
						DatasetID:  ds,
						Config:     strings.TrimSpace(learnHFConfig),
						Split:      strings.TrimSpace(learnHFSplit),
						MaxRecords: learnHFMaxRecords,
					})
				}
				if len(specs) > 0 {
					fmt.Printf("Indexing HF datasets: %d dataset(s)\n", len(specs))
					hfStats, hfErr := rag.IndexHFDatasets(mm, specs, remoteOpts)
					totalStats = mergeRemoteStats(totalStats, hfStats)
					if hfErr != nil {
						fmt.Printf("HF indexing error: %v\n", hfErr)
					}
				}
			}

			status := "SUCCESS"
			failureReason := ""
			if totalStats.ItemsIndexed == 0 &&
				(totalStats.HTTPErrors > 0 || totalStats.ParseErrors > 0 || totalStats.IndexErrors > 0 || totalStats.PreflightFailures > 0 || totalStats.SafetyErrors > 0 || totalStats.SafetyBlocked > 0) {
				status = "FAILED"
				failureReason = "No items were indexed."
			} else if totalStats.HTTPErrors > 0 || totalStats.ParseErrors > 0 || totalStats.IndexErrors > 0 || totalStats.PreflightFailures > 0 || totalStats.SafetyErrors > 0 || totalStats.SafetyBlocked > 0 {
				status = "PARTIAL"
			}
			session.finish(status, "Remote learning run completed.", failureReason, map[string]int64{
				"items_fetched":       int64(totalStats.ItemsFetched),
				"items_indexed":       int64(totalStats.ItemsIndexed),
				"chunks_indexed":      int64(totalStats.ChunksIndexed),
				"bytes_fetched":       totalStats.BytesFetched,
				"preflight_failures":  int64(totalStats.PreflightFailures),
				"safety_blocked":      int64(totalStats.SafetyBlocked),
				"safety_errors":       int64(totalStats.SafetyErrors),
				"safety_cache_hits":   int64(totalStats.SafetyCacheHits),
				"safety_cache_misses": int64(totalStats.SafetyCacheMisses),
				"parse_errors":        int64(totalStats.ParseErrors),
				"http_errors":         int64(totalStats.HTTPErrors),
				"index_errors":        int64(totalStats.IndexErrors),
			})
			if logErr := appendLearnSessionRecord(session); logErr != nil {
				fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
			}
			fmt.Println(renderRemoteLearnSummary(totalStats))
			return
		}

		var content string
		mode := "INLINE_TEXT"
		target := "inline"
		if learnFile != "" {
			data, err := ioutil.ReadFile(learnFile)
			if err != nil {
				fmt.Printf("Error reading file %s: %v\n", learnFile, err)
				session := newLearnSession("FILE", learnFile, []string{learnFile}, nil)
				session.finish("FAILED", "File learning failed.", err.Error(), map[string]int64{"items_indexed": 0, "errors": 1})
				if logErr := appendLearnSessionRecord(session); logErr != nil {
					fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
				}
				return
			}
			content = string(data)
			fmt.Printf("Learning from file: %s\n", learnFile)
			mode = "FILE"
			target = learnFile
		} else if len(args) > 0 {
			content = strings.Join(args, " ")
			fmt.Println("Learning from provided text.")
		} else {
			fmt.Println("Please provide text to learn or use the --file flag.")
			return
		}

		err = mm.AddKnowledge(content, nil)
		if err != nil {
			fmt.Printf("Error adding knowledge to memory: %v\n", err)
			session := newLearnSession(mode, target, []string{target}, nil)
			session.finish("FAILED", "Inline/file learning failed.", err.Error(), map[string]int64{"items_indexed": 0, "errors": 1})
			if logErr := appendLearnSessionRecord(session); logErr != nil {
				fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
			}
			return
		}
		session := newLearnSession(mode, target, []string{target}, nil)
		session.finish("SUCCESS", "Inline/file learning completed successfully.", "", map[string]int64{"items_indexed": 1, "chunks_indexed": 1})
		if logErr := appendLearnSessionRecord(session); logErr != nil {
			fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
		}

		fmt.Println(renderInlineLearnSummary(learnFile != "", learnFile))
	},
}

func init() {
	learnCmd.Flags().StringVarP(&learnFile, "file", "f", "", "Path to a file to learn from")
	learnCmd.Flags().StringVarP(&learnDir, "dir", "d", "", "Path to a directory of documents to index")
	learnCmd.Flags().BoolVarP(&learnRecursive, "recursive", "r", true, "Recursively index subdirectories when using --dir")
	learnCmd.Flags().IntVar(&learnChunkChars, "chunk-chars", 1200, "Maximum characters per indexed chunk when using --dir")
	learnCmd.Flags().IntVar(&learnChunkOverlap, "chunk-overlap", 200, "Overlap between chunks when using --dir")
	learnCmd.Flags().StringVar(&learnExtensions, "extensions", "", "Comma-separated extensions to index with --dir (e.g. .pdf,.md,.txt)")
	learnCmd.Flags().StringSliceVar(&learnTypes, "type", nil, "Repeatable file extension filter with --dir (e.g. --type .md --type .txt)")
	learnCmd.Flags().BoolVar(&learnAllTypes, "all-types", false, "Index all file types under --dir (binary files are skipped)")
	learnCmd.Flags().StringArrayVar(&learnURLs, "url", nil, "URL to index (repeatable)")
	learnCmd.Flags().StringVar(&learnURLFile, "url-file", "", "Path to a newline-delimited URL list")
	learnCmd.Flags().BoolVar(&learnCrawl, "crawl", false, "Follow links while indexing URLs")
	learnCmd.Flags().IntVar(&learnCrawlDepth, "crawl-depth", 1, "Crawl depth when --crawl is enabled")
	learnCmd.Flags().IntVar(&learnMaxPages, "max-pages", 200, "Maximum fetched pages/items per remote run")
	learnCmd.Flags().Float64Var(&learnRateLimit, "rate-limit", 2.0, "Remote request rate limit in requests/sec")
	learnCmd.Flags().StringSliceVar(&learnAllowedDomains, "allowed-domain", nil, "Allowed domain for URL crawling (repeatable)")
	learnCmd.Flags().StringVar(&learnUserAgent, "user-agent", "talos/1.0 (+https://thynaptic.com)", "User agent for remote requests")
	learnCmd.Flags().DurationVar(&learnRemoteTimeout, "remote-timeout", 20*time.Second, "Timeout for each remote request")
	learnCmd.Flags().Int64Var(&learnMaxBytes, "max-bytes", 10*1024*1024, "Maximum bytes per remote item")
	learnCmd.Flags().StringVar(&learnAuthHeaderEnv, "auth-header-env", "", "Environment variable containing Authorization header value for URL fetches")
	learnCmd.Flags().BoolVar(&learnURLSafety, "url-safety", true, "Run URL safety verdict checks before indexing URL content")
	learnCmd.Flags().DurationVar(&learnURLSafetyTimeout, "url-safety-timeout", 45*time.Second, "Timeout budget for URL safety scan verdict")
	learnCmd.Flags().DurationVar(&learnURLSafetyCacheTTL, "url-safety-cache-ttl", 24*time.Hour, "Cache TTL for URL safety verdicts")
	learnCmd.Flags().StringVar(&learnURLSafetyVisibility, "url-safety-visibility", "private", "URL safety scan visibility: private|unlisted|public")
	learnCmd.Flags().BoolVar(&learnURLSafetyFailOpen, "url-safety-fail-open", false, "Allow URL ingest when safety service errors/unavailable")
	learnCmd.Flags().StringArrayVar(&learnHFDatasets, "hf-dataset", nil, "Hugging Face dataset id to index (repeatable)")
	learnCmd.Flags().StringVar(&learnHFConfig, "hf-config", "", "Hugging Face dataset config")
	learnCmd.Flags().StringVar(&learnHFSplit, "hf-split", "train", "Hugging Face dataset split")
	learnCmd.Flags().IntVar(&learnHFMaxRecords, "hf-max-records", 100, "Maximum records per HF dataset to index")
	learnCmd.Flags().StringVar(&learnFromResearch, "from-research", "", "Ingest from a research artifact id or 'latest'")
	learnCmd.Flags().BoolVar(&learnIncludeResearchSummary, "include-research-summary", true, "Include research summary/findings text during --from-research ingest")
	learnCmd.Flags().BoolVar(&learnIncludeResearchSources, "include-research-sources", true, "Include source URL indexing during --from-research ingest")
	if f := learnCmd.Flags().Lookup("from-research"); f != nil {
		f.NoOptDefVal = "latest"
	}
	rootCmd.AddCommand(learnCmd)
}

func executeLearnFromResearch(selector string, includeSummary bool, includeSources bool) error {
	if !includeSummary && !includeSources {
		return fmt.Errorf("at least one of --include-research-summary or --include-research-sources must be true")
	}
	artifactID, err := resolveResearchArtifactID(selector)
	if err != nil {
		return err
	}
	artifact, err := loadResearchArtifactByID(artifactID)
	if err != nil {
		return err
	}
	mm, err := memory.NewMemoryManager()
	if err != nil {
		return err
	}

	session := newLearnSession("RESEARCH_CHAIN", artifactID, artifact.Sources, map[string]string{
		"artifact_id":      artifactID,
		"research_mode":    strings.ToLower(strings.TrimSpace(artifact.Mode)),
		"include_summary":  fmt.Sprintf("%t", includeSummary),
		"include_sources":  fmt.Sprintf("%t", includeSources),
		"research_status":  strings.ToUpper(strings.TrimSpace(artifact.Status)),
		"research_created": strings.TrimSpace(artifact.CreatedAt),
	})

	var sourceStats rag.RemoteIndexStats
	summaryIndexed := int64(0)
	var failureReasons []string
	if includeSummary {
		payload := buildResearchLearnPayload(artifact)
		if strings.TrimSpace(payload) != "" {
			if err := mm.AddKnowledge(payload, map[string]string{
				"source_type":   "research_artifact",
				"research_id":   artifact.SessionID,
				"research_mode": strings.ToLower(strings.TrimSpace(artifact.Mode)),
			}); err != nil {
				failureReasons = append(failureReasons, "summary ingest failed: "+err.Error())
			} else {
				summaryIndexed = 1
			}
		}
	}
	if includeSources && len(artifact.Sources) > 0 {
		opts := rag.DefaultRemoteIndexOptions()
		opts.MaxChunkChars = learnChunkChars
		opts.ChunkOverlap = learnChunkOverlap
		opts.Crawl = false
		opts.CrawlDepth = 0
		opts.MaxPages = learnMaxInt(1, learnMaxPages)
		opts.RateLimitPerSec = learnRateLimit
		opts.UserAgent = learnUserAgent
		opts.Timeout = learnRemoteTimeout
		opts.MaxBytes = learnMaxBytes
		opts.URLSafetyEnabled = learnURLSafety
		opts.URLSafetyTimeout = learnURLSafetyTimeout
		opts.URLSafetyCacheTTL = learnURLSafetyCacheTTL
		opts.URLSafetyVisibility = learnURLSafetyVisibility
		opts.URLSafetyFailOpen = learnURLSafetyFailOpen
		opts.URLSafetyAPIKey = strings.TrimSpace(os.Getenv("URLSCAN_API_KEY"))

		fmt.Printf("Indexing research sources from artifact %s: %d URL(s)\n", artifactID, len(artifact.Sources))
		if opts.URLSafetyEnabled && strings.TrimSpace(opts.URLSafetyAPIKey) == "" && !opts.URLSafetyFailOpen {
			failureReasons = append(failureReasons, "URL safety enabled but URLSCAN_API_KEY is not set")
		} else {
			stats, idxErr := rag.IndexURLs(mm, artifact.Sources, opts)
			sourceStats = mergeRemoteStats(sourceStats, stats)
			if idxErr != nil {
				failureReasons = append(failureReasons, idxErr.Error())
			}
		}
	}

	status := "SUCCESS"
	if summaryIndexed == 0 && sourceStats.ItemsIndexed == 0 {
		status = "FAILED"
	}
	if len(failureReasons) > 0 && status == "SUCCESS" {
		status = "PARTIAL"
	}
	session.finish(status, "Research artifact learning run completed.", strings.Join(failureReasons, "; "), map[string]int64{
		"artifact_findings":       int64(len(artifact.Findings)),
		"artifact_sources":        int64(len(artifact.Sources)),
		"summary_items_indexed":   summaryIndexed,
		"source_items_fetched":    int64(sourceStats.ItemsFetched),
		"source_items_indexed":    int64(sourceStats.ItemsIndexed),
		"source_chunks_indexed":   int64(sourceStats.ChunksIndexed),
		"source_preflight_errors": int64(sourceStats.PreflightFailures),
		"source_safety_blocked":   int64(sourceStats.SafetyBlocked),
		"source_safety_errors":    int64(sourceStats.SafetyErrors),
		"source_parse_errors":     int64(sourceStats.ParseErrors),
		"source_http_errors":      int64(sourceStats.HTTPErrors),
		"source_index_errors":     int64(sourceStats.IndexErrors),
	})
	if logErr := appendLearnSessionRecord(session); logErr != nil {
		fmt.Printf("Warning: Failed to write learn session log: %v\n", logErr)
	}
	fmt.Println(renderResearchLearnSummary(artifactID, includeSummary, includeSources, summaryIndexed, sourceStats, status))
	if status == "FAILED" {
		return fmt.Errorf("no research artifact content was indexed")
	}
	return nil
}

func buildResearchLearnPayload(artifact ResearchArtifact) string {
	var b strings.Builder
	b.WriteString("Research Artifact\n")
	b.WriteString("Session ID: " + strings.TrimSpace(artifact.SessionID) + "\n")
	b.WriteString("Mode: " + strings.ToUpper(strings.TrimSpace(artifact.Mode)) + "\n")
	b.WriteString("Query: " + strings.TrimSpace(artifact.Query) + "\n\n")
	b.WriteString("Executive Summary\n")
	b.WriteString(strings.TrimSpace(artifact.ExecutiveSummary) + "\n\n")
	if len(artifact.Findings) > 0 {
		b.WriteString("Findings\n")
		for i, f := range artifact.Findings {
			line := fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(f.Text))
			if len(f.Refs) > 0 {
				line += " [refs:"
				refParts := make([]string, 0, len(f.Refs))
				for _, ref := range f.Refs {
					refParts = append(refParts, fmt.Sprintf("%d", ref))
				}
				line += strings.Join(refParts, ",") + "]"
			}
			b.WriteString(line + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func renderResearchLearnSummary(artifactID string, includeSummary bool, includeSources bool, summaryIndexed int64, sourceStats rag.RemoteIndexStats, status string) string {
	var b strings.Builder
	b.WriteString("LEARN SUMMARY\n\n")
	b.WriteString("MODE\n")
	b.WriteString("  RESEARCH_CHAIN\n\n")
	b.WriteString("SOURCE\n")
	b.WriteString("  artifact_id: " + strings.TrimSpace(artifactID) + "\n")
	b.WriteString(fmt.Sprintf("  include_summary: %t\n", includeSummary))
	b.WriteString(fmt.Sprintf("  include_sources: %t\n\n", includeSources))
	b.WriteString("RESULTS\n")
	b.WriteString("  status: " + strings.ToUpper(strings.TrimSpace(status)) + "\n")
	b.WriteString(fmt.Sprintf("  summary_items_indexed: %d\n", summaryIndexed))
	b.WriteString(fmt.Sprintf("  source_items_fetched: %d\n", sourceStats.ItemsFetched))
	b.WriteString(fmt.Sprintf("  source_items_indexed: %d\n", sourceStats.ItemsIndexed))
	b.WriteString(fmt.Sprintf("  source_chunks_indexed: %d\n", sourceStats.ChunksIndexed))
	return strings.TrimRight(b.String(), "\n")
}

func learnMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func collectURLs(seed []string, filePath string) ([]string, error) {
	var out []string
	seen := make(map[string]bool)
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	for _, u := range seed {
		add(u)
	}
	if strings.TrimSpace(filePath) != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			add(sc.Text())
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func mergeRemoteStats(a, b rag.RemoteIndexStats) rag.RemoteIndexStats {
	return rag.RemoteIndexStats{
		ItemsFetched:       a.ItemsFetched + b.ItemsFetched,
		ItemsIndexed:       a.ItemsIndexed + b.ItemsIndexed,
		ChunksIndexed:      a.ChunksIndexed + b.ChunksIndexed,
		BytesFetched:       a.BytesFetched + b.BytesFetched,
		PreflightFailures:  a.PreflightFailures + b.PreflightFailures,
		SafetyBlocked:      a.SafetyBlocked + b.SafetyBlocked,
		SafetyErrors:       a.SafetyErrors + b.SafetyErrors,
		SafetyCacheHits:    a.SafetyCacheHits + b.SafetyCacheHits,
		SafetyCacheMisses:  a.SafetyCacheMisses + b.SafetyCacheMisses,
		SkippedDuplicate:   a.SkippedDuplicate + b.SkippedDuplicate,
		SkippedDomain:      a.SkippedDomain + b.SkippedDomain,
		SkippedUnsupported: a.SkippedUnsupported + b.SkippedUnsupported,
		ParseErrors:        a.ParseErrors + b.ParseErrors,
		HTTPErrors:         a.HTTPErrors + b.HTTPErrors,
		IndexErrors:        a.IndexErrors + b.IndexErrors,
	}
}

func renderDirectoryLearnSummary(dir string, opts rag.IndexOptions, stats rag.IndexStats) string {
	var b strings.Builder
	b.WriteString("LEARN SUMMARY\n\n")
	b.WriteString("MODE\n")
	b.WriteString("  DIRECTORY\n\n")
	b.WriteString("TARGET\n")
	b.WriteString("  " + strings.TrimSpace(dir) + "\n\n")
	b.WriteString("CONFIG\n")
	b.WriteString(fmt.Sprintf("  recursive: %t\n", opts.Recursive))
	b.WriteString(fmt.Sprintf("  chunk_chars: %d\n", opts.MaxChunkChars))
	b.WriteString(fmt.Sprintf("  chunk_overlap: %d\n", opts.ChunkOverlap))
	if opts.AllTypes {
		b.WriteString("  type_filter: all\n")
	} else {
		b.WriteString("  type_filter: " + normalizeExtensionsList(opts.Extensions) + "\n")
	}
	b.WriteString("\nRESULTS\n")
	b.WriteString(fmt.Sprintf("  files_scanned: %d\n", stats.FilesScanned))
	b.WriteString(fmt.Sprintf("  files_indexed: %d\n", stats.FilesIndexed))
	b.WriteString(fmt.Sprintf("  chunks_indexed: %d\n", stats.ChunksIndexed))
	b.WriteString(fmt.Sprintf("  skipped_unsupported: %d\n", stats.SkippedUnsupported))
	b.WriteString(fmt.Sprintf("  skipped_binary: %d\n", stats.SkippedBinary))
	b.WriteString(fmt.Sprintf("  parse_errors: %d\n", stats.ParseErrors))
	b.WriteString(fmt.Sprintf("  index_errors: %d\n", stats.IndexErrors))
	b.WriteString(fmt.Sprintf("  walk_errors: %d", stats.WalkErrors))
	return b.String()
}

func renderRemoteLearnSummary(stats rag.RemoteIndexStats) string {
	var b strings.Builder
	b.WriteString("LEARN SUMMARY\n\n")
	b.WriteString("MODE\n")
	b.WriteString("  REMOTE\n\n")
	b.WriteString("RESULTS\n")
	b.WriteString(fmt.Sprintf("  items_fetched: %d\n", stats.ItemsFetched))
	b.WriteString(fmt.Sprintf("  items_indexed: %d\n", stats.ItemsIndexed))
	b.WriteString(fmt.Sprintf("  chunks_indexed: %d\n", stats.ChunksIndexed))
	b.WriteString(fmt.Sprintf("  bytes_fetched: %d\n", stats.BytesFetched))
	b.WriteString(fmt.Sprintf("  preflight_failures: %d\n", stats.PreflightFailures))
	b.WriteString(fmt.Sprintf("  safety_blocked: %d\n", stats.SafetyBlocked))
	b.WriteString(fmt.Sprintf("  safety_errors: %d\n", stats.SafetyErrors))
	b.WriteString(fmt.Sprintf("  safety_cache_hits: %d\n", stats.SafetyCacheHits))
	b.WriteString(fmt.Sprintf("  safety_cache_misses: %d\n", stats.SafetyCacheMisses))
	b.WriteString(fmt.Sprintf("  skipped_duplicate: %d\n", stats.SkippedDuplicate))
	b.WriteString(fmt.Sprintf("  skipped_domain: %d\n", stats.SkippedDomain))
	b.WriteString(fmt.Sprintf("  skipped_unsupported: %d\n", stats.SkippedUnsupported))
	b.WriteString(fmt.Sprintf("  parse_errors: %d\n", stats.ParseErrors))
	b.WriteString(fmt.Sprintf("  http_errors: %d\n", stats.HTTPErrors))
	b.WriteString(fmt.Sprintf("  index_errors: %d", stats.IndexErrors))
	return b.String()
}

func renderInlineLearnSummary(fromFile bool, filePath string) string {
	var b strings.Builder
	b.WriteString("LEARN SUMMARY\n\n")
	b.WriteString("MODE\n")
	if fromFile {
		b.WriteString("  FILE\n\n")
		b.WriteString("TARGET\n")
		b.WriteString("  " + strings.TrimSpace(filePath) + "\n\n")
	} else {
		b.WriteString("  INLINE_TEXT\n\n")
	}
	b.WriteString("RESULTS\n")
	b.WriteString("  status: indexed")
	return b.String()
}

func normalizeExtensionsList(exts map[string]bool) string {
	if len(exts) == 0 {
		return "default"
	}
	ordered := make([]string, 0, len(exts))
	for ext, enabled := range exts {
		if enabled {
			ordered = append(ordered, ext)
		}
	}
	sort.Strings(ordered)
	if len(ordered) == 0 {
		return "default"
	}
	return strings.Join(ordered, ",")
}
