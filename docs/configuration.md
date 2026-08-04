# Configuration

Keep configuration in a protected service-manager environment file. Do not
commit it. The checked-in .env.example shows the complete variable set.

| Variable | Default | Purpose |
| --- | --- | --- |
| CODEXPRO_BRIDGE_HOST | 127.0.0.1 | Loopback listener address; non-loopback values are rejected. |
| CODEXPRO_BRIDGE_PORT | 18787 | Listener port. |
| CODEXPRO_BRIDGE_HTTP_TOKEN | none | Required dedicated endpoint token. |
| CODEXPRO_BRIDGE_HERMES_HOME | /hermes/hermes_data | Hermes data directory. |
| CODEXPRO_BRIDGE_HERMES_WORKDIR | /opt/hermes | Hermes source/runtime directory. |
| CODEXPRO_BRIDGE_HERMES_PYTHON | derived from Hermes workdir | Python used for the Skill worker. |
| CODEXPRO_BRIDGE_MEMOS_COMMAND | user-local memos-api-mcp | Installed Memos child executable. |
| CODEXPRO_BRIDGE_SKILL_TIMEOUT | 30 | Skill worker timeout in seconds. |
| CODEXPRO_BRIDGE_MCP_TIMEOUT | 180 | Upstream MCP call timeout in seconds. |
| CODEXPRO_BRIDGE_MAX_OUTPUT_CHARS | 120000 | Maximum serialized public result size. |

The code accepts CODEXPRO_HTTP_TOKEN as a backward-compatible fallback. New
deployments should set CODEXPRO_BRIDGE_HTTP_TOKEN explicitly and should not
reuse a token that grants a separate high-privilege CodexPro service.

The server has no built-in public listener mode. Terminate HTTPS outside the
process and keep the TCP listener on loopback. Health is available at /health;
the MCP endpoint is /mcp and rejects unauthenticated requests.
