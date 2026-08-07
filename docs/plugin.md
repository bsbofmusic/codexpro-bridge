# Plugin Integration

The Plugin is a convenience layer for a ChatGPT environment that supports
pre-registered MCP applications. It needs two distinct technical connection
IDs: one for CodexPro and one for CodexPro Bridge. These are identifiers, not
endpoint URLs or tokens.

## Use ordinary Chat, not Work

CodexPro and CodexPro Bridge are MCP applications. They do not require the
ChatGPT Work surface and must not consume Work quota. Start or continue the
conversation in ordinary **Chat**.

Some ChatGPT application detail pages currently open the `Try in chat` action
at `?surface=work`. Treat that as a product-navigation default, not as a Bridge
requirement. Before sending the first message, switch the surface selector to
**Chat**. If Work quota is exhausted, do not diagnose or restart either MCP
server until the same application has been tested from ordinary Chat.

Render the ignored local manifest:

    python scripts/render_plugin_app_manifest.py \
      --codexpro-connection-id '<codexpro-connection-id>' \
      --bridge-connection-id '<bridge-connection-id>'

Install plugin/codexpro-bridge through the relevant Plugin workflow. Its
router Skill is implicitly available for non-trivial work and directs ChatGPT
to load a live Hermes Skill before acting. It directs editing, Bash, Git, and
other VPS actions to CodexPro rather than to Bridge.

Changing the Bridge tool schema, tool metadata, or public descriptions requires
a refresh of the ChatGPT MCP application. Changing a Hermes Skill body does not
because the Bridge reads it live.

When a tool appears in metadata but cannot be invoked, follow
[troubleshooting.md](troubleshooting.md) before upgrading or restarting a
server.
