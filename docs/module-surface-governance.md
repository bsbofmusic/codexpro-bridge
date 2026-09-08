# Module and public tool-surface governance

Status: current operational contract

CodexPro Bridge does not use a fixed global public-tool count as a release invariant. The public MCP surface is derived from the enabled capability-module catalog at process startup.

## Core rule

A module owns four things:

1. a unique `module_id`
2. its own version and health/shutdown hooks
3. the public tool names it declares
4. one installer that registers exactly those tools

The server core owns none of the individual public tool decorators. It creates the backend objects, builds the module catalog, asks `CapabilityRegistry` to mount enabled modules, and then audits the resulting surface.

## Module selection

`CODEXPRO_BRIDGE_MODULES` controls which registered modules are exposed.

- unset, empty, `*`, `all`, or `auto`: enable every registered module
- comma-separated module IDs: enable only that set
- duplicate names: de-duplicated
- an unknown module ID: startup fails closed

Example:

```text
CODEXPRO_BRIDGE_MODULES=shared_skills,bridge_doctor
```

This is deployment hot-plugging: change module selection, restart only `codexpro-bridge.service`, then refresh MCP clients so they obtain the new schema. Do not mutate a running MCP tool surface in place; clients can cache schemas and create ghost-tool behavior.

## No fixed tool total

Release gates must never assert a literal total such as `N tools` as a future invariant.

Use these relationships instead:

```text
actual public tools
      ==
union(tools declared by every enabled module)
```

and:

```text
missing tools = 0
orphan tools  = 0
```

An observed count may be recorded in an immutable historical release receipt, but it is evidence about that release only.

## Duplicate prevention

Startup rejects:

- duplicate `module_id`
- duplicate tool names inside one module
- duplicate tool names across modules
- empty tool declarations

A collision is a deployment error, never something to resolve with aliases.

## Residue prevention

During mount, Registry snapshots the server tool set before and after each module installer.

For each enabled module:

```text
newly registered tools == module-declared tools
```

If the installer registers an undeclared tool, or fails to register a declared tool, startup fails.

After all enabled modules mount:

```text
actual server tools == expected enabled-module tools
```

Any orphan or missing tool fails startup. Disabled-module tools must therefore disappear completely instead of remaining as hidden compatibility aliases.

## Surface audit and fingerprint

Registry exposes a computed audit containing:

- enabled modules
- disabled modules
- enabled module count
- actual tool count
- expected tool count
- missing tools
- orphan tools
- consistency status
- SHA-256 fingerprint of the sorted actual tool-name set

`doctor` and `/health` expose sanitized surface metadata. The fingerprint is a quick drift detector; MCP `tools/list` remains the schema source of truth.

## Adding a module

A new built-in module should require only:

1. its backend/state implementation
2. one module factory returning a `ModuleDescriptor`
3. registration of that factory in the module catalog
4. module-specific tests

Do not edit a global expected-tool list, fixed total, compatibility alias table, or client manifest tool enumeration.

Default `auto` selection picks up the module when it becomes part of the registered catalog. Operators can pin an explicit module allowlist when staged rollout is needed.

## Removing a module

1. remove or disable the module at the catalog/config layer
2. restart Bridge only
3. require surface audit `consistent=true`
4. require removed module tools to be absent from `tools/list`
5. scan executable source/tests/plugin for retired tool names
6. do not retain request-rewrite aliases
7. refresh/re-register MCP clients that cached the old schema

Historical documentation may name retired tools for diagnosis, but executable current sources must not advertise or rewrite them.

## Release gate

A module/surface release passes only when:

- full pytest and compile audit pass
- registry collision tests pass
- module enable/disable hot-plug tests pass
- unknown module selection fails closed
- installer missing/orphan tests pass
- `surface.consistent=true`
- `missing_tools=[]`
- `orphan_tools=[]`
- loopback and public MCP tool sets match the enabled manifest
- public unauthenticated MCP remains rejected
- deep doctor passes for every enabled module
- official CodexPro remains independently healthy
- no retired executable alias or duplicate registration source remains

Tool count is telemetry, not a contract.
