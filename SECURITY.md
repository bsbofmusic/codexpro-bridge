# Security policy

## Reporting a vulnerability

Please use GitHub private vulnerability reporting for this repository. Do not
open a public issue containing credentials, authenticated URLs, private MCP
configuration, or details that would expose a live deployment.

## Deployment boundary

CodexPro Bridge is intended to listen on loopback and sit behind an
authenticated HTTPS proxy or tunnel. Use a dedicated random Bridge token and
keep Hermes, CodexPro, Cloudflare, SSH, and other credentials out of this
repository and out of process arguments.

The generic Hermes MCP call tool can invoke upstream tools with side effects.
Its effective authority is exactly the authority granted by the deployer's
Hermes MCP configuration. Review that configuration and require confirmation
for mutating tools in the calling product.
