# CodexPro Bridge 3.0 Go Migration Plan

Date: 2026-09-09

## Goal

Replace the Python 2.1.1 runtime with a lean Go implementation while keeping the ChatGPT-facing and upstream-facing contracts stable. Build and validate the Go implementation in parallel first. Do not switch production until all mechanical gates pass.

## Fixed baseline

- Go: 1.27.1 linux/amd64, verified against the official SHA-256.
- MCP SDK: official `github.com/modelcontextprotocol/go-sdk`, pin a current stable v1.x release that supports MCP 2026-07-28. Prefer the latest tagged stable release discoverable by the Go toolchain; record the exact version in go.mod/go.sum and the release receipt.
- Production Python Bridge remains on 127.0.0.1:18787 until cutover.
- Go shadow listener: default 127.0.0.1:18788 during migration.
- Preserve public host/tunnel/auth/database/upstream endpoints at cutover.

## Immutable architecture boundaries

Bridge remains a thin ChatGPT-facing adapter plus lightweight durable work-state layer. It must not own the shared Skill library, shared MCP registry, memory ingestion/storage, workspace execution, browser runner, scheduler, ToolHive orchestration, or agent orchestration.

Keep the five capability modules and module-derived surface:

1. shared_skills
2. shared_mcp
3. shared_memory
4. work_runtime
5. bridge_doctor

Tool count is telemetry, not a fixed core invariant. The current default surface is 13 tools. The registry must derive the expected tool union from enabled modules and fail closed on duplicate module IDs, duplicate tool names, undeclared registrations, unknown module selectors, missing tools, or orphan tools.

