codex_output:normal
final_output_style:casual_concise
final_output_key_points:2-3_bullets
final_output_requires_why_section:true
final_output_constraints:concise,clear,readable,minimal
effective_from:2026-02-16

# enterprise_cli_output_schema_v1
cli_output_sections_default:COMMAND,STATUS,SOURCE|TARGET,PROGRESS,RESULTS,NEXT
cli_output_rules:
- Use stable section labels in uppercase for human scanning.
- Keep default output compact; cap long lists and show overflow indicators when needed.
- Prefer one actionable NEXT line instead of long recommendation blocks.
- Preserve verbose/detail modes for diagnostics where applicable.
