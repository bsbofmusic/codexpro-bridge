# CodexPro Bridge

CodexPro Bridge is a small companion MCP server for a deployment that already
has both CodexPro and Hermes. CodexPro keeps ownership of files, Bash, Git,
and VPS changes. This Bridge exposes live Hermes Skills, filtered Hermes Native
MCP tools, conditional Memos recall, and a health check to ChatGPT.

It is intentionally a separate MCP connection. Keeping it separate avoids
turning CodexPro into a permanent fork and lets a Hermes integration failure
stay isolated from VPS operations.

## Capabilities

- Live Skill discovery and real SKILL.md loading through Hermes public Skill APIs.
- Read-only loading of allowed Skill resources below references, templates,
  scripts, and assets.
- Listing and one-at-a-time dispatch of enabled Hermes Native MCP tools after
  Hermes include and exclude filtering.
- Conditional Memos recall for explicit history, preference, or second-brain
  questions.
- A fixed eight-tool MCP surface with redacted, bounded results.
- A two-connection Plugin template that routes work to CodexPro or Bridge.

The Bridge does not provide file editing, shell access, Docker, systemd, or
root operations. Those belong to the separate CodexPro connection.

## ChatGPT surface

Use CodexPro and CodexPro Bridge from ordinary **Chat**, not **Work**. They are
MCP applications and do not need Work quota. If an application detail page's
`Try in chat` action opens `?surface=work`, switch the surface selector to
**Chat before sending**. A Work quota warning is not a CodexPro or Bridge
outage. See [docs/troubleshooting.md](docs/troubleshooting.md).

## Requirements

- Python 3.12 or newer.
- A locally installed Hermes runtime whose Skill and Native MCP APIs are
  compatible with this package.
- A separately configured CodexPro MCP connection for VPS operations.
- An authenticated HTTPS reverse proxy or tunnel if the endpoint will be used
  outside the host. The server itself binds to loopback only.

## Quick Start

Create a virtual environment and install the package:

    python3.12 -m venv .venv
    .venv/bin/python -m pip install --upgrade pip
    .venv/bin/python -m pip install -e '.[dev]'

Copy .env.example to a protected environment file outside the repository, set
a dedicated random value for CODEXPRO_BRIDGE_HTTP_TOKEN, and adapt the Hermes
paths. Load that file in the service manager, then start the server:

    set -a
    . /etc/codexpro-bridge.env
    set +a
    .venv/bin/codexpro-bridge

The default endpoint is http://127.0.0.1:18787/mcp. It accepts either an
Authorization header using the Bearer scheme or the codexpro_token query
parameter. Do not place a real authenticated URL in Git, screenshots, tickets,
or logs.

Run the offline suite before deploying:

    .venv/bin/python -m pytest -q

The example systemd unit in examples/codexpro-bridge.service is a starting
point only. Adapt its user, paths, and read-only access to your own Hermes
installation. Use any reverse proxy or tunnel you already operate; this
repository deliberately does not manage a DNS zone or Cloudflare account.

## ChatGPT Plugin

The bundled Plugin joins two pre-registered MCP connections: one CodexPro
connection and one Bridge connection. Render the local-only app manifest with
opaque connection IDs supplied by ChatGPT, then install the Plugin directory:

    python scripts/render_plugin_app_manifest.py \
      --codexpro-connection-id '<codexpro-connection-id>' \
      --bridge-connection-id '<bridge-connection-id>'

The generated plugin/codexpro-bridge/.app.json is ignored by Git. The routing
Skill tells ChatGPT to use Bridge for Hermes capabilities and CodexPro for VPS
operations. See docs/plugin.md for the exact boundary.

## Important Limits

This project calls Hermes integration APIs that are not a stable public Python
SDK. Test a Hermes upgrade in staging and restart Bridge after MCP connection
configuration changes. Bridge creates its own Hermes MCP connections; it does
not share the Hermes Agent process's already-open sessions.

The generic Hermes MCP call tool can reach tools with side effects. The
deployment's Hermes configuration remains the authority for which upstream
tools are visible and callable. Require explicit confirmation in the calling
product for mutating operations.

## Documentation

- docs/architecture.md - ownership and data flow.
- docs/configuration.md - environment variables and deployment boundary.
- docs/capability-contract.md - fixed public MCP tools.
- docs/plugin.md - two-connection Plugin installation.
- docs/troubleshooting.md - ChatGPT surface, connection, transport, and tool diagnostics.
- docs/development.md - tests and local development.
- docs/security.md - redaction and secret boundaries.
- docs/known-limitations.md - current operational limits.

## License And Affiliation

MIT licensed. This is an independent community project and is not affiliated
with or endorsed by OpenAI, CodexPro, Hermes, Memos, or Cloudflare. It includes
no credentials, private deployment configuration, or third-party runtime code.
