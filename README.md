# CodexPro Bridge

CodexPro Bridge is the thin ChatGPT-facing command bus for the VPS shared agent stack. The active 3.1.6 runtime is Go and deploys as one stripped binary; shared data and execution remain owned by their existing systems.

## Runtime

- Production runtime: Go 1.27.1
- MCP SDK: `github.com/modelcontextprotocol/go-sdk` v1.7.0
- SQLite: pure-Go `modernc.org/sqlite`, `CGO_ENABLED=0`
- Production binary: `/home/agent/.local/bin/codexpro-bridge`
- Listener: `127.0.0.1:18787`
- Public MCP: `https://codexpro-bridge.cosymart.top/mcp`
- Work DB: `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`
- Dynamic provider config: `/home/agent/.config/codexpro-bridge/providers.env`

The Go 3.0 migration preserved the Python 2.1.1 public MCP contract. Bridge 3.1 keeps those original 13 tool contracts unchanged, adds one deterministic Web Accelerator dispatcher, and upgrades Work Runtime from schema 1 to schema 2 through an additive migration. Python is no longer on the active service path and is retained only in bounded rollback snapshots.

## Responsibilities

- Read enabled Skills live from Skills Manager at `/home/agent/.skills-manager/skills`.
- Dynamically discover dedicated AgentGateway `/mcp/<name>` routes from the loopback admin `config_dump`, then list/call the live upstream tools without copying a route registry or tool inventory.
- Expose semantic shared-memory operations for Obsidian/MemOS by reusing those same dynamically discovered AgentGateway routes; Memory data/ingestion remains outside Bridge and there is no parallel direct Memory transport.
- Keep ChatGPT-facing routing lightweight: `route_and_recall` first for non-trivial work, honor managed Skill `triggers:` as private routing metadata, then load only the selected Skill/tool schema/context on demand.
- Maintain lightweight durable Work state for tasks, checkpoints, project context references, mechanical audits, operation receipts, result references, and Resume Capsules without becoming an execution plane.
- Provide bounded deterministic Web acceleration: conversation/task affinity, no-replay receipts, result shaping and limited read-only parallel MCP fan-out.
- Mount built-in and future capability systems through a provider-neutral thin Adapter contract instead of adding system-specific server switches. Adapters may map protocols, normalize results, expose health, and manage lifecycle; they do not own planning, scheduling, AI reasoning, workflow state, or a second data plane.
- Keep file, Bash, Git, repository inspection, edits, deployments, browser execution, and agent orchestration in their existing owner systems.

## Capability modules

The public MCP surface is derived from enabled capability modules; there is no fixed global tool-count contract. Built-in modules are:

1. `shared_skills`
2. `shared_mcp`
3. `shared_memory`
4. `work_runtime`
5. `web_accelerator`
6. `bridge_doctor`

Every module is mounted through the same provider-neutral Adapter contract (`Descriptor`, tool definitions, explicit operations, health). Descriptor metadata includes module kind, dependencies and descriptive traits. Future systems such as Vision should attach through a thin adapter rather than by modifying the Bridge server core or existing runtime packages.

`CODEXPRO_BRIDGE_MODULES` can enable a selected comma-separated module set. Unset, empty, `*`, `all`, or `auto` enables the registered catalog. Unknown module IDs and missing declared dependencies fail closed. After a module change, restart only Bridge and refresh MCP clients so their cached schema matches the live manifest.

The release invariant is `actual tools == union(enabled module tools)` with zero missing/orphan tools. Tool count is telemetry. See [docs/module-surface-governance.md](docs/module-surface-governance.md).

## Ownership

Bridge owns no shared Skill, MCP, Memory, or workspace data. Skills Manager owns Skill membership/content. AgentGateway owns shared general MCP routing. Obsidian is the readable/cutover memory source and MemOS is its derived retrieval service. Official CodexPro owns workspace execution. Bridge owns only its lightweight Work Runtime state.

## Reliability model

Bridge→AgentGateway uses short-lived request/response MCP sessions. The Go client disables standalone SSE and reconnects, closes each session deterministically, and never replays a call after delivery becomes uncertain. Outbound MCP calls start from a clean context and inherit only the tighter caller deadline, preventing inbound transport/session metadata from leaking into another MCP peer. Resource convergence after repeated list/call/batch/doctor traffic is a release gate.

The public Bridge server is stateless. It binds loopback only and sits behind the authenticated Cloudflare Tunnel. Because the tunnel connects from loopback while preserving the public Host header, Go SDK localhost Host-header protection is explicitly disabled; Bridge's own `/mcp` token authentication remains mandatory.

Bridge 3.1.6 preserves the 3.1.2 process-survival contract and adds one acceptance rule for layered Memory calls: catalog/list health must be followed by a harmless real semantic call so nested context/session failures cannot hide behind a green doctor. Authenticated MCP request bodies remain bounded (4 MiB default), HTTP header/idle time is bounded, the Go runtime has a soft 128 MiB memory budget, and the systemd cgroup has separate memory/swap/task/file-descriptor ceilings well above measured production pressure peaks. `cmd/bridge-stress` provides a read-only authenticated MCP pressure probe; shared-MCP failures may degrade requests but must not restart Bridge or leave persistent AgentGateway child-process growth.

## Documentation

- [Go 3.1.6 Memory context patch receipt](docs/bridge-v3.1.6-release-receipt.md)
- [Go 3.1 release receipt](docs/bridge-v3.1-go-release-receipt.md)
- [Go 3.1.3 trigger-routing patch receipt](docs/bridge-v3.1.3-trigger-routing-release-receipt.md)
- [Go 3.0 migration plan](docs/bridge-v3-go-migration-plan.md)
- [Go 3.0.1 release receipt](docs/bridge-v3.0.1-go-release-receipt.md)
- [Architecture](docs/architecture.md)
- [Operations](docs/operations.md)
- [Deployment and rollback](docs/deployment-and-rollback.md)
- [Testing and evals](docs/testing-and-evals.md)
- Historical 2.x contracts and receipts remain under `docs/` for rollback archaeology.
