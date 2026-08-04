# Plugin Integration

The Plugin is a convenience layer for a ChatGPT environment that supports
pre-registered MCP applications. It needs two distinct technical connection
IDs: one for CodexPro and one for CodexPro Bridge. These are identifiers, not
endpoint URLs or tokens.

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
