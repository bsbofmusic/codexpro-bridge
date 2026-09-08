# Architecture

CodexPro Bridge is a thin ChatGPT-facing shared-capability adapter. It does not replace the upstream systems and it does not own their data.

```text
ChatGPT Web
  +-> CodexPro Bridge
  |    +-> Skills Manager central library
  |    +-> AgentGateway Core / XYDC
  |    +-> direct Memory
  |    |    +-> Obsidian MCP -> /home/agent/obsidian-vault
  |    |    `-> MemOS MCP -> protected neutral wrapper
  |    `-> Work Runtime -> local SQLite lifecycle state
  |
  `-> CodexPro
       `-> workspace files / Bash / Git / edits
```

Ownership remains singular:

- Skills Manager owns shared Skill membership and content.
- AgentGateway owns the shared general MCP registry/routing plane.
- Obsidian remains the readable/cutover memory source; MemOS remains its derived retrieval service.
- Official CodexPro owns workspace execution.
- Bridge owns only its lightweight Work Runtime lifecycle state; it does not execute the work recorded there.
- Bridge otherwise only discovers, routes, and dispatches upstream capabilities.

The public Bridge surface remains small while upstream capabilities remain dynamic. Skills are loaded progressively, AgentGateway schemas stay behind `mcp_list`/`mcp_call`, complete Obsidian/MemOS schemas stay behind `memory_list`/`memory_call`, and the five Work Runtime modules stay behind one closed-enum `codexpro_bridge_work` dispatcher. `memory_search` is a convenience route only.

Work Runtime state is stored separately from source at `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`. A Work database failure degrades only the Work capability and must not prevent the original Skills/MCP/Memory surfaces from starting.

The Bridge process remains disposable with respect to shared infrastructure: deleting and reinstalling it must not require migrating Skills, AgentGateway state, Obsidian content, MemOS data, or CodexPro workspace state. Work Runtime state is the only Bridge-owned persistent state and must be handled explicitly when preserving or deleting Bridge task history.
