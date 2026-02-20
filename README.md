# T.A.L.O.S. Sentinel Runtime

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Architecture](https://img.shields.io/badge/Architecture-Aether%20%2B%20STRATA-111111)](docs/Aether%20v1.1.md)

T.A.L.O.S. is the sovereign sentinel runtime for Thynaptic: local-first reasoning, verified execution, and autonomous capability evolution.

## Core Model

This repository implements the boundary defined in the architecture docs:

- Skills (Self): local cognitive capability, persisted in `.skills/permanent`
- Tools (Environment): external capability routed through the S.T.R.A.T.A. substrate

This keeps reasoning local while environment actions remain policy-governed.

## Runtime Components

- `cmd/talos`: operator interface and primary execution path
- `cmd/personal-llm`: legacy-compatible TALOS entrypoint
- `cmd/benchmark`: health, audit, and capability checks
- `cmd/reflex-daemon`: runtime anomaly and pressure monitoring
- `cmd/scout-daemon`: contradiction and evidence scanning
- `cmd/archive-daemon`: reasoning trace archival
- `cmd/planner-daemon`: roadmap decomposition and handoff prep
- `cmd/compliance-daemon`: advisory hardening scans
- `cmd/lab-assistant`: shadow patch/verify workflow

## Capability Lifecycle

### Local Skills

1. Preflight policy check
2. Local synthesis
3. Registry upsert and enable
4. Reuse-on-match before any new generation

Artifacts:

- Skill files: `.skills/permanent/<skill_id>/`
- Registry index: `.skills/permanent/index.json`

### Environmental Tools

Tool requests follow the asynchronous plugin lifecycle in the substrate:

1. Request generation
2. Poll job status
3. Enable capability
4. Invoke from tool endpoint

## Quick Start

Run TALOS:

```bash
go run ./cmd/talos
```

View paged help:

```bash
talos --help
talos /next
talos find research
```

Use a specific skill for chat:

```bash
talos --skill report2markdown chat "Convert this report into markdown with headings"
```

Pin chat to a work domain namespace (zero-trust retrieval scope):

```bash
talos chat --domain talos-runtime "What changed in our toolserver policy?"
# or set a default:
PLM_CHAT_DOMAIN=talos-runtime talos chat "Summarize latest runtime learnings"
```

Run experimental TALOS-native text generation (no Ollama calls in that mode):

```bash
talos chat --text-gen talos-native "Summarize recent learn sessions"
```

Recommended namespace workflow:

```bash
# 1) Ingest domain knowledge into a namespace
talos learn --dir ./docs/runtime --namespace talos-runtime

# 2) Pin chat to that same namespace
talos chat --domain talos-runtime "What changed in our runtime architecture?"
```

Learn an entire directory (all supported default document types):

```bash
talos learn --dir ./docs
talos learn --dir ./docs --namespace talos-runtime
```

Ingest from Google Workspace (Gmail):

```bash
# Requires: GOOGLE_SERVICE_ACCOUNT_JSON=/path/to/sa.json GOOGLE_IMPERSONATE_USER=user@yourdomain.com
talos learn --gmail-query "label:inbox after:2024/01/01" --gmail-max 100
```

Ingest from Google Drive (includes Docs, Sheets, Slides):

```bash
talos learn --gdrive-folder <folder-id> --gdrive-max 200
talos learn --gdrive-query "mimeType='application/vnd.google-apps.document'"
```

Ingest from Notion:

```bash
# Requires: NOTION_API_KEY=secret_...
talos learn --notion-database <database-id>
talos learn --notion-database <database-id> --notion-filter '{"property":"Status","select":{"equals":"Done"}}'
```

Learn only specific file types in a directory:

```bash
talos learn --dir ./workspace --type .md --type .txt
```

Learn all file types in a directory (binary files are skipped):

```bash
talos learn --dir ./workspace --all-types
```

Learn from a URL:

```bash
URLSCAN_API_KEY=your_key talos learn --url https://example.com/guide
```

Learn from URLs with bounded crawling:

```bash
URLSCAN_API_KEY=your_key talos learn --url-file ./urls.txt --crawl --crawl-depth 1 --max-pages 200
```

Learn from a Hugging Face dataset:

```bash
HF_TOKEN=your_token talos learn --hf-dataset wikipedia --hf-split train --hf-max-records 100
```

Learn from a Kaggle dataset:

```bash
KAGGLE_USERNAME=your_user KAGGLE_KEY=your_key talos learn --kaggle-dataset zillow/zecon --kaggle-max-records 100
```

Generate deterministic synthetic text artifacts (JSONL) from inline/file input:

```bash
talos learn --synthetic-text-out .memory/synthetic/train.jsonl "Capture stable response examples for native text-gen"
talos learn --file ./docs/ops.md --synthetic-text-out .memory/synthetic/ops.jsonl
```

Chain multiple training sources in a fixed order:

```bash
talos learn --chain "url,books,hf,kaggle" --url https://example.com --book-search "frankenstein" --hf-dataset wikipedia --kaggle-dataset zillow/zecon
```

Create and use a reusable learn profile:

```bash
talos learn profile create --name hf-train-default --hf-dataset TeichAI/claude-4.5-opus-high-reasoning-250x --hf-config default --hf-split train
talos learn profile set-default --name hf-train-default
talos learn --profile hf-train-default
talos learn profile create --name runtime-domain --namespace talos-runtime --dir ./docs/runtime
```

Export and import unified learn/research profile bundles:

```bash
talos learn profile export --out .memory/talos_profiles_bundle.json
talos learn profile import --in .memory/talos_profiles_bundle.json --merge
```

Generate domain-specific profile baselines:

```bash
talos learn profile-gen security --set-default
talos research profile-gen compliance --set-default
```

Install TALOS for all terminals:

```bash
go run ./cmd/talos monitor install
```

Check terminal integration status:

```bash
talos monitor status
```

Remove terminal integration:

```bash
talos monitor uninstall
```

Run runtime diagnostics:

```bash
talos doctor
```

Run strict pre-release gate checks:

```bash
talos release-check
```

Run bounded research:

```bash
talos research run "What changed in dependency policy this week?"
```

Run research with a saved/default profile or dry-run:

```bash
talos research profiles set-default --name market-scan
talos research run --profile market-scan "What changed in dependency policy this week?"
talos research run --dry-run --category crypto "What changed in dependency policy this week?"
```

Run deep multi-agent research:

```bash
talos research deep "Evaluate tradeoffs between vector stores for this repository"
```

Preview learn execution without indexing:

```bash
talos learn --profile hf-train-default --dry-run
```

List recent research sessions/artifacts:

```bash
talos research sessions --last 10
```

Run a chained research-to-learn pipeline:

```bash
talos pipeline "research run 'What changed in dependency policy this week?' ; learn --from-research latest"
```

Run a skill-chained research-to-learn pipeline (per-step skill binding):

```bash
talos pipeline "research run 'What changed in dependency policy this week?' --skill analyst ; learn --from-research latest --skill memory_curator"
```

Create and preflight a user skill:

```bash
talos skills create --name report2markdown --description "Converts research reports to markdown files"
talos skills preflight --name report2markdown --description "Converts research reports to markdown files"
```

Explain a capability:

```bash
talos explain <capability>
```

Review recent learning sessions:

```bash
talos learned --last 5
```

Run audit:

```bash
go run ./cmd/benchmark talos-audit --create-skill=false --admin-writes=false --require-preflight=false
```

Run full test suite:

```bash
go test ./cmd/... ./pkg/...
```

## Documentation

- `README_TALOS.md`
- `docs/T.A.L.O.S. Sentinel Architecture Whitepaper.md`
- `docs/Aether v1.1.md`
- `docs/STRATA_ Sovereign AI Tool Foundry.md`
- `docs/RELEASE_NOTES_TEMPLATE.md`
- `CHANGELOG.md`

## CLI Documentation Maintenance Policy

When any TALOS feature, command, flag, or behavior changes, the same change must include updates to:

1. `talos --help` examples (root and command-specific help text as applicable)
2. `talos explain <capability>` content for affected capabilities
3. `README.md` command examples when user-facing workflows are impacted

No feature update is considered complete without synchronized help/explain documentation updates.
