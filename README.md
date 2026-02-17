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

- `cmd/personal-llm`: operator interface and primary execution path
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
go run ./cmd/personal-llm
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
