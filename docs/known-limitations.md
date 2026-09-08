# Known Limitations

- AgentGateway, Obsidian, and MemOS own their upstream tool namespaces; tool names and schemas may change when those upstreams change.
- `codexpro_bridge_mcp_call` and `codexpro_bridge_memory_call` are generic full-capability dispatchers. Callers should list tools and inspect unfamiliar schemas before invoking them.
- Direct Obsidian/MemOS memory transports are stdio and currently use short-lived sessions per list/call; this favors failure isolation and simple ownership over persistent-process latency optimization.
- `codexpro_bridge_memory_search` is a convenience route, not a replacement for the complete upstream memory tool surfaces. Use `memory_list`/`memory_call` for explicit non-search operations.
- Binary MCP result blocks are reported as unsupported by the current Bridge result normalizer; text and structured content are preserved.
- Work Runtime stores lifecycle metadata only. It is not a scheduler, reminder service, background worker, browser runner, parallel runner, approval system, or second agent framework.
- Resume restores structured state and references only. It cannot and must not reconstruct or replay unknown external side effects from an interrupted prior call.
- Mechanical Audit is intentionally narrow and deterministic in 2.1: each check compares recorded `actual` against `expected`, while evidence collection remains the responsibility of CodexPro or another existing owner tool.
- Artifact Registry does not read, upload, copy, or serve artifact bytes. Hash/size values remain declared metadata unless linked to an explicit verification evidence reference.
- Work Runtime uses a local SQLite database with DELETE journal mode. WAL is deliberately not enabled in 2.1.
- Bridge is intentionally not a file editor, shell, model proxy, general memory database, or execution plane. Official CodexPro remains the workspace execution plane.
