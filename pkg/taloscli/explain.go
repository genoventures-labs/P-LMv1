package taloscli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type capabilityDoc struct {
	Name      string
	Summary   string
	Usage     []string
	Abilities []string
	Examples  []string
	Related   []string
	Aliases   []string
}

var explainCmd = &cobra.Command{
	Use:   "explain <capability>",
	Short: "Explain TALOS capability usage and behavior.",
	Long:  "Explains what a TALOS capability does, how to use it, and where it fits in the runtime.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := normalizeCapability(strings.Join(args, " "))
		doc, ok := lookupCapabilityDoc(query)
		if !ok {
			writeExplainNotFound(cmd.OutOrStdout(), query)
			return nil
		}
		writeCapabilityDoc(cmd.OutOrStdout(), doc)
		return nil
	},
}

var capabilityDocs = []capabilityDoc{
	{
		Name:    "chat",
		Summary: "Primary conversational interface for interactive or one-shot prompts.",
		Usage: []string{
			"talos chat",
			"talos chat <prompt>",
			"talos --skill <skill_id|name> chat <prompt>",
			"talos chat --cognition auto|minimal|balanced|deep <prompt>",
			"talos chat --timeout-profile quick|normal|deep <prompt>",
			"talos chat --warmup=false <prompt>",
		},
		Abilities: []string{
			"Runs interactive REPL when no prompt is supplied.",
			"Sends a single prompt and exits when prompt args are supplied.",
			"Uses routing, memory, and tool orchestration during response generation.",
			"Supports explicit skill selection with global --skill flag.",
			"Supports cognition budgeting to avoid heavy reasoning on simple prompts.",
			"Supports latency profiles and warmup behavior for chat responsiveness.",
		},
		Examples: []string{
			`talos chat "Draft a release note from recent commits"`,
			"talos chat",
		},
		Related: []string{"multi-agent", "learn", "doctor"},
	},
	{
		Name:    "learn",
		Summary: "Ingests user-provided knowledge into TALOS memory systems.",
		Usage: []string{
			"talos learn <text>",
			"talos learn --file <path>",
			"talos learn --dir <path> --extensions .md,.txt",
			"talos learn --dir <path> --type .go --type .md",
			"talos learn --dir <path> --all-types",
			"talos learn --url https://example.com/doc",
			"talos learn --url-file urls.txt --crawl --crawl-depth 1",
			"talos learn --from-research latest",
			"HF_TOKEN=... talos learn --hf-dataset wikipedia --hf-split train",
		},
		Abilities: []string{
			"Ingests direct text, file content, or directory content.",
			"Ingests remote URL content with optional bounded crawling.",
			"Runs URL safety checks (urlscan) before indexing remote URL content.",
			"Ingests persisted research artifacts (summary/findings/sources) for chained workflows.",
			"Ingests Hugging Face dataset rows with optional HF_TOKEN auth.",
			"Chunks and indexes material for future retrieval.",
			"Supports configurable chunk sizing and overlap.",
			"Reports per-file crawl progress and continues through recoverable file errors.",
		},
		Examples: []string{
			`talos learn "Runbook: restart sequence is A -> B -> C"`,
			"talos learn --file ./docs/ops.md",
		},
		Related: []string{"learn-image", "skills", "chat"},
	},
	{
		Name:    "learn-image",
		Summary: "Indexes image-based content for visual retrieval and reasoning.",
		Usage: []string{
			"talos learn-image --image <path>",
			"talos learn-image <image-path ...>",
		},
		Abilities: []string{
			"Loads one or more images into TALOS memory.",
			"Supports repeated image flags and shell globs.",
			"Improves visual context recall in later tasks.",
		},
		Examples: []string{
			"talos learn-image --image ./screenshots/failure.png",
		},
		Related: []string{"learn", "chat"},
	},
	{
		Name:    "multi-agent",
		Summary: "Runs an extended planner-researcher-verifier-synthesizer flow.",
		Usage: []string{
			"talos multi-agent <query>",
			"talos multi-agent <query> --mode planning",
			"talos multi-agent <query> --agents planner,researcher,verifier,synthesizer",
			"talos multi-agent <query> --agents all --parallel",
		},
		Abilities: []string{
			"Executes staged multi-agent analysis.",
			"Performs source extraction and verification passes.",
			"Returns synthesized output after arbitration.",
			"Supports planner-only mode for initial task decomposition.",
			"Supports targeted sub-agent execution in any requested order.",
			"Supports parallel execution when explicit agent selection is provided.",
		},
		Examples: []string{
			`talos multi-agent "What changed in dependency policy this week?"`,
			`talos multi-agent "Prepare migration sequence" --mode planning`,
			`talos multi-agent "Review architecture changes" --agents researcher,verifier,synthesizer`,
		},
		Related: []string{"chat", "research", "doctor"},
	},
	{
		Name:    "skills",
		Summary: "Manages local TALOS cognitive skills lifecycle.",
		Usage: []string{
			"talos skills list",
			"talos skills show --id <skill_id>",
			"talos skills create --name <name> --description <text>",
			"talos skills preflight --goal <intent>",
		},
		Abilities: []string{
			"Lists and inspects available skills.",
			"Creates new skills from operator requests.",
			"Runs preflight checks before synthesis or activation.",
		},
		Examples: []string{
			"talos skills list",
			"talos skills show --id talos_user_skill",
		},
		Related: []string{"learn", "tools admin"},
		Aliases: []string{"skill"},
	},
	{
		Name:    "tools admin",
		Summary: "Administers toolserver credentials, plugins, and tool generation jobs.",
		Usage: []string{
			"talos tools admin list",
			"talos tools admin create --client-id <id>",
			"talos tools admin plugins list",
			"talos tools admin toolgen generate --name <name> --goal <goal>",
		},
		Abilities: []string{
			"Provision and rotate toolserver client credentials.",
			"Install and enable/disable plugins.",
			"Request and track tool generation jobs.",
		},
		Examples: []string{
			"talos tools admin list",
			"talos tools admin plugins list",
		},
		Related: []string{"doctor", "monitor"},
		Aliases: []string{"tools", "admin", "toolserver"},
	},
	{
		Name:    "monitor",
		Summary: "Installs and manages TALOS shell integration across terminals.",
		Usage: []string{
			"talos monitor install",
			"talos monitor status",
			"talos monitor uninstall",
		},
		Abilities: []string{
			"Builds and installs talos binary into a PATH location.",
			"Can modify shell profile PATH blocks for bash/zsh.",
			"Reports integration status and reverses integration cleanly.",
		},
		Examples: []string{
			"talos monitor install",
			"talos monitor status",
		},
		Related: []string{"doctor", "about"},
	},
	{
		Name:    "doctor",
		Summary: "Runs runtime diagnostics and connectivity checks.",
		Usage: []string{
			"talos doctor",
			"talos doctor --no-network",
			"talos doctor --timeout 5s",
		},
		Abilities: []string{
			"Checks environment, local files, and writable runtime paths.",
			"Optionally probes Ollama and toolserver endpoints.",
			"Reports actionable PASS/WARN/FAIL diagnostics.",
		},
		Examples: []string{
			"talos doctor",
			"talos doctor --no-network",
		},
		Related: []string{"monitor", "tools admin"},
	},
	{
		Name:    "research",
		Summary: "Structured research workflows with bounded `run` and advanced `deep` modes.",
		Usage: []string{
			"talos research run <query>",
			"talos research deep <query>",
			"talos research sessions --last 10",
			"talos research deep <query> --max-pages 120 --max-research-loops 8",
		},
		Abilities: []string{
			"Run mode uses a lighter tool-assisted research workflow.",
			"Deep mode uses expanded depth and multi-agent pipeline execution.",
			"Persists session artifacts for chaining and later ingestion.",
			"Returns a professional ASCII report with findings, risks, actions, and numbered citations.",
		},
		Examples: []string{
			`talos research run "What changed in Kubernetes security this month?"`,
			`talos research deep "Evaluate tradeoffs of vector DB choices for this codebase"`,
		},
		Related: []string{"multi-agent", "doctor", "learn"},
	},
	{
		Name:    "pipeline",
		Summary: "Executes strict, allowlisted command chains with structured handoff.",
		Usage: []string{
			`talos pipeline "research run 'query' --skill analyst ; learn --from-research latest --skill memory_curator"`,
			`talos pipeline "research run 'query' ; learn --from-research latest"`,
			`talos "research deep 'query' ; learn --from-research latest"`,
		},
		Abilities: []string{
			"Parses semicolon/pipe-separated step chains from a single quoted argument.",
			"Enforces allowlisted transitions only (v1: research run/deep -> learn --from-research).",
			"Supports per-step --skill bindings with fail-fast validation for missing/disabled skills.",
			"Rejects global --skill for chain mode to keep skill scope explicit per step.",
			"Stops on first step failure and reports the failing step.",
		},
		Examples: []string{
			`talos pipeline "research run 'incident review checklist' --skill researcher_v1 ; learn --from-research latest --skill kb_ingestor"`,
			`talos pipeline "research run 'incident review checklist' ; learn --from-research latest"`,
		},
		Related: []string{"research", "learn"},
	},
	{
		Name:    "learned",
		Summary: "Review what TALOS learned across recent learn/training sessions.",
		Usage: []string{
			"talos learned",
			"talos learned --last 5",
			"talos learned --status failed",
			"talos learned --json",
		},
		Abilities: []string{
			"Shows recent learn sessions across inline, file, directory, remote, and image learning modes.",
			"Reports session status, metrics, top sources, and concise observations.",
			"Supports status/mode filters and JSON output for automation.",
		},
		Examples: []string{
			"talos learned --last 5",
			"talos learned --status success",
		},
		Related: []string{"learn", "learn-image"},
	},
	{
		Name:    "about",
		Summary: "Displays TALOS identity, mission, and architecture context.",
		Usage: []string{
			"talos about",
		},
		Abilities: []string{
			"Provides architecture overview from local repository documents.",
			"Summarizes mission, layers, and operating model.",
		},
		Examples: []string{
			"talos about",
		},
		Related: []string{"explain", "doctor"},
	},
	{
		Name:    "version",
		Summary: "Displays TALOS build and runtime version metadata.",
		Usage: []string{
			"talos version",
		},
		Abilities: []string{
			"Prints semantic version, commit, and build date metadata.",
			"Prints runtime target information (OS/architecture).",
		},
		Examples: []string{
			"talos version",
		},
		Related: []string{"update", "doctor"},
	},
	{
		Name:    "completion",
		Summary: "Generates shell completion scripts for TALOS commands and flags.",
		Usage: []string{
			"talos completion bash",
			"talos completion zsh",
			"talos completion fish",
			"talos completion powershell",
		},
		Abilities: []string{
			"Outputs shell-specific completion scripts to stdout.",
			"Supports Bash, Zsh, Fish, and PowerShell completions.",
			"Improves operator speed and reduces command/flag typing errors.",
		},
		Examples: []string{
			"talos completion bash > ~/.local/share/bash-completion/completions/talos",
			"talos completion zsh > ~/.zfunc/_talos",
		},
		Related: []string{"monitor", "version", "update"},
		Aliases: []string{"completions", "shell completion"},
	},
	{
		Name:    "update",
		Summary: "Checks for updates and applies new TALOS versions.",
		Usage: []string{
			"talos update check",
			"talos update apply",
			"talos update auto --enable --interval 24h",
			"talos update auto --apply",
		},
		Abilities: []string{
			"Checks current installed TALOS version against latest module version.",
			"Applies updates through the Go install path.",
			"Configures periodic auto-update checks with optional auto-apply.",
			"Persists auto-update preferences and last-check metadata in local runtime state.",
		},
		Examples: []string{
			"talos update check",
			"talos update auto --enable --interval 24h",
		},
		Related: []string{"version", "doctor", "monitor"},
	},
	{
		Name:    "benchmark",
		Summary: "Runs model benchmark profiles and full recommendation orchestration.",
		Usage: []string{
			"talos benchmark run",
			"talos benchmark toolcall",
			"talos benchmark cognition",
			"talos benchmark full",
			"talos benchmark load --model llama3.2:latest --concurrency 4",
		},
		Abilities: []string{
			"Benchmarks standard, chat, json, codegen, tool-call, and cognition profiles.",
			"Runs full orchestration to produce ranking scores and recommended model list.",
			"Supports load testing for one model with configurable concurrency.",
		},
		Examples: []string{
			"talos benchmark full",
			"talos benchmark toolcall",
		},
		Related: []string{"doctor", "research"},
	},
	{
		Name:    "documentary",
		Summary: "Runs documentary provisioning and daemon workflows.",
		Usage: []string{
			"talos documentary provision",
			"talos documentary daemon",
		},
		Abilities: []string{
			"Provisions documentary runtime artifacts.",
			"Executes documentary daemon processing loop.",
		},
		Examples: []string{
			"talos documentary provision",
		},
		Related: []string{"archive-daemon", "scout-daemon"},
	},
	{
		Name:    "debug-memory",
		Summary: "Inspects and prints memory diagnostics for troubleshooting.",
		Usage: []string{
			"talos debug-memory",
		},
		Abilities: []string{
			"Shows memory state for debugging and validation.",
		},
		Examples: []string{
			"talos debug-memory",
		},
		Related: []string{"doctor", "learn"},
	},
}

