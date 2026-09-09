# Deployment and Rollback

CodexPro Bridge 3.1.2 is a Go single-binary service. Deployment is Bridge-only: shared Skills, AgentGateway, Memory providers, official CodexPro, Pi, Paseo and ToolHive are not restarted or migrated merely because Bridge changes.

## Production deployment

1. Build and test `/opt/gpt-workspace/codexpro-bridge` in isolation.
2. Require `gofmt`, `go test ./...`, `go vet ./...`, `CGO_ENABLED=0 go test ./...`, relevant race tests, protected-contract regression, CodeGraph structural review, real Skills/MCP/Memory/Work smokes and resource convergence.
3. Build the release artifact with `CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags=-s`; disabling VCS embedding keeps the release artifact reproducible across the live workspace and clean GitHub release clone.
4. Preserve a bounded rollback package before production mutation.
5. Install the verified binary at `/home/agent/.local/bin/codexpro-bridge`.
6. Install the generic systemd unit. The unit owns Bridge process settings only and loads:
   - `/home/agent/.config/codexpro-bridge/providers.env` for non-secret provider locations/endpoints;
   - `/home/agent/.config/codexpro-bridge/env` for authentication/secrets.
7. Restart only `codexpro-bridge.service`.
8. Verify loopback `/health`, authenticated loopback MCP, authenticated public MCP, unauthenticated rejection, deep doctor, real upstream calls, Work schema, Web Accelerator behavior and resource convergence.
9. Refresh/reconnect ChatGPT only when the live public MCP schema/module selection changed or the client has stale cached registration state.

Production identity:

- unit: `/home/agent/.config/systemd/user/codexpro-bridge.service`
- binary: `/home/agent/.local/bin/codexpro-bridge`
- listener: `127.0.0.1:18787`
- public endpoint: `https://codexpro-bridge.cosymart.top/mcp`
- provider config: `/home/agent/.config/codexpro-bridge/providers.env`
- secret env: `/home/agent/.config/codexpro-bridge/env`
- Work DB: `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`

Provider changes do not require a new ChatGPT endpoint or token. Change provider config, restart Bridge, and mechanically verify the newly discovered provider state.

## Provider configuration

Do not encode concrete Skills/MCP/Memory provider paths or optional AgentGateway endpoints in the systemd unit. They belong in `providers.env`.

`shared_mcp` dynamically lists tools from the configured primary and optional AgentGateway endpoints. Tool count/content remains runtime telemetry.

`shared_memory` enumerates its runtime source registry. Production currently registers Obsidian and MemOS; source-specific search behavior is carried by a small search binding. Adding a source must not require a new source-name branch in core list/call/status/search logic.

Prefer adding MCP-speaking new capabilities such as Vision/Search to AgentGateway so Bridge discovers them with zero code changes. A special non-MCP protocol may use a thin Generic Adapter.

## 3.1.x rollback

Preferred rollback package for the 3.1 cutover:

`/home/agent/.local/share/codexpro-bridge/rollback-3.1-cutover-20260909T094801Z/`

It contains:

- prior 3.0.1 binary;
- prior service unit;
- coherent schema-v1 Work database.

Because Bridge 3.1 migrated the production Work DB to schema version 2, rollback to 3.0.1 must restore the matching schema-v1 DB together with the 3.0.1 binary. Do not assume 3.0.1 can safely operate a schema-v2 database.

Rollback procedure:

1. Stop `codexpro-bridge.service`.
2. Preserve the current 3.1.x DB/binary as failure evidence.
3. Restore the 3.0.1 binary and saved pre-3.1 service unit.
4. Restore the coherent v1 Work DB from the same rollback package.
5. Reload/restart only `codexpro-bridge.service`.
6. Verify loopback/public MCP, auth, deep doctor and Work schema `1`.

The older Python 2.1.1 archives and Go→Python cutover backups remain deeper disaster-recovery history only. Do not run Python beside Go as a standby service.

Skills Manager, AgentGateway, Memory provider data, official CodexPro, Pi, Paseo and ToolHive are outside Bridge rollback ownership.
