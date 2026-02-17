# Release Notes Template

Use this template for every release note to keep style and structure consistent.

## [vX.Y.Z] - YYYY-MM-DD

### Summary
One paragraph describing the release objective and scope.

### Highlights
- Key capability added or changed.
- Key reliability/performance/security improvement.
- Key operator/developer workflow improvement.

### Architecture and Behavior Changes
- Change:
  Impact:
- Change:
  Impact:

### Interfaces and APIs
- Added:
- Changed:
- Removed:

### Migration Notes
- Required environment/config changes.
- Behavior changes that may affect existing automations.

### Validation
- Commands run:
  - `go test ./cmd/... ./pkg/...`
- Operational checks:
  - `go run ./cmd/benchmark talos-audit ...`

### Known Limitations
- Any deferred cleanup or known non-blocking issues.

### Full Scope
- Link to PR/commit range/tag compare.

