# Changelog

All notable changes to this project will be documented in this file.

## [v0.5.0] - 2026-02-19

### Summary
API connector ingestion layer for Google Workspace (Gmail, Drive, Docs, Sheets, Slides), Notion, Project Gutenberg, and GitHub — plus chained multi-source training via `--chain`.

### Highlights
- Added `pkg/connectors/` with a formal `Connector` interface and global registry — a clean extension point for all API data sources.
- Implemented Google Workspace connector suite (`pkg/connectors/google/`): shared service-account JWT auth (`GoogleAuth`), Gmail connector, and Drive connector (auto-exports Docs/Sheets/Slides as plain text/CSV).
- Implemented Notion connector (`pkg/connectors/notion/`) for database page ingestion with block-level text extraction.
- Implemented Project Gutenberg connector (`pkg/connectors/gutenberg/`) — searches ~70k public domain books via Gutendex API and downloads full plain-text. No credentials required. Auto-paginates search results when `--book-max > 32` (one Gutendex page).
- Implemented GitHub connector (`pkg/connectors/github/`) — walks full repo file tree via `git/trees?recursive=1`, fetches and base64-decodes each text/code file. `GITHUB_TOKEN` optional (raises rate limit from 60→5000 req/hr). Supports `--github-path` subdirectory filter and `--github-max` file cap.
- Added `--chain` flag to `talos learn` for ordered multi-source training: `--chain "dir,github,hf"` runs each source in sequence, sharing one `MemoryManager`. Continues through per-source errors. Valid tokens: `file`, `dir`, `url`, `hf`, `gmail`, `drive`, `notion`, `books`/`gutenberg`, `github`, `research`.
- Added `pkg/rag/connector_indexer.go` — bridges any `Connector` into the existing memory pipeline (chunking, `AddKnowledge`, `SourceIndexedEvent`).
- Added `pkg/connectors/subagent.go` — wraps any connector as a `ProvisionedSubAgent` for autonomous invocation from daemon workflows.
- Extended `talos learn` with `--gmail-query`, `--gmail-max`, `--gdrive-folder`, `--gdrive-query`, `--gdrive-max`, `--notion-database`, `--notion-filter`, `--book-search`, `--book-id`, `--book-max`, `--github-repo`, `--github-path`, `--github-max`, `--chain` flags.
- Extended `LearnProfileConfig` with all connector and chain fields — saveable and reusable in learn profiles.
- All connector packages call `envload.Autoload()` defensively so credentials from `.env` are resolved regardless of call path (CLI, sub-agent, or test).

### Architecture
- No new external Go dependencies — Google service account JWT auth uses stdlib `crypto/rsa` + `encoding/json`. Gutenberg and GitHub connectors use only stdlib `net/http`.
- Credentials: `GOOGLE_SERVICE_ACCOUNT_JSON` (path), `GOOGLE_IMPERSONATE_USER` (domain-wide delegation), `NOTION_API_KEY`, `GITHUB_TOKEN` (optional). Gutenberg requires none. All read from shell exports or `.env` via `envload.Autoload()`.
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

