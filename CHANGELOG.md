# Changelog

All notable changes to this project will be documented in this file.

## [v0.7.0] - 2026-02-20

### Summary
Zero-trust domain context release: chat can now be explicitly pinned to a work domain namespace, and learn can ingest directly into that namespace for deterministic, isolated retrieval.

### Highlights
- Added `talos chat --domain <name>` with `PLM_CHAT_DOMAIN` fallback to pin retrieval/memory context to a single namespace per chat run.
- Integrated domain pinning with strict zero-trust gating so ungrounded responses in the pinned namespace are blocked with the standard insufficiency response.
- Added `talos learn --namespace <name>` so directory/file/url/connector ingestion can populate explicit work domains.
- Extended learn profile serialization/apply/visited-flag restoration to include `namespace`.
- Updated `talos explain` docs for `chat` and `learn` with domain/namespace examples and behavior notes.

### Validation
- `go test ./pkg/taloscli`
- `go test ./cmd/... ./pkg/...`

## [v0.6.0] - 2026-02-19

### Summary
Enterprise UX and ingestion reliability release: cleaner, more uniform CLI output across TALOS surfaces plus safer URL crawl behavior, Kaggle/HF chain parity, and expanded explain/help coverage.

### Highlights
- Standardized user-facing reports to stable sectioned layouts (`COMMAND`, `STATUS`, `SOURCE/TARGET`, `PROGRESS`, `RESULTS`, `NEXT`) across learn/research/doctor/explain and related dry-run/session/report commands.
- Added friendly learn output as default with `--verbose` toggle for technical detail output; friendly summaries now preserve source and progress visibility.
- Added explain compactness controls (capped lists with overflow indicators) and new capability coverage for chaining patterns.
- Improved monitor reboot discoverability with explicit reboot examples and alias support (`restart`).
- Hardened URL ingest/crawl policy: default HTML text-only indexing; strict extension filtering when `--extensions` is provided, including crawl-link filtering.
- Added Kaggle dataset ingestion support in `learn` to align with existing HF workflows, including profile/chain wiring and env-based credential resolution.
- Expanded output governance tests for standardized rendering sections and updated CLI output contract guidance.

### Validation
- `go test ./pkg/taloscli`
- `go test ./cmd/... ./pkg/...`

## [v0.5.0] - 2026-02-19

### Summary
API connector ingestion layer for Google Workspace (Gmail, Drive, Docs, Sheets, Slides), Notion, Project Gutenberg, GitHub, and Kaggle datasets — plus chained multi-source training via `--chain`.

### Highlights
- Added `pkg/connectors/` with a formal `Connector` interface and global registry — a clean extension point for all API data sources.
- Implemented Google Workspace connector suite (`pkg/connectors/google/`): shared service-account JWT auth (`GoogleAuth`), Gmail connector, and Drive connector (auto-exports Docs/Sheets/Slides as plain text/CSV).
- Implemented Notion connector (`pkg/connectors/notion/`) for database page ingestion with block-level text extraction.
- Implemented Project Gutenberg connector (`pkg/connectors/gutenberg/`) — searches ~70k public domain books via Gutendex API and downloads full plain-text. No credentials required. Auto-paginates search results when `--book-max > 32` (one Gutendex page).
- Implemented GitHub connector (`pkg/connectors/github/`) — walks full repo file tree via `git/trees?recursive=1`, fetches and base64-decodes each text/code file. `GITHUB_TOKEN` optional (raises rate limit from 60→5000 req/hr). Supports `--github-path` subdirectory filter and `--github-max` file cap.
- Added Kaggle dataset ingest path in remote indexing (`IndexKaggleDatasets`) with row-capped parsing for CSV/TSV/JSONL/TXT/MD files; credentials from `KAGGLE_USERNAME` + `KAGGLE_KEY` or `KAGGLE_API_KEY` alias.
- Added `--chain` flag to `talos learn` for ordered multi-source training: `--chain "dir,github,hf,kaggle"` runs each source in sequence, sharing one `MemoryManager`. Continues through per-source errors. Valid tokens: `file`, `dir`, `url`, `hf`, `kaggle`, `gmail`, `drive`, `notion`, `books`/`gutenberg`, `github`, `research`.
- Added `pkg/rag/connector_indexer.go` — bridges any `Connector` into the existing memory pipeline (chunking, `AddKnowledge`, `SourceIndexedEvent`).
- Added `pkg/connectors/subagent.go` — wraps any connector as a `ProvisionedSubAgent` for autonomous invocation from daemon workflows.
- Extended `talos learn` with `--gmail-query`, `--gmail-max`, `--gdrive-folder`, `--gdrive-query`, `--gdrive-max`, `--notion-database`, `--notion-filter`, `--book-search`, `--book-id`, `--book-max`, `--github-repo`, `--github-path`, `--github-max`, `--kaggle-dataset`, `--kaggle-file`, `--kaggle-max-records`, and `--chain` flags.
- Extended `LearnProfileConfig` with all connector and chain fields — saveable and reusable in learn profiles.
- All connector packages call `envload.Autoload()` defensively so credentials from `.env` are resolved regardless of call path (CLI, sub-agent, or test).

