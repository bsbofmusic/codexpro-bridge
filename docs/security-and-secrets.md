# Security and Secrets

- Bridge listens on loopback only. Public reachability is provided by the existing authenticated tunnel, not by widening the local listener.
- HTTP access requires a token unless an explicit local test enables anonymous mode.
- The Bridge token is stored in `/home/agent/.config/codexpro-bridge/env` with mode `0600`.
- Shared general MCP traffic goes only to loopback AgentGateway Core/XYDC endpoints.
- Direct memory MCP is local stdio. Obsidian is constrained to `/home/agent/obsidian-vault`; MemOS credentials remain in `/home/agent/.config/agent-stack/memos.env` and are loaded only by the neutral wrapper.
- Bridge deliberately adds no MCP allowlist/RBAC layer. `mcp_call` and `memory_call` preserve whatever read/write/create/delete capability the selected upstream server exposes. The public dispatcher itself is therefore annotated as potentially destructive/open-world.
- Skill reads are restricted to managed Skills and allowed support roots: `references`, `templates`, `scripts`, `assets`, `agents`, `prompts`, and `evals`.
- Absolute Skill-resource paths, traversal, hidden components, obvious secret/credential names, private-key formats, and escaping symlinks are rejected.
- Tool results and errors pass through bounded redaction.
- No API keys, cookies, tunnel credentials, protected environment contents, or resolved secret values are logged or returned.
- Delivery-unknown upstream calls are never replayed automatically.
