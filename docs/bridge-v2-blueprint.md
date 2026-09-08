# CodexPro Bridge 2.x Shared Capability Blueprint

Status: **implemented and mechanically accepted in production**

Date: 2026-09-08

This file is the implementation contract for the production Bridge upgrade. Changes must not silently narrow permissions, create duplicate sources of truth, or move execution ownership away from upstream components.

## Goal

Make CodexPro Bridge expose the VPS shared capabilities as natural ChatGPT-facing modules while keeping each upstream system authoritative:

```text
ChatGPT Web
   |
   +-- CodexPro ---------------- workspace/files/Bash/Git/edit execution
   |
   `-- CodexPro Bridge ---------- shared capability adapter
          |
          +-- Skills ------------ Skills Manager
          +-- MCP --------------- AgentGateway Core + XYDC
          `-- Memory ------------ Obsidian MCP + MemOS MCP
```

## Non-negotiable invariants

1. **CodexPro upstream is not modified.** Keep official CodexPro in full mode; Bridge changes must not fork or patch CodexPro.
2. **Skills Manager remains the only live shared Skill truth.** Bridge reads it live; no copied index or second Skill tree.
3. **AgentGateway remains the only shared MCP registry/control plane.** Bridge dynamically lists/calls upstream tools and does not duplicate shared server definitions.
4. **Bridge must not narrow upstream MCP permissions.** If an upstream MCP exposes a tool, Bridge may list and invoke it with the original arguments. Read/write/create/delete semantics stay upstream-owned.
5. **Memory is a direct convenience plane, not a new memory store.** Bridge connects directly to the canonical Obsidian MCP and MemOS MCP. It creates no DB, vector store, ledger, or parallel ingestion path.
6. **Memory full-permission parity is required.** Obsidian and MemOS tool surfaces are not read-only filtered by Bridge.
7. **Progressive disclosure stays.** ChatGPT sees a small stable Bridge tool surface; hundreds of upstream schemas remain behind list/call dispatchers.
8. **Fault domains stay independent.** Core, XYDC, Obsidian and MemOS failures must not poison unrelated Bridge modules.
9. **No second approval/RBAC system in Bridge.** Upstream capability and ChatGPT/platform permission semantics remain authoritative.
10. **No secret duplication.** Bridge never prints or stores MemOS credentials; protected upstream wrappers/env files remain authoritative.

## Public capability surface

Target stable tools:

```text
codexpro_bridge_route_and_recall
codexpro_bridge_route
codexpro_bridge_skills_list
codexpro_bridge_load_skill
codexpro_bridge_load_skill_resource
codexpro_bridge_mcp_list
codexpro_bridge_mcp_status
codexpro_bridge_mcp_call
codexpro_bridge_memory_search
codexpro_bridge_memory_list
codexpro_bridge_memory_call
codexpro_bridge_doctor
```

`route` remains a compatibility/pure-Skill route. `route_and_recall` is the canonical first tool for non-trivial work.

## Shared Skill behavior

- Read `/home/agent/.skills-manager/skills` live.
- Route from live name/description/tags.
- Load canonical `SKILL.md` and allowed resources directly.
- A newly promoted Skill must be discoverable without Bridge source edits or a copied index.

## Shared MCP behavior

- Core: `http://127.0.0.1:19090/mcp`
- XYDC: `http://127.0.0.1:19094/mcp`
- Dynamic `tools/list` and exact `tools/call` proxying.
- No Bridge tool allowlist.
- Preserve upstream tool schemas/annotations as data returned by list operations.
- Do not replay an operation after uncertain delivery.

## Memory behavior

### Obsidian

- Canonical vault: `/home/agent/obsidian-vault`
- Use a neutral, pinned local stdio MCP runtime with full write/delete support.
- MCP operations must stay vault-contained.

### MemOS

- Canonical wrapper: `/home/agent/.local/share/agent-stack/memory/memos-api-mcp.sh`
- Credential source remains `/home/agent/.config/agent-stack/memos.env`.
- Expose the complete MemOS MCP surface, including mutation tools, without Bridge-side filtering.

### Convenience search

`memory_search` is only a shortcut. It may query Obsidian and/or MemOS and merge bounded results. It does not replace `memory_call`, which preserves full upstream capability.

## Routing behavior

For non-trivial requests, `route_and_recall` performs two independent decisions:

1. Skill routing against Skills Manager.
2. Memory intent routing against Obsidian/MemOS.

Memory recall must not depend on a Skill match. A query such as “Leah previously said what?” must still reach memory even when no Skill name contains `Leah`.

## Failure behavior

- MemOS unavailable -> report MemOS degraded; Skills/Core/XYDC/Obsidian continue.
- Obsidian unavailable -> report Obsidian degraded; Skills/Core/XYDC/MemOS continue.
- XYDC unavailable -> Core remains healthy.
- Core unavailable -> Skills and Memory remain usable.

## Implementation TODO

- [x] Freeze baseline and bounded rollback artifacts.
- [x] Pin/stabilize neutral MemOS executable path without changing memory ownership.
- [x] Install neutral pinned Obsidian MCP runtime and wrapper.
- [x] Refactor/extend MCP transport support for AgentGateway HTTP and direct-memory stdio.
- [x] Add `shared_memory` capability module.
- [x] Add `memory_list`, `memory_call`, `memory_search`.
- [x] Restore canonical `route_and_recall`; keep `route` compatibility behavior.
- [x] Update doctor to report Skills/Core/XYDC/Obsidian/MemOS independently.
- [x] Update bundled bootstrap Skill and docs.
- [x] Update tests to the new fixed public surface.
- [x] Run offline unit/compile audit.
- [x] Run live full-permission reversible write/delete tests.
- [x] Run fault-isolation tests.
- [x] Restart only Bridge after all preflight checks pass.
- [x] Verify public MCP surface and fresh independent MCP-client behavior.
- [x] Update shared operations Skills only after production parity is proven.
- [x] Produce final CodexPro and CodexPro Bridge client MCP JSON snippets in `client-mcp-config.md`.

Client-registration note: independent fresh MCP clients now initialize against the public Bridge and receive the 12-tool schema. An already-open ChatGPT conversation/connection can retain the schema it cached before the upgrade; refreshing/reconnecting that client registration is an external client action and does not change the accepted production bytes.

## Mechanical acceptance gate

Production is accepted only when all applicable checks pass:

1. CodexPro self-test has zero failures and remains full-mode.
2. Bridge pytest + compileall pass.
3. Live Skills Manager list/load succeeds.
4. Core + XYDC list/call succeeds and dynamic tools are not Bridge-hardcoded.
5. Obsidian MCP lists read and mutation tools.
6. Obsidian reversible create/read/update/delete smoke leaves no residue.
7. MemOS full tool surface includes both retrieval and mutation tools; one safe reversible mutation workflow is verified when a disposable target can be isolated.
8. `route_and_recall` recalls memory independently from Skill routing; simple arithmetic skips memory.
9. Each optional fault domain can fail without breaking unrelated modules.
10. Bridge listener remains loopback-only; public unauthenticated MCP access remains rejected.
11. No credentials appear in logs, diffs, docs, or receipts.
12. No new duplicate Skill tree, MCP registry, memory DB, or approval layer exists.
