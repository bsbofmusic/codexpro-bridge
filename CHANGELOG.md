# Changelog

## 2.1.1

- Fixed a P0 Streamable HTTP session-lifecycle leak: short-lived Bridge→AgentGateway sessions now terminate on context exit instead of leaving AgentGateway stdio target subprocess groups alive.
- Updated live/surface/full-permission smoke clients to terminate their sessions as well, so verification itself cannot accumulate target processes.
- Added regression coverage requiring both MCP list and call transports to request session termination.
- Preserved the existing modular public tool surface and Work Runtime contract; this is a lifecycle/reliability bugfix, not a schema change.

## 2.1.0

- Restored the public GitHub repository as the maintained versioned source/documentation/release mirror for the current 2.1 runtime while keeping the live MCP endpoint authoritative for client schema.
- Added a lean progressive-disclosure policy to the bundled `codexpro-bridge-router`: ordinary one-shot tasks stay stateless; non-trivial tasks route/load a PRIMARY Skill first; Memory, Work, and MCP are activated only by continuity, lifecycle, or external-capability needs.
- Clarified that Work records lifecycle state/evidence only; official CodexPro/CyberKate or upstream MCP tools remain the execution planes, and final audits should be created only after material task/step state is frozen.
- Added the isolated SQLite-backed Work Runtime for task lifecycle state, immutable checkpoints/resume, project context references, deterministic audits, and artifact references.
- Added one public dispatcher, `codexpro_bridge_work`, growing the observed 2.1 release surface from 12 to 13 tools without changing any original 2.0 tool schema. This count is historical release evidence, not a future contract.
- Added task revision binding so a PASS audit becomes stale after material task, step, or artifact changes and cannot unlock completion.
- Added evidence provenance rules so manual evidence cannot silently satisfy required mechanical checks.
- Added state-only resume semantics that never replay delivery-unknown or other external mutations.
- Kept the Work database outside the source tree at `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`, with foreign keys, deterministic `user_version` migration, DELETE journal mode, and no WAL.
- Added Work Runtime fault isolation and sanitized doctor reporting; a Work database failure does not prevent the original Skills/MCP/Memory tool surface from starting.
- Preserved official CodexPro, Skills Manager, AgentGateway, Obsidian, and MemOS ownership unchanged.
- Reworked the public MCP surface to be capability-module-derived instead of count-locked; server core no longer owns a global expected-tool list.
- Added `CODEXPRO_BRIDGE_MODULES` deployment-level module hot-plug selection, fail-closed unknown selectors, duplicate module/tool rejection, installer missing/orphan detection, and surface fingerprint auditing.
- Added reduced-surface tests proving disabled-module tools disappear without compatibility aliases or residue. Tool count is telemetry; module ownership and zero missing/orphan tools are the release contract.

## 2.0.0

- Added `shared_memory` as a direct Bridge capability over full upstream Obsidian and MemOS MCP tool surfaces.
- Restored `codexpro_bridge_route_and_recall` as the canonical non-trivial entry point; memory intent is independent from Skill matching.
- Added `codexpro_bridge_memory_search`, `codexpro_bridge_memory_list`, and `codexpro_bridge_memory_call` while preserving progressive disclosure.
- Kept Skills Manager and AgentGateway as the only shared Skill/MCP sources of truth; Bridge adds no copied registry, memory database, or approval layer.
- Pinned the neutral local Obsidian MCP runtime and switched the MemOS wrapper from runtime `@latest` resolution to the installed 1.1.2 executable.
- Preserved official CodexPro unchanged as the full workspace/Bash/Git/edit execution plane.

## 1.1.0

- Removed stale build artifacts that still contained the retired agent-owned tool schema.
- Kept the live public surface at exactly eight Skills Manager + AgentGateway tools.
- Bumped the Bridge server and plugin version so reconnecting clients can register the new schema cleanly.

## 1.0.0

- Made Skills Manager the only shared Skill source.
- Made AgentGateway the only shared MCP source.
- Removed agent-specific Skill and MCP runtime dependencies from Bridge.
- Removed memory handling from Bridge; memory is intentionally out of scope.
- Added standalone Python environment, direct shared MCP transport, bounded Skill loading, and explicit rollback/deployment gates.
