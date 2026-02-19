# T.A.L.O.S. Sentinel Runtime - Copilot Instructions

## Overview

T.A.L.O.S. (Thynaptic Autonomous Local Operational Sovereign) is a local-first AI reasoning runtime built on the Aether cognitive framework and S.T.R.A.T.A. tool substrate. It's a Go-based system that manages **Skills** (internal cognitive capabilities) and **Tools** (external environment actions) through a coordinated daemon ecosystem.

## Build, Test, and Lint

### Building
```bash
# Build main TALOS binary
go build ./cmd/talos

# Build specific daemon
go build ./cmd/reflex-daemon
go build ./cmd/scout-daemon
```

### Testing
```bash
# Run full test suite
go test ./cmd/... ./pkg/...

# Run tests for specific package
go test ./pkg/skills
go test ./pkg/orchestration

# Run a single test
go test -run TestJITGeneratorGenerate ./pkg/skills
go test -run TestSkillRegistry_UpsertListFind ./pkg/skills
```

### Validation
```bash
# Run TALOS audit benchmark
go run ./cmd/benchmark talos-audit --create-skill=false --admin-writes=false --require-preflight=false

# Run diagnostics
go run ./cmd/talos doctor

# Run strict pre-release checks
go run ./cmd/talos release-check
```

## Architecture: Skill vs Tool Boundary

This is the **most critical architectural concept** in T.A.L.O.S.:

### Skills (Self / Internal Cognition)
- **What**: Internal reasoning capabilities that extend T.A.L.O.S.'s cognitive abilities
- **Where**: Stored in `.skills/permanent/` with registry at `.skills/permanent/index.json`
- **Lifecycle**: JIT generation → Preflight check → Registry upsert → Reuse-before-generate on future matches
- **Implementation**: Generated Go code packages with manifest files (V3 format)
- **Key packages**: `pkg/skills/` (registry, jit_generator, preflight_gate, router)

### Tools (Environment / External Actions)
- **What**: External capabilities for environment interaction (web search, file ops, etc.)
- **Where**: Routed through S.T.R.A.T.A. (GLM toolserver) endpoints
- **Lifecycle**: Async plugin request → Poll job status → Enable → Invoke from tool endpoint
- **Implementation**: HTTP client to external toolserver (`POST /admin/plugins`, `GET /admin/plugins/jobs/{id}`)
- **Key packages**: `pkg/tools/` (admin client, oracle dispatcher, plugin policy)

### Capability Orchestration
The `pkg/capability/orchestrator.go` routes missing capabilities:
1. Policy-based routing (>0.70 confidence) decides tool vs skill
2. Falls back to LLM classifier if policy confidence is low
3. Executes selected path: skill generation via `JITGenerator` or tool provisioning via `GLMAdminClient`

## Key Conventions

### Skill Manifest Format
- Use **V3 manifest** format (see `pkg/skills/manifest_v3.go`)
- Skills require preflight verification: `POST /skills/preflight` to GLM toolserver
- Always check registry for existing matches before generating new skills
- Skill IDs follow pattern: `skill_{unix_nano_timestamp}`

### Cognitive Engine Pattern
- `pkg/cog/engine.go` implements priority-based thought processing with worker pools
- Thoughts are enqueued to cognitiveHeap (priority queue)
- Engine supports Start/Stop lifecycle with graceful shutdown
- Registry (`pkg/cog/registry.go`) tracks decision history

### Memory and RAG
- Memory manager in `pkg/memory/` handles document indexing and retrieval
- BM25 scoring for relevance ranking
- MAR (Memory-Augmented Retrieval) resolver with caching and policy controls
- Topology-based graph structures for document relationships

### Environment Loading
- **All binaries** auto-load `.env` via `pkg/envload/Autoload()` in main()
- This ensures dev/prod parity for API keys and configuration

### Output Styling
- Honor `OUTPUT_STYLE_CONTRACT.md` specifications
- Current contract: `final_output_style:casual_concise`, `final_output_key_points:2-3_bullets`
- Output package: `pkg/output/` (writer, stylist)

### Daemon Ecosystem
Each daemon has specific responsibilities:
- **reflex-daemon**: Runtime anomaly and pressure monitoring
- **scout-daemon**: Contradiction detection and evidence scanning
- **archive-daemon**: Reasoning trace archival
- **planner-daemon**: Roadmap decomposition and task handoff
- **compliance-daemon**: Advisory security hardening (non-blocking)
- **lab-assistant**: Shadow patch/verify workflows

### CLI Structure
- Main CLI: `cmd/talos/` → delegates to `pkg/taloscli/`
- Legacy entrypoint: `cmd/personal-llm/` (TALOS-compatible)
- Use Cobra for command structure (`github.com/spf13/cobra`)

### Testing Patterns
- Test files: `*_test.go` alongside implementation files
- 88 test files across the codebase
- Common test prefixes: `Test<Component>_<Behavior>` (e.g., `TestRegistryRevisionLifecycle`)
- Integration tests may require GLM toolserver mock

## Documentation Maintenance Policy

**Critical**: When changing ANY feature, command, flag, or behavior, update these in the same commit:
1. `talos --help` examples (root and command-specific)
2. `talos explain <capability>` content for affected capabilities
3. `README.md` command examples when user workflows change

No feature update is complete without synchronized documentation.

## Important Files

- `README.md` - Main documentation and quick start
- `README_TALOS.md` - Identity, mission, and philosophical foundation
- `CHANGELOG.md` - Release history and validation procedures
- `OUTPUT_STYLE_CONTRACT.md` - Current output formatting rules
- `docs/Aether v1.1.md` - Cognitive framework architecture
- `docs/STRATA_ Sovereign AI Tool Foundry.md` - Tool substrate architecture
- `docs/T.A.L.O.S. Sentinel Architecture Whitepaper.md` - System design
- `.skills/permanent/index.json` - Skill registry (runtime artifact)
- `api_keys.yaml` - API key configuration (not committed)

## Quick Reference

### Running TALOS
```bash
go run ./cmd/talos
go run ./cmd/talos --skill report2markdown chat "Convert this to markdown"
go run ./cmd/talos learn --dir ./docs
go run ./cmd/talos research run "What changed in policy this week?"
```

### Creating Skills
```bash
go run ./cmd/talos skills create --name report2markdown --description "Converts reports to markdown"
go run ./cmd/talos skills preflight --name report2markdown --description "Converts reports to markdown"
```

### Development Workflow
1. Make code changes
2. Run tests: `go test ./pkg/<modified-package>`
3. Run audit: `go run ./cmd/benchmark talos-audit ...`
4. Update help/explain documentation if user-facing
5. Update CHANGELOG.md for release-worthy changes
