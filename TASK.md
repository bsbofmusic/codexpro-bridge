# CodexPro Bridge 3.1.3

## Goal

Provide ChatGPT Web with a thin, deterministic, modular bridge to shared Skills, MCP tools, Memory providers, durable Work state, and bounded Web-acceleration helpers without becoming another autonomous agent runtime.

## Hard boundary

- ChatGPT Web is the only reasoning/planning brain.
- Bridge does not own AI planning, autonomous continuation, schedulers, workspace execution, a second Skill library, a second shared MCP registry, or a second memory/index plane.
- Workspace files/Bash/Git/tests/deployment remain the separate official CodexPro/CyberKate execution plane.

## Sources of truth

- Skills: live Skills Manager catalog from provider configuration.
- Shared MCP: AgentGateway primary plus zero or more optional providers from runtime provider configuration; upstream tool contents are discovered dynamically.
- Shared Memory: runtime memory-source registry; provider tool contents are discovered dynamically.
- Work lifecycle state: Bridge Work Runtime SQLite database outside the source tree, schema version 2.
- Provider instances/endpoints/paths: runtime provider configuration, not systemd/service-core hard-coding.

## Public surface

The public MCP surface is derived from enabled capability adapters/modules. Tool count is telemetry, not a compatibility contract.

Current built-in modules are:

1. `shared_skills`
2. `shared_mcp`
3. `shared_memory`
4. `work_runtime`
5. `web_accelerator`
6. `bridge_doctor`

Healthy deployment invariants:

- actual tools equal the union declared by enabled modules;
- `surface.consistent=true`;
- `missing_tools=[]`;
- `orphan_tools=[]`;
- duplicate module/tool registrations and missing dependencies fail closed.

## Extensibility

All capability systems mount through the provider-neutral `capabilities.Adapter` boundary. Prefer an existing dynamic shared protocol plane first; for example, an MCP-speaking Vision/Search provider should attach to AgentGateway and become discoverable without a Bridge code/schema change. Only genuinely different protocols should require a thin adapter.

Adapters may map protocol, arguments/results, health, and lifecycle. They must not grow planning, scheduling, business orchestration, replacement registries, or autonomous loops.

## Work Runtime / Web Accelerator

Work Runtime stores deterministic lifecycle state only. Web Accelerator provides mechanical helpers such as Resume Capsule, conversation affinity, operation receipts/idempotency/no-replay, bounded result shaping/result references, and bounded sequential/parallel read-only MCP fan-out. It does not decide the next plan and never replays an uncertain external mutation.

## Reliability / containment

- MCP sessions are short-lived request/response sessions with deterministic Close and no automatic reconnect/replay.
- Inbound MCP handler context values are not reused as outbound provider-session context.
- Transport/batch changes require resource-convergence testing across configured gateways.
- Release pressure gates include Bridge-only high-concurrency MCP surface testing, bounded upstream-aware failure testing, oversized-request rejection, and live cgroup/systemd guardrail verification.
- Current process containment includes a Go soft memory budget plus systemd memory/swap/task/FD/restart ceilings materially above measured pressure peaks.

## Release line

Runtime 3.1.3 is the current Go small-cannon line: single stripped binary, Work schema 2, Generic Adapter architecture, dynamic provider/source discovery, deterministic Web Accelerator, trigger-aware managed Skill routing, graceful degradation, and anti-crash/anti-OOM containment. See `README.md`, `CHANGELOG.md`, `docs/bridge-v3.1-go-release-receipt.md`, and `docs/bridge-v3.1.3-trigger-routing-release-receipt.md` for maintained evidence.