func normalizeCapability(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func lookupCapabilityDoc(query string) (capabilityDoc, bool) {
	for _, doc := range capabilityDocs {
		if normalizeCapability(doc.Name) == query {
			return doc, true
		}
		for _, a := range doc.Aliases {
			if normalizeCapability(a) == query {
				return doc, true
			}
		}
	}
	return capabilityDoc{}, false
}

func writeCapabilityDoc(w io.Writer, doc capabilityDoc) {
	_, _ = fmt.Fprintf(w, "TALOS EXPLAIN\n\nCAPABILITY\n  %s\n\nSUMMARY\n  %s\n", doc.Name, doc.Summary)
	if len(doc.Usage) > 0 {
		_, _ = io.WriteString(w, "\nUSAGE\n")
		for _, u := range doc.Usage {
			_, _ = fmt.Fprintf(w, "  %s\n", u)
		}
	}
	if len(doc.Abilities) > 0 {
		_, _ = io.WriteString(w, "\nFUNCTIONALITY\n")
		for i, f := range doc.Abilities {
			_, _ = fmt.Fprintf(w, "  %d. %s\n", i+1, f)
		}
	}
	if len(doc.Examples) > 0 {
		_, _ = io.WriteString(w, "\nEXAMPLES\n")
		for _, ex := range doc.Examples {
			_, _ = fmt.Fprintf(w, "  %s\n", ex)
		}
	}
	if len(doc.Related) > 0 {
		_, _ = io.WriteString(w, "\nRELATED\n")
		for _, rel := range doc.Related {
			_, _ = fmt.Fprintf(w, "  talos explain %s\n", rel)
		}
	}
}

func writeExplainNotFound(w io.Writer, query string) {
	names := make([]string, 0, len(capabilityDocs))
	for _, d := range capabilityDocs {
		names = append(names, d.Name)
	}
	sort.Strings(names)

	_, _ = fmt.Fprintf(w, "TALOS EXPLAIN\n\nERROR\n  Unknown capability: %s\n", query)
	_, _ = io.WriteString(w, "\nAVAILABLE CAPABILITIES\n")
	for _, n := range names {
		_, _ = fmt.Fprintf(w, "  %s\n", n)
	}
	_, _ = io.WriteString(w, "\nTIP\n  Use: talos explain <capability>\n")
}

func init() {
	rootCmd.AddCommand(explainCmd)
}
