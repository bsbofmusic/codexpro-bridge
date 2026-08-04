# Security

Bridge binds only to loopback and requires an endpoint token unless explicitly
placed in local anonymous test mode. Use a dedicated random token in a
protected environment file. Never put a token, authenticated URL, private key,
environment-file body, or resolved Hermes MCP configuration in source control
or logs.

Responses are recursively redacted for credential-like keys and content. The
server also bounds serialized output so an unusually large upstream response
does not become an uncontrolled model input.

Bridge reads Hermes Skills through Hermes APIs and confines resource reads to
references, templates, scripts, and assets. It rejects traversal and common
secret-bearing paths. Hermes remains the owner of MCP configuration, OAuth,
and secret loading. In its in-memory MCP overlay, Bridge sets
sampling.enabled=false and elicitation.enabled=false and excludes Hindsight.

The Memos child command is configurable. Point it at an already installed
binary when an npx-at-runtime path is unsuitable. Bridge preserves the outer
Hermes wrapper so existing secret loading and recall defaults remain Hermes
owned.
