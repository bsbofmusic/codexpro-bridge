# Architecture

CodexPro Bridge is a thin ChatGPT-facing shared-capability adapter. The active 3.1.3 runtime is a statically linked Go binary. ChatGPT Web remains the only reasoning/planning brain; Bridge is deterministic mechanical infrastructure.

```text
ChatGPT Web
  |
  +-> CodexPro Bridge
       |
       +-> Generic Capability Adapter Registry
       |    +-> shared_skills
       |    +-> shared_mcp
       |    +-> shared_memory
       |    +-> work_runtime
       |    +-> web_accelerator
       |    `-> bridge_doctor
       |
       +-> Dynamic provider configuration
       |    `-> /home/agent/.config/codexpro-bridge/providers.env
       |
       +-> Skills Manager
       +-> AgentGateway Core + zero or more optional AgentGateway endpoints
       +-> Memory source registry
       |    +-> Obsidian MCP
       |    +-> MemOS MCP
       |    `-> future registered memory sources
       `-> Work Runtime SQLite

ChatGPT Web
  `-> CodexPro/CyberKate
       `-> workspace files / Bash / Git / edits / deployment
```

## Ownership

Ownership remains singular:

- Skills Manager owns shared Skill membership and Skill content.
- AgentGateway owns the shared general MCP registry/routing plane.
- Memory providers own their own memory data and tool surfaces.
- Official CodexPro/CyberKate owns workspace execution.
- Bridge owns only its lightweight Work Runtime state and the deterministic Web Accelerator metadata stored there.
- Bridge does not own another AI runtime, planner, scheduler, workflow engine, MCP registry, Skill copy, or memory database.

## Dynamic discovery boundary

Dynamic discovery is the default for external capability content:

- `shared_skills` reads the live Skills Manager catalog and loads only the requested Skill/resource.
- `shared_mcp` reads live `tools/list` data from the configured AgentGateway endpoints; upstream tool count and contents are telemetry/data, never compiled Bridge contracts.
- `shared_memory` enumerates its registered memory-source transports at runtime. `list`, `call`, `status`, and `search(auto)` are source-registry driven; source-specific search behavior is supplied by a thin search binding rather than source-name branches in core logic.
- Future systems should use an existing dynamic plane first. For example, a Vision system that exposes MCP should be attached to AgentGateway and becomes available to Bridge without Bridge code changes.
- A non-MCP or otherwise special system may attach through the generic `capabilities.Adapter` boundary. The adapter may translate protocol/arguments/results and expose health/lifecycle only.

Bridge-owned Work operations remain a closed deterministic state-machine API. They are intentionally not dynamically discovered because making them runtime-extensible would turn Work into a workflow engine.

## Generic capability adapters

Bridge server composition understands one small interface:

```text
Adapter
  +-> Descriptor(ID, Kind, Version, Dependencies, Traits)
  +-> Tools
  +-> Operations
  `-> Health
```

`BuildWithAdapters` mounts built-in and extra adapters through the same Registry. Registry performs module collision checks, dependency validation, enabled-module selection, health aggregation, and public-surface auditing. The server no longer contains a per-tool handler switch.

Adding a new provider class must not require edits to unrelated Skills/MCP/Memory/Work runtimes. The release test uses a mock Vision adapter to enforce this boundary.

## Provider configuration

Concrete external-provider locations do not belong in the systemd unit. Production provider configuration lives at:

`/home/agent/.config/codexpro-bridge/providers.env`

The systemd unit only owns Bridge process settings and loads the provider config plus the secret auth EnvironmentFile. Changing provider endpoints/paths requires a Bridge restart so the process receives the new environment, but does not require editing the unit or changing the ChatGPT connector URL/token.

## Public surface

The public Bridge MCP surface stays intentionally small while external systems remain dynamic behind dispatchers. The release invariant is:

```text
actual public tools == union(tools declared by enabled Bridge capability modules)
missing_tools == []
orphan_tools == []
surface.consistent == true
```

Tool counts are telemetry, not contracts. The original Python 2.1.1 public tool definitions remain the compatibility oracle for the 13 protected legacy tools; Bridge 3.1 adds the single deterministic `codexpro_bridge_accelerator` extension tool.
