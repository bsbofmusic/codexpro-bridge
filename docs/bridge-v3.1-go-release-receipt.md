# CodexPro Bridge 3.1.2 Go Release Receipt

Date: 2026-09-09
Runtime: `3.1.2`
Status: **PASS — production cutover completed and mechanically revalidated**

## Scope

Bridge 3.1 keeps ChatGPT Web as the only reasoning/planning brain and upgrades the Go Bridge only as a deterministic mechanical layer. It adds durable resume/receipt/result/batch helpers and a provider-neutral thin Adapter boundary without adding an AI runtime, planner, scheduler, autonomous loop, workflow engine, second memory plane, second shared MCP registry or workspace execution ownership.

## Production identity

- Runtime version: `3.1.2`
- Go toolchain used: Go 1.27.1
- MCP Go SDK: `github.com/modelcontextprotocol/go-sdk` v1.7.0
- Production binary: `/home/agent/.local/bin/codexpro-bridge`
- Release binary SHA-256: `874c727a6ae45596a8e2cfcb1662d342e4792137c59045119874d6c625ee6b10`
- Installed binary SHA-256 matches the clean GitHub release-clone artifact exactly; release builds use `-buildvcs=false` so different Git histories cannot perturb the binary hash
- Binary size: `14,135,456` bytes (~13.5 MiB)
- Production listener: `127.0.0.1:18787`
- Public MCP endpoint: `https://codexpro-bridge.cosymart.top/mcp`
- Public endpoint/token were preserved; this was an in-place backend upgrade, not a connector re-registration or credential rotation

Post-restart systemd evidence:

- `ActiveState=active`
- `SubState=running`
- `NRestarts=0`
- observed production RSS after final 3.1.2 cutover/pressure settle: `20,440 KiB`

## Public surface

Current default module manifest:

1. `shared_skills`
2. `shared_mcp`
3. `shared_memory`
4. `work_runtime`
5. `web_accelerator`
6. `bridge_doctor`

Production surface evidence:

- observed public tool count: `14` (telemetry only)
- original protected Python 2.1.1 tool contracts preserved for all 13 legacy tools
- new extension tool: `codexpro_bridge_accelerator`
- `surface.consistent=true`
- `missing_tools=[]`
- `orphan_tools=[]`
- surface fingerprint: `8354d70afc527c8e0d518045da59eda01182122122bc3295c6c7dba1e7132781`

Authenticated public deep doctor passed through Cloudflare Tunnel. Unauthenticated public `/mcp` returned HTTP `401`.

## Generic Capability Adapter

Bridge core now mounts capability systems through `capabilities.Adapter`:

- `Descriptor()` — ID/kind/version/dependencies/traits
- `Tools()` — explicit public tool definitions
- `Operations()` — exact tool-name → deterministic operation binding
- `Health()` — bounded provider health

Registry now generically validates dependencies and public-surface ownership. The old per-tool server handler switch was removed.

A test-only mock Vision provider was attached through `BuildWithAdapters` and mechanically proved:

- Vision descriptor/tool/operation/health registration works through the generic path
- existing built-in modules remain present
- surface audit remains consistent
- no modification to Skills/MCP/Memory/Work runtime packages is required merely to attach a new provider

Adapters are intentionally thin and may perform protocol conversion, argument/result mapping, health and lifecycle management only. They do not own planning, scheduling, AI reasoning, workflow state or replacement data planes.

## Work Runtime v2 / Web Accelerator

Production Work DB: `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`

Pre-cutover coherent v1 backup:

`/home/agent/.local/share/codexpro-bridge/rollback-3.1-cutover-20260909T094801Z/work-runtime-v1.sqlite3`

Pre-cutover verification:

- `integrity_check=ok`
- `user_version=1`
- task count `12`

Post-cutover verification:

- `integrity_check=ok`
- `user_version=2`
- task count still `12`

