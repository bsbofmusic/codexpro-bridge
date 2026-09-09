# Changelog

## 3.1.3

- Fixed `shared_skills` routing to honor managed Skill frontmatter `triggers:` as private routing metadata. This lets exact trigger phrases match inside natural Chinese text without changing the public Skill record or MCP tool schema.
- Route scoring now reads internal canonical Skill records, applies one bounded trigger-match bonus, and strips private trigger metadata before returning results. No second router, copied index, provider binding, or tool-count contract was added.
- Added a regression test for a natural Chinese request containing `人生经验`, proving `life-experience` outranks an unrelated community-research candidate while `_triggers` remains private.

## 3.1.2

- Added a reusable read-only `bridge-stress` probe for authenticated MCP surface/tool pressure tests without printing credentials or adding a second runtime.
- Added a configurable authenticated MCP request-body cap (`CODEXPRO_BRIDGE_MAX_REQUEST_BYTES`, default 4 MiB, bounded 64 KiB–16 MiB) so oversized inputs are rejected before the MCP SDK can allocate on them.
- Added HTTP idle/header bounds while preserving long-running MCP tool-call behavior: `ReadHeaderTimeout=10s`, `IdleTimeout=60s`, `MaxHeaderBytes=64 KiB`; no global write timeout was added because upstream calls may legitimately run for minutes.
- Added Bridge-local failure-domain limits: `GOMEMLIMIT=128MiB`, `MemoryHigh=192M`, `MemoryMax=256M`, `MemorySwapMax=64M`, `TasksMax=256`, `LimitNOFILE=4096`, plus a five-restarts-per-minute start limit. These limits are intentionally far above observed Bridge steady-state/pressure peaks and protect the VPS from runaway Bridge regressions without constraining normal operation.
- Pressure-tested Bridge-only traffic at 5,000 unauthenticated health requests with concurrency 100 and 1,000 authenticated MCP surface sessions with concurrency 64, both with zero failures and no Bridge restart. The 3.1.2 shadow MCP run peaked around 11.4 MiB inside the cgroup.
- Exercised shared-MCP failure pressure while the external AgentGateway system was being modified. Upstream operations failed as expected, but Bridge stayed up with `NRestarts=0`, returned to roughly 23–25 MiB steady memory, and naturally closed sessions so AgentGateway direct child processes returned to zero after the probe. Upstream tool totals remain runtime telemetry and were not frozen into tests.
- Preserved the existing public MCP URL, HTTP token, six-module/14-tool Bridge surface, Work schema 2, provider configuration model, and all 13 protected legacy tool contracts plus the accelerator extension.
- Standardized release builds on `CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags=-s` so the same source/Go toolchain produces a byte-identical binary even when the production workspace and GitHub release clone have different Git histories.

## 3.1.1

- Removed concrete provider endpoints and provider-owned paths from the systemd service template. Runtime provider details now live in `/home/agent/.config/codexpro-bridge/providers.env`, so changing Skills/MCP/Memory providers does not require editing the service unit.
- Kept MCP contents fully dynamic: Core and any configured optional AgentGateway endpoints are discovered from provider configuration and every upstream tool remains live `tools/list` data behind the stable `shared_mcp` dispatcher.
- Generalized shared-memory source handling from hard-coded `obsidian/memos` branches into a runtime source registry plus per-source optional search binding. `list`, `call`, `status`, and `search(auto)` now enumerate registered sources; a third mock `vector` source passes without adding a source-specific branch to core logic.
- Updated stdio Memory MCP client identity to use the current Bridge runtime version rather than a stale literal.
- Preserved the Bridge 3.1 public MCP contract: the original 13 protected tools plus `codexpro_bridge_accelerator` remain unchanged.

## 3.1.0

