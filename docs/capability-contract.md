# Capability Contract

CodexPro Bridge exposes a **module-derived** MCP surface. Tool count is telemetry; the contract is:

```text
public tools = union(tools declared by enabled modules)
```

Current built-in modules are:

- `shared_skills` — routing, Skill listing/loading and resource loading
- `shared_mcp` — AgentGateway list/status/call dispatch
- `shared_memory` — Obsidian/MemOS search/list/call dispatch
- `work_runtime` — durable Bridge-owned lifecycle state
- `web_accelerator` — deterministic resume/receipt/result/batch helpers
- `bridge_doctor` — Bridge/module/surface diagnostics

## Generic Adapter contract

Every capability system mounts through the same provider-neutral `capabilities.Adapter` boundary:

- `Descriptor()` — stable `ID`, `Kind`, `Version`, optional `Dependencies`, descriptive `Traits`
- `Tools()` — explicit public MCP tool definitions
- `Operations()` — exact tool-name → deterministic operation bindings
- `Health()` — bounded provider health metadata

The Registry owns module identity, dependency validation, hot-plug selection, health, public-surface accounting and collision checks. The server mounts enabled adapters generically; it does not maintain a per-system handler switch.

A new provider such as Vision, Search or another Memory/MCP implementation should first use an existing dynamic plane when possible. An MCP-speaking Vision/Search system belongs behind AgentGateway and becomes available through live `tools/list` with zero Bridge code changes. Only a genuinely different protocol should require a thin adapter plus tests/registration. Existing Skills/MCP/Memory/Work runtimes must not be edited merely to attach that provider.

Adapters may perform protocol conversion, argument mapping, result normalization, bounded health checks and lifecycle management. They must not grow an AI planner, autonomous loop, scheduler, workflow DSL, second memory/index, second shared MCP registry, approval database, cache empire or provider-owned orchestration layer.

## Dependency and surface invariants

`CODEXPRO_BRIDGE_MODULES` controls which registered modules are exposed. Unset/empty/`*`/`all`/`auto` enables the registered catalog; an explicit comma-separated list enables only those modules.

Startup fails closed on:

- unknown selected module IDs;
- duplicate module IDs;
- duplicate public tool names;
- missing tool/operation bindings;
- operations with no declared public tool;
- module dependencies that reference an unknown module;
- enabled modules whose declared dependency is disabled;
- final missing/orphan public tool residue.

The final release invariant is `actual tools == union(enabled module tools)`, with `missing_tools=[]`, `orphan_tools=[]` and `surface.consistent=true`.

## Ownership boundaries

`codexpro_bridge_route_and_recall` remains the canonical non-trivial entry when `shared_skills` is enabled. Skills failure may degrade that composite route without hiding invalid caller input. Memory intent remains independent and reports disabled/unavailable state rather than using a hidden path.

`shared_mcp` dynamically consumes AgentGateway Core and zero or more optional isolated AgentGateway endpoints from provider configuration. Bridge does not copy their tool registry or narrow upstream permissions. Delivery-unknown calls are never automatically replayed.

`shared_memory` enumerates a runtime source registry rather than a hard-coded source-name list. Each source contributes a transport and may optionally contribute a deterministic search binding. Production currently registers Obsidian and MemOS through their existing wrappers; future sources can join the registry without source-specific branches in list/call/status/search core logic. Bridge creates no second memory store or ingestion path.

`work_runtime` owns only Bridge lifecycle metadata. Schema version 2 adds receipts, conversation affinity, result references, Resume Capsules and event-linked checkpoint state while retaining task/project/audit/artifact ownership. It still does not execute Bash, Git, file edits, deployments, sends, purchases, browser actions or autonomous jobs.

`web_accelerator` is deterministic. It may bind a conversation fingerprint to an active task, assemble a Resume Capsule, persist operation receipts/no-replay state, shape/store bounded results and perform limited MCP batch fan-out. Parallel mode is limited to explicitly read-only upstream tools. It is not an AI runtime or workflow engine.

## MCP lifecycle invariant

Inbound MCP handler context must not be reused as the value-bearing parent of an outbound MCP client. Outbound calls start from a clean context and preserve only the tighter caller deadline. This prevents inbound transport/session metadata from leaking to another MCP peer.

Short-lived AgentGateway sessions must Close deterministically. Lifecycle tests must not use an outer hard kill as positive convergence evidence because killing the test process can bypass `Close()` and strand AgentGateway stdio targets. Repeated list/call/batch/doctor traffic must converge back to the upstream process/resource baseline after the settle window.

Skill membership comes from Skills Manager. Shared MCP schemas come live from AgentGateway. Memory schemas come live from the currently registered memory providers. Concrete provider locations are runtime configuration, not service-unit or server-core contracts. Bridge owns none of those upstream sources of truth.

See `module-surface-governance.md`, `operations.md` and `testing-and-evals.md` for release gates.
