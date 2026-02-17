# P-LMv1

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Architecture](https://img.shields.io/badge/Architecture-Local%20Skills%20%2B%20GLM%20Toolserver-111111)](README.glm-toolserver.md)

P-LMv1 is the sovereign TALOS runtime: a local-first reasoning stack with:

- Local JIT skill creation (`self` capability)
- Public GLM toolserver integration for tools/plugins (`environment` capability)
- Verification-first execution with preflight gating and audit tooling

## Overview

TALOS follows a strict bifurcation:

- Skills: local, persistent, and indexed under `.skills/permanent`
- Tools: externalized through `glm-toolserver` APIs (`/tools/*`, `/admin/plugins*`)

This keeps cognition local while environmental actions remain policy-governed through the GLM server.

## Core Components

- `cmd/personal-llm`: primary operator CLI
- `cmd/benchmark`: benchmark and TALOS audit commands
- `cmd/reflex-daemon`: runtime health and anomaly monitoring
- `cmd/scout-daemon`: evidence scanning and contradiction detection
- `cmd/archive-daemon`: decision-feed archival
- `cmd/planner-daemon`: roadmap decomposition
- `cmd/compliance-daemon`: advisory compliance hardening
- `cmd/lab-assistant`: shadow fix/verify workflow

## Quick Start

### 1) Configure environment

Create `.env` in repo root:

```bash
GLM_TOOLSERVER_BASE_URL=https://chat.thynaptic.com
GLM_TOOL_BASE_URL=https://chat.thynaptic.com
GLM_CLIENT_ID=your-client-id
GLM_API_KEY=your-client-api-key
GLM_ADMIN_TOKEN=your-admin-token
GLM_ADMIN_ALLOW_REMOTE=true
```

All binaries autoload `.env` at startup.

### 2) Run TALOS CLI

```bash
go run ./cmd/personal-llm
```

### 3) Run architecture audit

```bash
go run ./cmd/benchmark talos-audit --create-skill=false --admin-writes=false --require-preflight=false
```

## Skill Lifecycle

- User skill create: `personal-llm skills create ...`
- Preflight validation: `POST /skills/preflight` (via GLM credentials)
- Persistence:
  - Skill artifact: `.skills/permanent/<skill_id>/`
  - Registry index: `.skills/permanent/index.json`
- Activation: enabled by default, reused on intent/task match before generating new skill

## Tool Lifecycle

Tool generation/execution uses GLM toolserver:

- Start generation: `POST /admin/plugins`
- Poll job: `GET /admin/plugins/jobs/{id}`
- Enable plugin: `POST /admin/plugins/{name}/enable`
- Invoke plugin: `POST /tools/plugins/{name}`

Reference: `README.glm-toolserver.md`

## Documentation

- `README_TALOS.md`: TALOS identity and operational model
- `README.glm-toolserver.md`: canonical GLM toolserver usage and API contract
- `docs/T.A.L.O.S. Sentinel Architecture Whitepaper.md`
- `docs/Aether v1.1.md`
- `docs/STRATA_ Sovereign AI Tool Foundry.md`

## Development

Run full checks:

```bash
go test ./cmd/... ./pkg/...
```

Module path: `github.com/Thynaptic/P-LMv1`
