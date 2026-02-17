# Changelog

All notable changes to this project will be documented in this file.

## [v0.1.0] - 2026-02-17

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