`CODEXPRO_BRIDGE_MODULES` semantics must remain: unset/empty/*/all/auto enables the catalog; comma-separated module IDs select modules; unknown IDs fail startup.

## Public tool contract to preserve

Current default public tools:

- codexpro_bridge_route_and_recall
- codexpro_bridge_route
- codexpro_bridge_skills_list
- codexpro_bridge_load_skill
- codexpro_bridge_load_skill_resource
- codexpro_bridge_mcp_list
- codexpro_bridge_mcp_status
- codexpro_bridge_mcp_call
- codexpro_bridge_memory_search
- codexpro_bridge_memory_list
- codexpro_bridge_memory_call
- codexpro_bridge_work
- codexpro_bridge_doctor

Create a canonical contract fixture from the existing Python server before final cutover. Compare name, description, annotations and inputSchema semantically/canonically. Do not rely only on tool count or names.

Tool annotations must retain the existing intent:

- read-only tools: readOnly=true, destructive=false, idempotent=true, openWorld=false
- upstream dispatch: readOnly=false, destructive=true, idempotent=false, openWorld=true
- Work state dispatch: readOnly=false, destructive=true, idempotent=false, openWorld=false

## HTTP/auth contract

- loopback-only host validation
- `/health` remains unauthenticated and sanitized
- `/mcp` requires the configured token unless anonymous mode is explicitly enabled
- accept token from `codexpro_token` query parameter or Bearer Authorization header
- compare tokens safely
- access logs must not leak query tokens
- public unauthenticated MCP must be rejected
- keep max-output bounding and secret redaction behavior

## Configuration compatibility

Retain the current environment names and defaults:

- CODEXPRO_BRIDGE_HOST
- CODEXPRO_BRIDGE_PORT
- CODEXPRO_BRIDGE_HTTP_TOKEN / CODEXPRO_HTTP_TOKEN fallback
- CODEXPRO_BRIDGE_ALLOW_ANONYMOUS
- CODEXPRO_BRIDGE_SKILLS_ROOT
- CODEXPRO_BRIDGE_MCP_URL
- CODEXPRO_BRIDGE_MCP_OPTIONAL_URLS
- CODEXPRO_BRIDGE_MCP_TIMEOUT
- CODEXPRO_BRIDGE_OBSIDIAN_MCP_COMMAND
- CODEXPRO_BRIDGE_MEMOS_MCP_COMMAND
- CODEXPRO_BRIDGE_MEMORY_TIMEOUT
- CODEXPRO_BRIDGE_WORK_DB_PATH
- CODEXPRO_BRIDGE_MODULES
- CODEXPRO_BRIDGE_MAX_OUTPUT_CHARS

Keep the same loopback checks, token minimum length, absolute path checks and timeout/output bounds.

## shared_skills

- Skills Manager metadata remains the membership authority under `<skills_root>/.skills-manager/skills/*.json`.
- SKILL.md remains content authority.
- Parse YAML front matter safely.
- Preserve path traversal/drive/absolute path rejection.
- Preserve allowed resource roots: references, templates, scripts, assets, agents, prompts, evals.
- Preserve sensitive component/suffix rejection.
- Preserve SHA-256 reporting and bounded text output.
- Routing behavior and result shape must remain compatible with the Python implementation.

## shared_mcp

Use the official MCP Go SDK as a client to AgentGateway Core and optional endpoints.

- Primary default: http://127.0.0.1:19090/mcp
- Optional current endpoint: http://127.0.0.1:19094/mcp
- Keep dynamic upstream tools behind list/status/call dispatch; never re-export all upstream tools as first-class Bridge actions.
- list merges primary then optional endpoints, de-duplicates by tool name, primary wins.
- optional endpoint failure degrades only that optional endpoint.
- call selects the endpoint exposing the named tool.
- never replay a call after delivery may have happened; preserve `delivery_unknown` semantics.
- short-lived sessions/requests must close/terminate correctly. Resource convergence after repeated list/call/doctor is a P0 gate.
- Prefer the current 2026-07-28 stateless MCP model when compatible with AgentGateway, while retaining compatibility with the negotiated upstream version.

## shared_memory

Use the official MCP Go SDK child-process/stdio transport or an equally direct protocol implementation if the current official SDK transport is the correct supported primitive.

Sources:

- obsidian: /home/agent/.local/share/agent-stack/memory/obsidian-mcp.sh
- memos: /home/agent/.local/share/agent-stack/memory/memos-api-mcp.sh

Requirements:

- short-lived child sessions/processes
- deterministic cleanup on normal return, timeout and cancellation
- full upstream tool parity via memory_list/memory_call
- no Bridge-side permission downgrade
- memory_search keeps current convenience mapping: Obsidian search_content; MemOS search_memory
- recall_if_needed keeps the current strong memory-signal behavior
- no ingestion/index/backup ownership

## Work Runtime SQLite compatibility

The existing DB is canonical and must remain usable without destructive migration:

`/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`

Keep schema version 1 and current tables/columns/indexes/semantics unless a migration is genuinely required. Prefer no schema change for the language migration.

Connection invariants:

- foreign_keys=ON
- busy_timeout=5000
- journal_mode=DELETE
- PRAGMA user_version deterministic
- Work DB failure degrades Work only and must not prevent Skills/MCP/Memory startup

Preserve all current Work operations, state transitions, revision/audit binding, immutable checkpoints, state-only resume, artifact verification rules, JSON field semantics and error codes.

Choose a SQLite dependency that can produce a clean static-ish Linux binary without requiring a production C toolchain. Prefer a pure-Go implementation if its compatibility with this DB and required pragmas is proven by tests. If CGO is unavoidable, document and justify it; do not silently introduce a runtime dependency.

## Go design constraints

- Keep the implementation small and boring.
- Prefer standard library types and explicit structs/functions.
- Avoid framework layers that duplicate MCP SDK or net/http ownership.
- Avoid a second registry/router/state database.
- Do not add background daemons or schedulers.
- Avoid package proliferation; package boundaries should follow the five modules plus core/server/work as needed.
- Build with reproducible version metadata and `-trimpath`.
- Target `CGO_ENABLED=0` if SQLite choice passes compatibility tests.
- The production artifact must be one stable binary plus external runtime data/config.

## Testing and gates

Add Go tests that cover at least:

- config validation
- auth and token redaction
- module duplicate/unknown/missing/orphan behavior
- reduced-surface module selection
- canonical tool definitions and annotations
- skills traversal/sensitive-resource defenses
- upstream result normalization
- optional AgentGateway degradation
- no-replay/delivery_unknown path
- memory child cleanup
- Work Runtime schema/open/health and representative operations for every Work submodule
- task revision + audit stale binding
- state-only resume
- artifact verification semantics
- output bounding/redaction

Create/retain black-box scripts that can compare Python :18787 with Go shadow :18788 without mutating production state except disposable test records that are cleaned up.

Required pre-cutover mechanical PASS:

1. `go test ./...`
2. `go vet ./...`
3. formatted source (`gofmt` clean)
4. release build with fixed version metadata
5. binary file/size/dependency audit
6. `/health` loopback smoke
7. public/loopback tool surface canonical parity with Python
8. public unauthenticated rejection
9. real Skill list/load
10. real AgentGateway list and harmless read-only call
11. real Obsidian/MemOS list/search
12. Work DB compatibility on a copied DB first; production DB only after parity
13. module allowlist/hot-plug tests
14. repeated MCP/Memory/doctor resource convergence; no persistent duplicate child group
15. RSS/peak RSS/startup time/binary size captured
16. official CodexPro self-test still passes independently
17. no secret leakage

## Cutover

Only after all above PASS:

- install the verified Go release artifact to a stable path such as `/home/agent/.local/bin/codexpro-bridge`
- update only the Bridge user service ExecStart to that binary
- preserve EnvironmentFile, listener, module env, tunnel and DB path
- daemon-reload and restart only codexpro-bridge.service
- do not restart AgentGateway, memory services, tunnel, CodexPro or other agents unless independently necessary
- repeat loopback/public surface, doctor and representative calls
- verify process RSS/task convergence
- retain the Python checkpoint and previous service unit for immediate rollback

## Versioning/documentation

Treat this as runtime 3.0.0 because the implementation/runtime/build model changes while the public capability contract stays compatible. Update package/runtime version, plugin manifest/template, README, CHANGELOG, deployment/rollback docs, architecture/testing docs and a `docs/bridge-v3.0-go-release-receipt.md` with exact toolchain/SDK/dependency versions, binary SHA-256/size, resource baseline, parity checks and rollback reference.

Do not delete the Python source/venv until after production Go has passed the final post-cutover audit and a bounded rollback retention decision is recorded. It is acceptable to keep Python as an explicit rollback snapshot while removing it from the active runtime path.