Schema v2 is additive and adds operation receipts, conversation affinity, bounded result blobs and event/receipt checkpoint linkage while preserving existing task/project/checkpoint/audit/artifact rows.

Web Accelerator is deterministic and reports `ai_runtime=false`. Production operations include conversation bind/resolve/unbind, Resume Capsule, receipt prepare/finalize/get/list, receipt-aware MCP call/batch and result read/delete.

Safety/behavior gates include:

- mutation operations require stable operation IDs where replay protection is needed
- same operation ID + same arguments deduplicates without replay
- same operation ID + different arguments fails as idempotency conflict
- confirmed mutation success emits `DO_NOT_REPLAY`
- uncertain delivery emits `VERIFY_BEFORE_ANY_RETRY`
- Resume Capsule is state-only and never replays mutations
- conversation affinity stores a normalized one-way fingerprint rather than raw chat text
- result shaping is mechanical only; no AI summary is performed
- parallel batch is bounded and allowed only for explicitly read-only upstream tools
- no loop/condition/DAG/automatic retry/workflow DSL exists

## Graceful degradation

Composite `route_and_recall` now isolates infrastructure degradation such as unavailable Skills/Memory while preserving independent healthy modules. Invalid caller input still fails rather than being hidden as a degraded success.

## MCP lifecycle fix and convergence

A production-blocking nested MCP issue was traced to reusing the inbound MCP handler context as the outbound AgentGateway client context. MCP SDK transport/session values can belong to the inbound peer and must not leak to a different peer.

Bridge 3.1 therefore starts outbound MCP from a clean value context while preserving the tighter inbound deadline.

The reliability test rule was also corrected: an outer hard timeout/kill may bypass deterministic client `Close()` and can strand AgentGateway stdio target groups. Such a test is cleanup evidence only, not proof of a Bridge leak.

Valid probes now use bounded internal contexts and natural process exit/session Close.

Production convergence evidence:

- real `mcp_status` / authenticated surface pressure / doctor traffic was exercised while the shared AgentGateway system was concurrently being modified
- configured optional AgentGateway endpoints remain provider-configured at `19094` and `19098`, but they may be unavailable during independent upstream maintenance; Bridge does not treat their availability or tool totals as public-surface contracts
- aggregate upstream tool count was observed changing from `185` earlier in the release to `123` after the shared-MCP refactor progressed; this change required no Bridge code/schema/config rewrite
- a bounded upstream-aware stress probe encountered provider failures, but Bridge stayed alive with `NRestarts=0`, returned to roughly 23–25 MiB cgroup memory after settle, and the AgentGateway Core direct child-process count returned to zero after natural session Close
- no persistent Bridge-created duplicate target group or zombie residue remained

## Shared systems

Deep doctor after final daemon-reload/restart:

- Skills Manager: healthy, observed live Skill count `139`
- AgentGateway Core: healthy during final doctor; current aggregate live tool count observed by Bridge `123` while optional endpoints were independently under maintenance
- Obsidian MCP: healthy, `10` tools
- MemOS MCP: healthy, `10` tools
- Work Runtime: healthy, schema `2`, `12` tasks
- Web Accelerator: healthy, deterministic, no AI runtime

Bridge 3.1.1 removed concrete provider endpoints/paths from both the deploy template and installed user-systemd unit. Bridge 3.1.2 preserves that model unchanged: the unit loads `/home/agent/.config/codexpro-bridge/providers.env`; provider changes therefore do not require editing the unit, rotating the ChatGPT token, or changing the public MCP URL.

## 3.1.2 anti-crash / resource-containment gate

Bridge 3.1.2 adds a second reliability boundary above normal request/session correctness:

