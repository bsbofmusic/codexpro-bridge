# Security

CodexPro Bridge is designed to expose shared capabilities without duplicating their secret stores or authorization state.

## Repository rules

Never commit:

- `.app.json` with real ChatGPT/Codex connection IDs;
- MCP bearer tokens, API keys, cookies, OAuth tokens, private keys, or tunnel credentials;
- runtime `.env` files or deployment-specific secret files;
- Work Runtime databases, memory databases, caches, logs, or generated session data.

The checked-in `.app.json.template` contains placeholders only. Real application/connection IDs are rendered locally and `.app.json` is ignored by Git.

## Runtime boundaries

- The Bridge listener should remain local/private behind the intended authenticated ingress.
- Bridge does not create a second RBAC or credential database for AgentGateway, Obsidian, or MemOS.
- Upstream permissions stay upstream-owned.
- Work Runtime stores lifecycle metadata only; it must not become a credential or artifact-byte store.
- Delivery-unknown upstream mutations must never be blindly replayed.

## Schema safety

The live MCP endpoint is the authority for the currently enabled tool surface. Do not hard-code a permanent tool count in clients, docs, or release gates.

After capability-module changes, restart only Bridge as required and refresh/reconnect clients so stale cached schemas are replaced. A repository checkout or historical release must never be treated as proof of the currently live schema.

See [`docs/security-and-secrets.md`](docs/security-and-secrets.md) for the detailed runtime policy.
