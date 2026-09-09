# Known Limitations

- AgentGateway, Obsidian, and MemOS own their upstream tool namespaces; tool names and schemas may change when those upstreams change.
- `codexpro_bridge_mcp_call` and `codexpro_bridge_memory_call` are generic full-capability dispatchers. Callers should list tools and inspect unfamiliar schemas before invoking them.
- Bridge→AgentGateway uses short-lived request/response sessions. AgentGateway may temporarily spawn its configured stdio targets during a call, so individual list/status/doctor operations can take seconds even though all target groups converge after the session closes.
- Direct Obsidian/MemOS memory transports are stdio and use short-lived sessions per list/call; this favors failure isolation and simple ownership over persistent-process latency optimization.
- `codexpro_bridge_memory_search` is a convenience route, not a replacement for the complete upstream memory tool surfaces. Use `memory_list`/`memory_call` for explicit non-search operations.
- Binary MCP result blocks are reported as unsupported by the current Bridge result normalizer; text and structured content are preserved.
- Go reserves a comparatively large virtual address space. Treat RSS/cgroup physical memory as the resource signal; a large VSZ/VIRT value alone is not evidence of large RAM consumption.
- The production public server intentionally disables the Go MCP SDK localhost Host-header protection because Cloudflare Tunnel connects to the loopback origin while retaining a public Host header. Safety depends on the listener remaining loopback-only and Bridge `/mcp` token authentication remaining mandatory.
- Work Runtime stores lifecycle metadata only. It is not a scheduler, reminder service, background worker, browser runner, parallel runner, approval system, or second agent framework.
- Resume restores structured state and references only. It cannot and must not reconstruct or replay unknown external side effects from an interrupted prior call.
- Mechanical Audit is intentionally narrow and deterministic: each check compares recorded `actual` against `expected`, while evidence collection remains the responsibility of CodexPro or another existing owner tool.
- Artifact Registry does not read, upload, copy, or serve artifact bytes. Hash/size values remain declared metadata unless linked to an explicit verification evidence reference.
- Work Runtime is schema version 2 with local SQLite DELETE journal mode; WAL is deliberately not enabled. Schema v1→v2 is additive, but rollback to a v1 runtime requires the matching coherent v1 database backup.
- Web Accelerator parallel batch is intentionally limited: only explicitly read-only upstream tools may run in parallel, concurrency/batch sizes are bounded, and there is no loop/condition/DAG/automatic retry/workflow language.
- Conversation affinity is a soft deterministic mapping based on a normalized one-way fingerprint; it is not a ChatGPT session manager and cannot start a new assistant turn.
- Generic Adapters reduce attachment cost but do not make arbitrary providers magically compatible; each provider still needs an explicit thin protocol/operation mapping and tests. Adapters are not allowed to become orchestration middleware.
- Bridge is intentionally not a file editor, shell, model proxy, general memory database, AI agent, or execution plane. Official CodexPro remains the workspace execution plane.