- authenticated MCP request bodies are capped by `CODEXPRO_BRIDGE_MAX_REQUEST_BYTES` (default `4 MiB`, configurable `64 KiB`–`16 MiB`) before downstream MCP handling;
- HTTP server guardrails: `ReadHeaderTimeout=10s`, `IdleTimeout=60s`, `MaxHeaderBytes=64 KiB`; no global write timeout is used because legitimate upstream tool calls may run for minutes;
- Go runtime soft limit: `GOMEMLIMIT=128MiB`;
- systemd failure-domain limits: `MemoryHigh=192M`, `MemoryMax=256M`, `MemorySwapMax=64M`, `TasksMax=256`, `LimitNOFILE=4096`, plus `StartLimitBurst=5` per 60 seconds;
- current production settings were mechanically read back after deployment and matched these values.

Pressure evidence:

- local `/health`: `5,000` requests at concurrency `100`, zero failures, ~`6,138 req/s`, p99 `32ms`, longest `38ms`, no Bridge restart;
- authenticated MCP surface on 3.1.2 shadow: `1,000` sessions at concurrency `64`, zero failures, ~`410 ops/s`, p99 `324ms`, cgroup MemoryPeak ~`11.4 MiB`, no restart;
- authenticated MCP surface on final production 3.1.2: `1,000` sessions at concurrency `64`, zero failures, ~`401 ops/s`, p99 `325ms`, no restart;
- authenticated `5 MiB` MCP request: HTTP `413` before downstream MCP handling;
- final production after pressure: `NRestarts=0`, cgroup `MemoryCurrent` ~`9 MiB`, process RSS ~`20 MiB`; observed cgroup peak remained far below the 192/256 MiB guardrails.

The read-only `cmd/bridge-stress` harness is part of the repository release evidence. It receives the Bridge token only through process environment and never prints it.

## Mechanical quality gates

Passed before production cutover:

- `gofmt`
- `go test ./...`
- `go vet ./...`
- `CGO_ENABLED=0 go test ./...`
- race tests for Accelerator, Work, Shared MCP, Capabilities and Server
- stripped `CGO_ENABLED=0` release build
- protected legacy contract tests
- Work v1→v2 production-copy migration tests
- receipt/idempotency/no-replay tests
- result shaping/ref tests
- bounded parallel batch tests
- graceful degradation tests
- dependency fail-closed tests
- mock Vision Generic Adapter test
- authenticated MCP request-size cap tests and live `5 MiB → 413` black-box check
- `systemd-analyze verify` for the production unit (only an unrelated host unit warning was emitted)
- read-only `bridge-stress` high-concurrency surface tests and bounded shared-MCP failure-pressure test
- live systemd cgroup limit readback and post-pressure `NRestarts=0` verification

## CodeGraph audit

CodeGraph was used both before/during the implementation and again on the cleaned final tree.

Final clean index after release pressure tooling and cleanup:

- indexed files: `42`
- nodes: `623`
- edges: `2,091`

Final structural flow:

`BuildWithAdapters → registerAdapters → Adapter interface → provider adapter`

`BuildWithAdapters` remains confined to the server composition/entrypoint/test blast radius; new provider attachment does not create reverse dependencies into existing Skills/MCP/Memory/Work runtimes.

CodeGraph remains an engineering overlay only and is not a Bridge runtime dependency.

## Rollback

Preferred 3.1 rollback package:

`/home/agent/.local/share/codexpro-bridge/rollback-3.1-cutover-20260909T094801Z/`

Contains:

- prior 3.0.1 binary
- prior service unit
- coherent schema-v1 Work database

Because 3.1 migrates Work Runtime to schema 2, rollback to 3.0.1 must restore the matching v1 DB together with the prior binary. Deeper Go→Python/Python 2.1.1 archives remain historical disaster recovery only.

## ChatGPT Web connector

The MCP URL and HTTP token were intentionally not rotated. Existing ChatGPT registration can remain pointed at the same public endpoint. Because the live public schema changed by adding `codexpro_bridge_accelerator`, a reconnect/refresh is required only so ChatGPT re-runs MCP initialization/`tools/list` and drops its cached 13-tool schema.

No new endpoint, token or duplicate connector registration is required.
