# Operations

Routine checks:

1. `systemctl --user is-active codexpro-bridge.service`
2. `systemctl --user is-active agentgateway.service agentgateway-xydc.service`
3. `GET http://127.0.0.1:18787/health`
4. Run Bridge doctor in deep mode. Enabled modules must report independently; disabled modules must not be probed as if active.
5. Run an authenticated local `tools/list` and compare it with `doctor.surface`: `consistent=true`, `missing_tools=[]`, `orphan_tools=[]`.
6. When `shared_memory` is enabled, run `codexpro_bridge_memory_list` and confirm both memory sources expose their current upstream tools, including mutation tools when provided upstream.
7. When `work_runtime` is enabled, run a lightweight `codexpro_bridge_work` smoke using disposable state or an isolated test DB.
8. Verify the public endpoint rejects unauthenticated MCP access, then run the authenticated public smoke and require the public tool set to match the enabled module manifest.
9. Periodically run the official CodexPro self-test separately; Bridge maintenance must not change CodexPro full-mode settings.
10. For module enable/disable changes, edit `CODEXPRO_BRIDGE_MODULES`, restart only Bridge, refresh the MCP client schema, and require removed-module tools to be absent rather than aliased.

Operational ownership:

- Change shared Skills through Skills Manager, then re-run Bridge Skill smoke. Do not maintain a Bridge copy.
- Change shared general MCP routing through AgentGateway, then re-run Bridge MCP smoke. Do not maintain per-Bridge shared server definitions.
- Obsidian and MemOS memory data remain owned by the memory plane. Bridge direct MCP access does not create a second ingestion or backup path.
- Work Runtime owns only task/project/checkpoint/audit/artifact lifecycle metadata at `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`.
- Workspace files, Bash, Git, edits, tests, deploys, sends, purchases, and other external mutations remain outside Work Runtime. Use the existing owner tool and record only the result/evidence reference in Work Runtime.
- Resume is state-only. Never treat a checkpoint as a replay log and never automatically retry delivery-unknown mutations.
- Restart only Bridge when Bridge code/configuration changes. Restart an upstream only when that upstream itself changes.
- A failed optional domain is reported as degraded and must not be “fixed” by merging it into another fault domain.

SQLite policy for 2.1:

- `PRAGMA foreign_keys = ON` on every connection.
- `PRAGMA user_version` is the migration boundary.
- Default DELETE journal mode is used; WAL is not enabled in 2.1.
- Database path stays outside `/opt/gpt-workspace/codexpro-bridge` so source rollback/cleanup cannot silently delete task state.
