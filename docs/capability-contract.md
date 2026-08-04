# Capability Contract

The server exposes exactly these eight public MCP tools:

| Tool | Purpose |
| --- | --- |
| codexpro_bridge_route_and_recall | Return live Skill metadata and conditionally recall Memos. |
| codexpro_bridge_skills_list | List enabled Hermes Skills. |
| codexpro_bridge_load_skill | Load the real SKILL.md of one active Skill. |
| codexpro_bridge_load_skill_resource | Load an allowed Skill reference, template, script, or asset. |
| codexpro_bridge_hermes_mcp_list | List enabled Hermes Native MCP tools after filtering. |
| codexpro_bridge_hermes_mcp_status | Return redacted MCP status and compatibility information. |
| codexpro_bridge_hermes_mcp_call | Make one allowed upstream MCP call without replay. |
| codexpro_bridge_doctor | Report Bridge module and optional deep health. |

Skill resources must be relative paths below references, templates, scripts,
or assets. Absolute paths, traversal, dotfiles, environment files, credential
directories, and common key-file extensions are rejected. Hindsight is denied
even if it is present in Hermes configuration.

Listing, routing, loading, status, and doctor are annotated read-only. The
generic upstream call is intentionally marked non-idempotent and potentially
destructive because the Bridge cannot know whether an upstream tool writes.
An ambiguous upstream result is delivery_unknown, never an automatic retry.
