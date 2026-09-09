# CodexPro Bridge 2.1.1 — MCP Session Teardown Reliability Fix

Date: 2026-09-09

## Incident

The VPS experienced repeated non-clean resets while Bridge MCP operations were active. Bridge's `AgentGatewayTransport` described sessions as short-lived but explicitly passed `terminate_on_close=False` to the MCP Python SDK for both tool listing and tool calls. The SDK default is session termination on close. With termination disabled, AgentGateway retained each multiplex MCP session and its stdio target subprocess group.

On this 3.6 GiB VPS, a leaked AgentGateway state after reboot already held two complete target groups and approximately 489 MiB RAM plus 472 MiB swap. A single additional multiplex session temporarily pushed the AgentGateway cgroup peak above 1 GiB. The two most recent crash boots ended during bursts of Bridge MCP initialize/fan-out activity. Kernel-level OOM evidence was not available to the unprivileged executor, so the proven mechanism is retained subprocess/resource accumulation and resulting memory/swap/IO pressure; this receipt does not claim a specific kernel OOM kill without kernel logs.

## Fix

- `AgentGatewayTransport.list_tools()` now terminates the Streamable HTTP MCP session on context exit.
- `AgentGatewayTransport.call_tool()` now terminates the Streamable HTTP MCP session on context exit.
- Bridge smoke scripts use the same teardown behavior so validation cannot leak sessions.
- Regression test requires both list and call paths to request session termination.

AgentGateway currently acknowledges DELETE teardown with HTTP 202. MCP Python SDK 2.0.0 logs this as a termination warning because it treats 200/204 as clean termination responses, but black-box verification showed the target subprocess group is actually removed. Do not disable teardown merely to hide this warning.

## Rollback

Pre-fix bounded evidence is stored under:

`/home/agent/.config/agent-stack/backups/codexpro-bridge-session-teardown-20260909/`

Rollback is permitted only for a verified regression and must not restore a leaking production state without an alternative session-lifecycle fix.

## Release gate

- targeted MCP runtime regression tests pass;
- full Bridge test suite and compile audit pass;
- package/source/plugin runtime versions agree at 2.1.1;
- AgentGateway is restarted once to clear previously leaked sessions;
- Bridge is restarted to load the fixed runtime;
- repeated live MCP list/call/doctor operations return AgentGateway tasks to baseline with no duplicate persistent stdio target groups;
- Bridge surface remains module-derived and consistent with zero missing/orphan tools.
