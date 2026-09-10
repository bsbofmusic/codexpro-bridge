# CodexPro Bridge 3.1.6 Release Receipt

Date: 2026-09-10
Runtime: `3.1.6`
Shared Memory module: `1.2.1`
Status: **PASS — production patch deployed and mechanically revalidated**

## Scope

3.1.6 is a narrow reliability patch for the Shared Memory semantic layer after 3.1.5 consolidated MemOS/Obsidian transport onto the dynamically discovered Shared MCP/AgentGateway routes.

The public Bridge contract, module set, Work schema, public endpoint, authentication token, Skills ownership, AgentGateway ownership, Memory data ownership, and dynamic route-discovery model are unchanged.

## Incident

`codexpro_bridge_doctor(deep=true)` and Memory `tools/list` were healthy, but a real `codexpro_bridge_memory_search` failed for both MemOS and Obsidian with `mcp_unavailable`.

Bridge logs identified the actual failure at outbound MCP initialization:

```text
calling initialize ... session not found
```

The semantic Memory wrapper reused the Shared MCP route transport but created its outbound timeout context from the inbound Bridge MCP handler context. MCP transport/session values owned by the inbound peer could therefore leak into the new outbound MCP client.

## Fix

`shared_memory.withTimeout` now:

- starts outbound MCP work from a clean `context.Background()` value context;
- preserves a tighter caller deadline when one exists;
- adds no automatic replay or retry;
- keeps MemOS/Obsidian behind the same dynamically discovered AgentGateway routes;
- adds no direct Memory transport, compatibility alias, second MCP registry, daemon, or copied route list.

A regression test verifies that an inbound context value cannot reach the Memory transport while a tighter caller deadline is retained.

## Production identity

- Go toolchain: `go1.27.1 linux/amd64`
- Production binary: `/home/agent/.local/bin/codexpro-bridge`
- Release build: static, stripped, `CGO_ENABLED=0`, `-buildvcs=false`, `-trimpath`, `-ldflags=-s`
- Installed/release SHA-256: `2da4f68881841efc1a91874567a2045e832a8c6cc98d026bda13e013ae288cfa`
- Listener: `127.0.0.1:18787`
- Work Runtime schema: `2`
- Public tool surface: unchanged from 3.1.5; tool count is telemetry only
- Surface fingerprint: `8354d70afc527c8e0d518045da59eda01182122122bc3295c6c7dba1e7132781`

The release artifact SHA matches the installed binary SHA exactly.

## Validation already passed before/after cutover

- `gofmt`
- `go test ./...`
- `go vet ./...`
- `CGO_ENABLED=0 go test ./...`
- race tests for `shared_memory`, `shared_mcp`, `server`, and `capabilities`
- production `/health` after restart
- `codexpro-bridge.service` active/running with `NRestarts=0`
- Bridge deep doctor with all enabled modules healthy
- real Shared Memory search after cutover: MemOS PASS, Obsidian PASS, `degraded_sources=[]`
- dynamic AgentGateway route discovery remained healthy
- temporary add/remove probe route was discovered/removed automatically by Pi, Codex, and Bridge without consumer code changes
- ToolHive and CodeGraph remained ordinary dynamically discovered Shared MCP capabilities

## Acceptance rule added by this patch

Catalog health is not semantic-call health. For a layered public adapter, acceptance must include one harmless real call through the highest semantic layer that changed.

For Shared Memory this means `doctor/status/tools-list` must be followed by a real `memory_search` or representative `memory_call`. This prevents nested MCP session/context failures from hiding behind a green catalog.

## Rollback

Bounded pre-3.1.6 rollback package:

`/home/agent/.local/share/codexpro-bridge/rollback-3.1.6-20260910T1835/`

It contains the previous production binary and user-systemd unit. Restoring the pair does not require changing Skills Manager, AgentGateway routes, Memory data, Work schema, public URL, or credentials.

Deeper 3.1 disaster recovery remains under the existing bounded 3.1 cutover archive. Do not run multiple Bridge runtimes side by side.

## Reusable operational lesson

When `doctor/status/list` is green but a real public operation is red, compare the catalog path and execution path before blaming the upstream service. In nested MCP adapters, inspect context/session ownership first. Start each outbound peer interaction from a clean value context, preserve only explicit cancellation/deadline semantics, and prove the fix with the real public semantic operation plus resource convergence.
