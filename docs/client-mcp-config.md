# Client MCP Configuration

Both public services use MCP over HTTPS and require Bearer authentication. Never commit or paste the real tokens into shared documentation.

## CodexPro

For a client whose remote-MCP JSON uses `type: "http"`:

```json
{
  "type": "http",
  "url": "https://<CODEXPRO_HOST>/mcp",
  "headers": {
    "Authorization": "Bearer <CODEXPRO_HTTP_TOKEN>"
  }
}
```

## CodexPro Bridge

```json
{
  "type": "http",
  "url": "https://<CODEXPRO_BRIDGE_HOST>/mcp",
  "headers": {
    "Authorization": "Bearer <CODEXPRO_BRIDGE_HTTP_TOKEN>"
  }
}
```

## Transport-name compatibility

The servers speak Streamable HTTP. Some clients label that transport explicitly. If the target UI rejects `"type": "http"` and its documented enum includes `streamable-http`, use the same configuration with only this field changed:

```json
{
  "type": "streamable-http",
  "url": "https://SERVICE_HOST/mcp",
  "headers": {
    "Authorization": "Bearer <SERVICE_TOKEN>"
  }
}
```

The endpoint, header name, and Bearer format do not change. Do not use stdio `command`/`args` for these two public services.

## Verification

A correct fresh client must be able to initialize and list tools. The Bridge tool set is derived from the currently enabled module manifest; clients must compare against the live MCP `tools/list` / `doctor.surface`, not a fixed number. An already-connected ChatGPT Plugin/connector may retain an older schema until that connection is refreshed/reconnected; the live public endpoint remains the source of truth. After a module add/remove, refresh the client and require its visible schema to match the live enabled-module surface exactly.