- Added the deterministic `web_accelerator` capability module while preserving the protected Python 2.1.1 contract for the original 13 public tools. The current default surface adds only `codexpro_bridge_accelerator`; tool totals remain telemetry rather than a compatibility contract.
- Upgraded Work Runtime to schema version 2 with additive migration for operation receipts, idempotency/no-replay state, conversation-to-task affinity, result references, Resume Capsules, and event-linked checkpoints. Existing tasks are preserved in place.
- Added bounded result shaping/ref storage and limited MCP sequential/parallel fan-out. Parallel mode is restricted to explicitly read-only upstream tools; Bridge still has no planner, scheduler, workflow DSL, autonomous loop, or AI runtime.
- Added graceful degradation so infrastructure failure in Skills or Memory can be isolated without hiding invalid caller input.
- Introduced the provider-neutral `capabilities.Adapter` contract with identity, kind, dependencies, traits, tools, operations, and health. Built-in systems now mount through the same generic adapter path; a mock Vision provider mechanically proves that future systems can attach without modifying Skills/MCP/Memory/Work runtimes or a central handler switch.
- Moved module dependency validation into the generic Registry so every adapter fails closed on missing dependencies instead of relying on module-specific server conditionals.
- Fixed nested MCP dispatch by starting outbound AgentGateway sessions from a clean context while preserving the tighter inbound deadline. Reusing the inbound MCP handler context could leak transport/session values into the outbound peer and produce `session not found` failures.
- Hardened lifecycle testing: MCP reliability probes must exit naturally and deterministically Close sessions; an outer hard timeout that kills the probe can strand AgentGateway stdio targets and is not valid convergence evidence.
- Revalidated production with the live Core plus optional AgentGateway endpoints, authenticated Cloudflare MCP, Work v1→v2 migration, parallel batch, and post-settle zero-child-process convergence across the active upstream gateways.
- Added CodeGraph as a required second-view structural audit for non-trivial Bridge engineering changes and validated the generic adapter boundary with a test-only Vision provider.

## 3.0.1

- Fixed `shared_skills` to consume the canonical Skills Manager CLI contract from `/home/agent/.skills-manager` instead of depending on the retired nested `skills/.skills-manager/` shadow metadata store.
- Added fail-closed validation that every Skills Manager path remains under the canonical shared Skill root, while preserving local SKILL.md hashing/frontmatter enrichment and the existing public tool schema.
- Added regression coverage proving Skill list/load works with no nested shadow metadata and rejects manager records that escape the shared Skill root.

## 3.0.0

- Replaced the active Python runtime with a Go 1.27.1 implementation while preserving the Python 2.1.1 ChatGPT-facing MCP contract byte-for-byte for all 13 current public tools.
- Kept the five capability modules (`shared_skills`, `shared_mcp`, `shared_memory`, `work_runtime`, `bridge_doctor`) and module-derived hot-plug surface; tool count remains telemetry, not a core invariant.
- Pinned the official MCP Go SDK v1.7.0 and pure-Go SQLite (`modernc.org/sqlite`), allowing `CGO_ENABLED=0` single-binary production builds.
- Preserved Work Runtime schema version 1 and production SQLite data without migration.
- Preserved AgentGateway Core + isolated XYDC routing, direct Obsidian/MemOS dispatch, full upstream permissions, no-replay `delivery_unknown` semantics, secret redaction, output bounds, and existing environment variable names.
- Disabled the Go SDK standalone SSE stream for short-lived Bridge→AgentGateway request/response sessions and disabled client reconnects; repeated real list/status/call/doctor stress returned Core and XYDC tasks/processes/RAM/swap to baseline with no persistent target group.
- Kept the public Bridge server stateless and enabled request-cancellation propagation. The SDK localhost Host-header protection is intentionally disabled because the service is loopback-only behind the authenticated Cloudflare Tunnel; Bridge token authentication remains mandatory for `/mcp`.
- Removed redundant deep-doctor MCP/Memory/Work health re-probes by reusing the first module-health result, eliminating duplicate short-lived upstream sessions inside one doctor call.
- Production artifact is one stripped Go binary at `/home/agent/.local/bin/codexpro-bridge`; Python 2.1.1 is retained only in bounded rollback snapshots, not the active service path.

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
