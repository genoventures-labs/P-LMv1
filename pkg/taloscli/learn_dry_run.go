package taloscli

import (
	"fmt"
	"strings"
)

func renderLearnDryRunPlan(args []string) (string, error) {
	mode := "INLINE_TEXT"
	target := "inline"
	notes := []string{}

	if strings.TrimSpace(learnFromResearch) != "" {
		mode = "RESEARCH_CHAIN"
		target = strings.TrimSpace(learnFromResearch)
		notes = append(notes,
			fmt.Sprintf("include_research_summary=%t", learnIncludeResearchSummary),
			fmt.Sprintf("include_research_sources=%t", learnIncludeResearchSources),
		)
	} else if strings.TrimSpace(learnDir) != "" {
		mode = "DIRECTORY"
		target = strings.TrimSpace(learnDir)
		notes = append(notes,
			fmt.Sprintf("recursive=%t", learnRecursive),
			fmt.Sprintf("all_types=%t", learnAllTypes),
		)
	} else if len(learnURLs) > 0 || strings.TrimSpace(learnURLFile) != "" || len(learnHFDatasets) > 0 {
		mode = "REMOTE"
		target = "remote sources"
		if len(learnURLs) > 0 {
			notes = append(notes, fmt.Sprintf("seed_urls=%d", len(learnURLs)))
		}
		if strings.TrimSpace(learnURLFile) != "" {
			notes = append(notes, "url_file="+strings.TrimSpace(learnURLFile))
		}
		if len(learnHFDatasets) > 0 {
			notes = append(notes, fmt.Sprintf("hf_datasets=%d", len(learnHFDatasets)))
		}
		notes = append(notes,
			fmt.Sprintf("crawl=%t", learnCrawl),
			fmt.Sprintf("crawl_depth=%d", learnCrawlDepth),
			fmt.Sprintf("max_pages=%d", learnMaxPages),
			fmt.Sprintf("url_safety=%t", learnURLSafety),
		)
	} else if strings.TrimSpace(learnFile) != "" {
		mode = "FILE"
		target = strings.TrimSpace(learnFile)
	} else if len(args) > 0 {
		mode = "INLINE_TEXT"
		target = "inline"
		notes = append(notes, fmt.Sprintf("inline_chars=%d", len(strings.TrimSpace(strings.Join(args, " ")))))
	} else {
		return "", fmt.Errorf("no learn input resolved; provide text, --file, --dir, --url/--hf-dataset, or --from-research")
	}

	var b strings.Builder
	b.WriteString("TALOS LEARN DRY RUN\n\n")
	b.WriteString("MODE\n")
	b.WriteString("  " + mode + "\n\n")
	b.WriteString("TARGET\n")
	b.WriteString("  " + target + "\n\n")
	b.WriteString("PROFILE\n")
	b.WriteString("  selected: " + emptyAsNA(learnProfileApplied) + "\n")
	b.WriteString("  requested: " + emptyAsNA(learnProfile) + "\n\n")
	b.WriteString("CONFIG\n")
	b.WriteString(fmt.Sprintf("  chunk_chars: %d\n", learnChunkChars))
	b.WriteString(fmt.Sprintf("  chunk_overlap: %d\n", learnChunkOverlap))
	b.WriteString(fmt.Sprintf("  max_pages: %d\n", learnMaxPages))
	b.WriteString(fmt.Sprintf("  remote_timeout: %s\n", learnRemoteTimeout))
	b.WriteString(fmt.Sprintf("  max_bytes: %d\n", learnMaxBytes))
	if len(notes) > 0 {
		b.WriteString("\nNOTES\n")
		for _, n := range notes {
			b.WriteString("  - " + n + "\n")
		}
	}
	b.WriteString("\nEXECUTION\n")
	b.WriteString("  skipped: true\n")
	b.WriteString("  reason: dry-run mode\n")
	return strings.TrimRight(b.String(), "\n"), nil
}
