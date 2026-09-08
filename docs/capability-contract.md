# Capability Contract

The Bridge public MCP surface is **module-derived**, not a fixed global tool count. The authoritative relationship is:

```text
public tools = union(tools declared by enabled modules)
```

Current built-in modules are:

- `shared_skills` — routing, Skill listing/loading, Skill resource loading
- `shared_mcp` — AgentGateway list/status/call dispatch
- `shared_memory` — Obsidian/MemOS search/list/call dispatch
- `work_runtime` — durable task/checkpoint/project/audit/artifact state
- `bridge_doctor` — Bridge/module/surface diagnostics

Each module owns its own public tool names, schemas, annotations, health hook, optional shutdown hook, and installer. The server core does not maintain a global expected-tool list or literal total.

`CODEXPRO_BRIDGE_MODULES` controls which registered modules are exposed. Unset/empty/`*`/`all`/`auto` enables the registered catalog; an explicit comma-separated list enables only those modules. Unknown module IDs fail startup rather than being ignored.

`codexpro_bridge_route_and_recall` is the canonical first tool when `shared_skills` is enabled. Skill routing and memory intent are independent. If `shared_memory` is disabled, route-and-recall does not silently call it and reports `module_disabled` for requested recall.

`codexpro_bridge_route` remains the pure Skill-only route. `codexpro_bridge_mcp_call` and `codexpro_bridge_memory_call` remain dynamic upstream dispatchers when their modules are enabled; Bridge does not narrow upstream permissions. Delivery-unknown calls are never automatically replayed.

`codexpro_bridge_work`, when enabled, mutates only Bridge-owned lifecycle state. It does not run Bash, Git, file edits, deployments, purchases, sends, or other external mutations. Resume is state-only and never replays prior side effects.

Mechanical Audit records definitions, evidence references, and deterministic PASS/FAIL/UNCERTAIN evaluation. A required manual evidence item cannot silently satisfy a mechanical check unless explicitly allowed. A PASS audit is bound to the task revision it audited; later material changes make the old audit stale for completion.

Artifact Registry stores references and verification metadata, not blobs. `verification_status=verified` requires a verification evidence reference.

Skill membership comes from Skills Manager. Shared MCP schemas come live from AgentGateway. Memory schemas come live from Obsidian and MemOS. Bridge owns none of those upstream sources of truth and creates no copied registry or memory store.

At startup, `CapabilityRegistry` rejects duplicate module IDs, duplicate public tool names, undeclared extra tools, missing declared tools, unknown module selectors, and any final orphan/missing tool residue. `doctor` reports the computed module/tool surface audit and fingerprint.

See `module-surface-governance.md` for add/remove/hot-plug rules and mechanical release gates.
