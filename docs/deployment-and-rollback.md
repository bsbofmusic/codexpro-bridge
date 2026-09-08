# Deployment and Rollback

Deployment is staged and Bridge-only:

1. Freeze a bounded backup of Bridge-owned source/config before the production switch.
2. Build/test `/opt/gpt-workspace/codexpro-bridge` in isolation. Unit tests, compile audit, original-tool schema comparison, direct AgentGateway probes, direct Obsidian/MemOS probes, and Work Runtime tests must pass before service restart.
3. Keep Work Runtime state outside the source tree at `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`. Database creation/migration is lazy and must not become a Bridge startup dependency.
4. Refresh the Bridge editable package metadata without changing its pinned MCP dependency set.
5. Restart only `codexpro-bridge.service` after preflight passes. The existing tunnel endpoint and authentication token remain unchanged.
6. Verify local authenticated MCP, public unauthenticated rejection, public authenticated MCP, `doctor.surface.consistent=true`, zero missing/orphan tools, and enabled-module smokes.
7. Re-run the official CodexPro self-test and confirm full mode stayed unchanged.
8. Refresh/reconnect the ChatGPT connector and prove the fresh session schema matches the enabled module manifest. Never restore a compatibility alias to satisfy a stale registration.
9. For module hot-plug changes, use `CODEXPRO_BRIDGE_MODULES`; unknown selectors must fail closed and removed-module tools must disappear after the Bridge-only restart.

The bounded Bridge 2.0 rollback point created before the 2.1 production edits is:

`/opt/gpt-workspace/codexpro-bridge-rollback-2.0-20260908T112339Z`

The older 2.0 cutover backup remains documented at:

`/home/agent/.config/agent-stack/backups/codexpro-bridge-v2-20260908T1800`

2.1 rollback restores only Bridge-owned source/config to the bounded 2.0 point and restarts only Bridge. The Work Runtime database is not deleted automatically; Bridge 2.0 ignores it, so retaining it preserves a possible forward-recovery path. If task history must be discarded, that is a separate explicit data-retention action.

Skills Manager, AgentGateway, Obsidian content, MemOS data, official CodexPro, Pi, Paseo, and ToolHive are not rolled back as part of a Bridge rollback because Bridge does not own them.

CodexPro lifecycle remains separate. Do not pin, patch, fork, downgrade, or restart official CodexPro merely to change Bridge.
