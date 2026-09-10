# Operations

## Routine checks

1. Confirm `codexpro-bridge.service` is active and the process path is `/home/agent/.local/bin/codexpro-bridge`.
2. `GET http://127.0.0.1:18787/health` and require the computed enabled-module surface to be consistent.
3. Run Bridge doctor in deep mode. Every enabled module must report independently; disabled modules must not be probed as active.
4. Run authenticated local `tools/list` and compare it with `doctor.surface`: `consistent=true`, `missing_tools=[]`, `orphan_tools=[]`.
5. When `shared_skills` is enabled, load one real Skill from Skills Manager.
6. When `shared_mcp` is enabled, list the live AgentGateway catalog and make one harmless real upstream call. Upstream tool count is telemetry only.
7. When `shared_memory` is enabled, list every currently registered memory source and make representative harmless calls. Source names come from the runtime source registry, not a hard-coded operational list.
8. When `work_runtime` is enabled, run a lightweight Work read and confirm schema version `2`.
9. When `web_accelerator` is enabled, validate one Resume Capsule and one bounded read-only batch/result-ref path.
10. Verify the public endpoint rejects unauthenticated `/mcp`, then run authenticated public MCP and require the same enabled-module surface.
11. For non-trivial code changes, use workspace-local CodeGraph before and after implementation to inspect call paths, impact, provider boundaries and stale nodes.
12. After MCP transport/session/batch changes, run the resource-convergence gate against every currently configured AgentGateway endpoint. Temporary target processes during live sessions are acceptable; persistent post-settle growth is not.

## Runtime ownership

- Production process: `/home/agent/.local/bin/codexpro-bridge`
- Source: `/opt/gpt-workspace/codexpro-bridge`
- Work DB: `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`
- Dynamic provider config: `/home/agent/.config/codexpro-bridge/providers.env`
- Secret auth env: `/home/agent/.config/codexpro-bridge/env`
- Skills: Skills Manager
- General shared MCP: dedicated AgentGateway `/mcp/<name>` routes discovered dynamically from the loopback admin `config_dump`
- Memory: semantic source registry only; MemOS/Obsidian transport reuses the same dynamically discovered Shared MCP runtime/routes
- Workspace execution: official CodexPro/CyberKate

The systemd unit owns Bridge process settings only. Shared-owner discovery roots such as the Skills root and AgentGateway discovery URL belong in `providers.env`; per-route or per-Memory command inventories must not be copied there.

## Provider changes

- Change provider endpoints/paths in `providers.env`.
- Restart only `codexpro-bridge.service` so the new provider environment is loaded.
- Do not change the ChatGPT connector URL/token merely because an internal provider changed.
- `shared_mcp` dynamically discovers upstream MCP tools from live `tools/list`; never encode an upstream tool total or tool list in Bridge.
- `shared_memory` dynamically enumerates its runtime source registry. A new source should provide a transport and, if it participates in `memory_search`, a deterministic search binding.
- Prefer attaching an MCP-speaking new capability (for example Vision/Search) to AgentGateway. This requires no Bridge code change.
- Only a genuinely new protocol should need a thin `capabilities.Adapter`; that adapter must not own planning, scheduling, AI reasoning, workflow state, a second registry, or a second data plane.

## Go transport policy

Bridge→AgentGateway uses the official Go MCP SDK with short-lived request/response sessions:

- `DisableStandaloneSSE=true`
- `MaxRetries=-1`
- deterministic session Close is mandatory
- delivery-unknown external operations are never automatically replayed
- outbound MCP clients start from a clean value context and inherit only the tighter inbound deadline; inbound MCP transport/session context values must never be forwarded to another MCP peer

Lifecycle tests must exit naturally. An outer hard kill that bypasses client Close may strand AgentGateway stdio targets and is invalid positive convergence evidence.

The Bridge public server is stateless and loopback-only. Go SDK localhost Host-header protection is disabled only because Cloudflare Tunnel reaches this authenticated loopback origin while preserving the public Host header. `/mcp` token authentication remains mandatory.

Bridge process containment is also a release invariant. The Go runtime uses `GOMEMLIMIT=128MiB`; the user service keeps `MemoryHigh=192M`, `MemoryMax=256M`, `MemorySwapMax=64M`, `TasksMax=256`, and `LimitNOFILE=4096`, with a bounded restart rate. These are guardrails, not capacity targets: if observed normal/pressure peaks approach them, investigate the regression before raising limits. Authenticated MCP request bodies are capped by `CODEXPRO_BRIDGE_MAX_REQUEST_BYTES` (4 MiB default) before the MCP SDK handles them.

## Work Runtime / Web Accelerator

Work Runtime uses SQLite schema version `2`:

- `PRAGMA foreign_keys = ON`
- `PRAGMA busy_timeout = 5000`
- `PRAGMA journal_mode = DELETE`
- deterministic `PRAGMA user_version` migration
- schema v2 is additive over v1

Work remains a state layer. Resume is state-only. Operation receipts/idempotency protect mutations from replay. `delivery_unknown` requires state verification before any retry. Conversation affinity stores a one-way fingerprint, not raw conversation text. Result shaping is mechanical, bounded and non-AI. Parallel MCP batch is bounded and allowed only for explicitly read-only upstream tools.

## Change rules

- Shared Skill changes go through Skills Manager.
- Shared MCP/provider routing changes go through AgentGateway/provider config.
- Memory data/ingestion changes stay in the memory plane; Bridge remains a consumer.
- Workspace files/Bash/Git/deployments stay in CodexPro/CyberKate.
- Restart only Bridge for Bridge code/provider config changes. Restart an upstream only when that upstream itself changes.
- Build release binaries with `CGO_ENABLED=0`, `-buildvcs=false`, `-trimpath`, and stripped symbols so production and clean release-clone builds are byte reproducible across different Git histories.
- Keep Python only in explicit rollback archives; do not run a second active Bridge runtime.
