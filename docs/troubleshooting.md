# Troubleshooting

Diagnose the narrowest failing layer before changing a server.

## 1. ChatGPT surface

CodexPro and CodexPro Bridge run in ordinary **Chat**. They do not need Work.
If ChatGPT displays a Work quota warning or the URL contains `surface=work`,
switch the surface selector to **Chat** before sending the first message.

A Work quota warning means the request has not reached the application. Do not
restart, upgrade, rotate credentials, or alter Cloudflare in response.

## 2. Application selection

Test each application independently in a fresh ordinary Chat conversation:

1. Select only CodexPro and request its safe configuration plus a harmless
   `pwd` command.
2. Select only CodexPro Bridge and request `codexpro_bridge_doctor`, followed by
   one real read-only Skill load.
3. For collaboration, select both applications. Use Bridge for Skill, MCP, and
   Memos work; use CodexPro for files, Bash, Git, and edits.

Seeing cached tool metadata is not proof that the current conversation has an
active application connection. The ChatGPT tool trace is the acceptance gate.

## 3. Public MCP transport

If ordinary Chat sends a tool call but the application reports a connection
failure, verify the public endpoint in this order:

- health endpoint succeeds;
- an unauthenticated MCP request is rejected;
- authenticated `initialize` succeeds;
- `tools/list` returns the expected fixed surface;
- one harmless public tool call succeeds.

Keep credentials and authenticated URLs out of command output, logs, issues,
screenshots, and support messages.

## 4. Tool-specific failures

Separate application health from downstream failures:

- CodexPro file and Bash tools can be healthy while a Codex CLI handoff lacks
  its configured provider environment.
- Bridge can be healthy while one Hermes MCP upstream is unauthorized,
  out of quota, unavailable, or parked.
- A failure in one Bridge module must not disable live Skill loading or the
  separate CodexPro application.

Upgrade only when the failing behavior is reproduced against the currently
published version and the release notes identify a relevant fix.