### Architecture
- No new external Go dependencies — Google service account JWT auth uses stdlib `crypto/rsa` + `encoding/json`. Gutenberg and GitHub connectors use only stdlib `net/http`.
- Credentials: `GOOGLE_SERVICE_ACCOUNT_JSON` (path), `GOOGLE_IMPERSONATE_USER` (domain-wide delegation), `NOTION_API_KEY`, `GITHUB_TOKEN` (optional), and `KAGGLE_USERNAME` + `KAGGLE_KEY` (or `KAGGLE_API_KEY` alias). Gutenberg requires none. All read from shell exports or `.env` via `envload.Autoload()`.
- Drive connector routes by MIME type: Google Docs → `text/plain` export, Sheets → `text/csv`, Slides → `text/plain`, regular files → direct download, binary files → skipped.
- Gutenberg connector preference order for format: `text/plain; charset=utf-8` → `text/plain` → any `text/plain` variant. Paginates across Gutendex pages automatically.
- GitHub connector resolves default branch via `/repos/{owner}/{repo}`, walks full tree via `/git/trees/{branch}?recursive=1`, fetches each text/code file via `/contents/{path}` and base64-decodes the response. Binary extensions are skipped via heuristic list.
- `--chain` executes sources in a single pass sharing one `MemoryManager`. Each source step uses its already-set flags; no step is required. Errors are collected and reported; the chain does not abort on a single failure.

### Validation
- `go test ./cmd/... ./pkg/...`
- `go run ./cmd/benchmark talos-audit --create-skill=false --admin-writes=false --require-preflight=false`

## [v0.4.0] - 2026-02-18

### Summary
Initial public baseline for TALOS on P-LMv1 with local skill JIT, GLM toolserver-first tool lifecycle, global `.env` autoload across binaries, and full removal of Foundry runtime/daemon paths.

### Highlights
- Added local skill registry with enabled index (`.skills/permanent/index.json`) and reuse-before-generate matching.
- Enforced skill preflight verification through GLM (`POST /skills/preflight`) for user and orchestrated skill creation paths.
- Standardized tool generation to async plugin lifecycle (`/admin/plugins` + `/admin/plugins/jobs/{id}`) with compatibility fallback support.
- Added global `.env` autoload for all binaries and benchmark parity with production config behavior.
- Purged Foundry-specific runtime/code/docs references from active source tree (`cmd/`, `pkg/`, `internal/`), including daemon/client removal.

### Architecture and Behavior Changes
- Skills are now the canonical local self-capability mechanism.
  Impact: TALOS can evolve internal capability without external runtime listeners.
- Tools are now sourced through GLM toolserver endpoints only.
  Impact: environmental capability execution aligns with external API contract and centralized policy.
- Foundry runtime routes were removed.
  Impact: no `foundry`/`foundryd` dependency in active runtime flows.

### Validation
- `go test ./cmd/... ./pkg/...`
- `go run ./cmd/benchmark talos-audit --create-skill=false --admin-writes=false --require-preflight=false`
