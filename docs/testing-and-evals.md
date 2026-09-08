# Testing and Evals

Required gates before/after a production switch:

- Full pytest suite passes.
- Python compile check passes.
- Bridge package metadata matches the source release version.
- Registry tests reject duplicate module IDs, duplicate public tool names, unknown module selectors, undeclared installer extras, and missing declared tools.
- Module allowlist tests prove enable/disable changes alter the public tool surface without residue.
- The default surface and every tested module subset satisfy `actual tools == union(enabled module tools)`.
- `doctor.surface.consistent=true`, `missing_tools=[]`, and `orphan_tools=[]`.
- Skills Manager live catalog is readable and at least one real Skill can be loaded when `shared_skills` is enabled.
- AgentGateway Core/XYDC `initialize` and `tools/list` pass; dynamic dispatcher can make a real upstream call when `shared_mcp` is enabled.
- Direct Obsidian and MemOS MCP `initialize`/`tools/list` pass when `shared_memory` is enabled.
- `route_and_recall` skips direct memory with `reason=module_disabled` when memory is disabled.
- Work Runtime tests cover task state transitions, revision increments, immutable checkpoints, state-only resume, ambiguous resume, deterministic audit evaluation, stale-audit rejection, artifact verification metadata, and compact task summaries.
- Manual evidence cannot silently satisfy a required mechanical check unless the check explicitly allows it.
- A broken/unavailable Work database does not prevent other enabled modules from registering.
- The Work Runtime database uses an external data path, foreign keys, deterministic `PRAGMA user_version` migration, and DELETE journal mode.
- Historical backward-compatibility tests may compare protected old tool schemas, but no future release gate may require a literal global tool total.
- Loopback Bridge health passes and reports the computed module list, tool telemetry, and surface fingerprint.
- Authenticated local and public MCP endpoints expose the same tool set as the enabled module manifest.
- Unauthenticated public MCP access is rejected.
- Bridge service is active after restart and does not enter a restart loop.
- Deep doctor reports each enabled module separately and does not pretend disabled modules are active.
- Official CodexPro is re-audited separately and remains in full mode with its official tool surface.
- Final scans show no duplicate active Bridge source, retired executable alias, copied Skill tree, second shared MCP registry, second memory DB, approval layer, scheduler, browser runner, ToolHive integration, or secret copy.

Mutation probes must use disposable targets and include cleanup assertions. A list-only result is not sufficient evidence of full permission.

## Fresh-connector acceptance

1. Prove the live endpoint tool set equals the enabled module manifest and the surface audit is consistent.
2. Refresh/reconnect the ChatGPT connector and open a fresh session.
3. Prove the ChatGPT-visible schema matches the live enabled-module schema; do not compare against a fixed number.
4. Run scenario tests only for modules that are enabled.
5. For Work Runtime, create disposable state, resume from a fresh session, verify `mutation_replayed=false`, exercise audit gating, and inspect artifact/task summary state.
6. If a module was removed, prove its old tools are absent and return `Unknown tool`; never retain request-rewrite aliases.
7. Re-run the full suite and live smoke after any module catalog or selection change.

Tool count is telemetry. Surface consistency and module ownership are the contract.
