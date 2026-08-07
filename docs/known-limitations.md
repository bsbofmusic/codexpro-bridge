# Known Limitations

- ChatGPT application detail pages may open `Try in chat` on the Work surface.
  The Bridge cannot change that product-side navigation. Switch to ordinary
  Chat before sending; Work quota is unrelated to MCP availability.
- Hermes integration depends on implementation APIs, not a stable public Python
  SDK. Review compatibility information and run a staging smoke after a Hermes
  upgrade.
- Bridge opens its own Hermes MCP connections. It does not share the Hermes
  Agent process's live sessions.
- Restart Bridge after changes to an existing Hermes MCP server's connection or
  access policy. Hot configuration replacement is not a supported contract.
- The route tool returns a live catalog; it does not run a model or promise a
  single automatic Skill selection.
- Memos is semantic retrieval, not the authoritative store for source material.
- Bridge has no file, Bash, Git, Docker, systemd, apt, or root tools.
- An upstream MCP call can be non-idempotent. Bridge never retries an uncertain
  delivery.
