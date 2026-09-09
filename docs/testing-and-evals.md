# Testing and Evals

Required Go 3.1 gates before and after a production switch:

- `gofmt` leaves all Go source formatted.
- `go test ./...` passes.
- `go vet ./...` passes.
- `CGO_ENABLED=0 go test ./...` passes.
- Race tests pass for changed concurrent packages, including Accelerator/Work/MCP/Registry/server when those paths change.
- `CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags=-s` produces the release binary. `-buildvcs=false` is required so a clean GitHub release clone and the live production workspace build byte-identical artifacts despite different Git histories.
- The protected Python 2.1.1 fixture remains the compatibility oracle for the original 13 public tools. New extension tools are tested separately; the historical fixture is not rewritten to hide drift.
- Registry tests reject duplicate module IDs, duplicate public tool names, unknown selectors, missing dependencies, unknown dependencies, missing operation bindings and orphan operation/tool residue.
- Module allowlist tests prove enable/disable changes alter the public surface without aliases or hidden residue.
- `doctor.surface.consistent=true`, `missing_tools=[]`, `orphan_tools=[]`.
- Generic Adapter tests prove built-in modules mount through one path and a test-only mock Vision provider can attach through `BuildWithAdapters` without modifying Skills/MCP/Memory/Work runtimes or adding a server handler switch.
- Adapter descriptors expose provider-neutral ID/kind/version/dependencies/traits; traits are metadata, not a workflow or permission language.
- Skills Manager live catalog is readable and at least one real Skill can be loaded when `shared_skills` is enabled.
- AgentGateway Core and every configured optional endpoint initialize/list pass; the dynamic dispatcher can make a harmless upstream call when `shared_mcp` is enabled.
- Every currently registered memory source initializes/lists when `shared_memory` is enabled. Tests include a third mock source proving list/status/search dispatch is registry-driven with no source-name branch. Reversible mutation probes, when required, must clean up disposable targets.
- `route_and_recall` isolates `skills_unavailable`/memory degradation without hiding invalid caller input.
- Work Runtime tests cover v1→v2 additive migration and preservation of existing rows; schema 2 task state/revision semantics; immutable/event-linked checkpoints; state-only resume and Resume Capsule; conversation affinity fingerprinting; receipts/idempotency/no-replay; deterministic audit evaluation/stale-audit rejection; artifact verification; bounded/redacted result refs.
- A broken/unavailable Work DB does not prevent unrelated enabled modules from registering.
- Work Runtime uses the external data path, foreign keys, deterministic `PRAGMA user_version=2` and DELETE journal mode.
- Web Accelerator tests cover mutation `operation_id` requirements, idempotency conflict, finalized receipt immutability, `DO_NOT_REPLAY`, `VERIFY_BEFORE_ANY_RETRY`, result shaping/ref behavior, sequential batch and read-only-only parallel batch.
- Parallel batch limits are bounded and no loop/condition/DAG/automatic retry/workflow DSL exists.
- Nested MCP tests preserve the invariant that outbound MCP starts from a clean value context while respecting the tighter inbound deadline.
- The installed systemd unit contains no concrete Skills/MCP/Memory provider endpoint/path; it loads `/home/agent/.config/codexpro-bridge/providers.env`, and doctor configuration reflects that provider file after restart.
- Loopback `/health` passes and reports the computed module list, tool telemetry and surface fingerprint.
- Authenticated loopback and public MCP expose the same enabled module surface. Unauthenticated public MCP is rejected.
- Cloudflare Tunnel public-host behavior is exercised because localhost Host-header protection is disabled only behind Bridge's loopback listener plus mandatory token auth.
- Deep doctor reports each enabled module once and avoids redundant duplicate upstream probes.
- Repeated real `mcp_list` / `mcp_status` / harmless `mcp_call` / accelerator parallel batch / doctor traffic must return Core and every configured optional AgentGateway to baseline after the settle window. Temporary target groups are expected only while a session is alive.
- Run the read-only `bridge-stress` probe for authenticated MCP surface pressure. A release pressure gate must include at least one high-concurrency Bridge-only run and one bounded upstream-aware run; the latter may observe provider errors during upstream maintenance, but Bridge must not restart, trend upward in memory/tasks after settle, or strand new AgentGateway child-process groups.
- Verify the Bridge-local failure-domain controls are active after deployment: `GOMEMLIMIT`, `MemoryHigh`, `MemoryMax`, `MemorySwapMax`, `TasksMax`, `LimitNOFILE`, and restart-rate limiting. Chosen limits must remain materially above measured normal/pressure peaks rather than becoming accidental throttles.
- Send an authenticated oversized MCP request above `CODEXPRO_BRIDGE_MAX_REQUEST_BYTES` and require HTTP `413` before the downstream MCP handler. Normal authenticated MCP surface load must remain unaffected.
- Lifecycle probes must use bounded contexts and deterministic session Close. An outer hard process timeout/kill is not valid positive convergence evidence because it can strand AgentGateway stdio targets.
- CodeGraph is a required second-view audit for non-trivial engineering changes: rebuild the current index, inspect relevant call paths/blast radius and require thin package/ownership boundaries. `.codegraph/` is an overlay, not a production dependency.
- Official CodexPro is re-audited separately and remains in full mode with its official ownership/tool surface.
- Final scans show no active Python runtime, second Bridge listener, duplicate active source tree, copied Skill tree, second shared MCP registry, second memory DB, approval layer, scheduler, browser runner, ToolHive execution layer, temporary shadow probes, shadow DB/log, or secret copy.

## Schema migration acceptance

For v1→v2:

1. Create a coherent pre-migration SQLite `.backup`.
2. Require `integrity_check=ok`, `user_version=1` and record the pre-migration task count.
3. Run v2 against a copy first; verify additive migration and preserved task/project/checkpoint/audit/artifact state.
4. After production migration require `user_version=2` and preserved pre-existing task count.
5. Keep the v1 coherent backup with the 3.0.1 binary rollback artifact because 3.0.1 must not be assumed compatible with a schema-v2 DB.

## Fresh-connector acceptance

1. Prove the live endpoint tool set equals the enabled Adapter/module manifest and surface audit is consistent.
2. Refresh/reconnect the ChatGPT connector after a schema/module change. An in-place Bridge binary upgrade keeps the same MCP URL and token unless credentials were deliberately rotated.
3. Prove ChatGPT-visible schema matches the live enabled-module schema; never compare against a permanently fixed tool total.
4. Run scenarios only for enabled modules.
5. For Web Accelerator, exercise Resume Capsule, affinity, no-replay receipt state, result ref and a harmless read-only parallel batch.
6. If a module is removed, prove its old tools are absent and return `Unknown tool`; never retain request-rewrite aliases.
7. Re-run Go suite, public smoke, deep doctor and resource-convergence after any module catalog, adapter, transport, batch or session-lifecycle change.

Tool count is telemetry. Surface consistency, ownership boundaries, deterministic no-replay semantics, thin Adapter attachment and convergent lifecycle behavior are the release contract.
