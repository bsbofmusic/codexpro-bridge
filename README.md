# CodexPro Bridge

CodexPro Bridge is a thin ChatGPT-facing MCP adapter for a shared agent stack. It gives ChatGPT a small, stable public surface while lazily reaching the live Skills Manager library, AgentGateway MCP tools, Obsidian/MemOS memory, and a lightweight Work Runtime only when a task needs them.

Current runtime release: **2.1.0**.

## Design goal

Bridge is intentionally a small high-output command bus, not a second autonomous agent platform.

```text
ChatGPT
  ↓
CodexPro Bridge
  ├─ Skills Manager      → route + load selected Skills on demand
  ├─ AgentGateway        → discover + call shared MCP tools on demand
  ├─ Obsidian / MemOS    → recall shared memory only when continuity matters
  └─ Work Runtime        → durable task/checkpoint/audit/artifact state

Official CodexPro / CyberKate
  └─ files, Bash, Git, tests, deployment and other workspace execution
```

Bridge owns no second Skill library, MCP registry, memory database, scheduler, browser runner, or general execution engine.

## Progressive-disclosure usage

The bundled `codexpro-bridge-router` Skill keeps the default path short:

1. **Ordinary one-shot questions stay ordinary.** No Work task, memory search, or MCP inventory just because Bridge is installed.
2. **Non-trivial tasks route first.** Load one PRIMARY Skill; add at most one SECONDARY only when the task genuinely crosses owners.
3. **Memory is continuity-triggered.** Use it for “continue / previous / remember / compare with earlier work” style tasks, not every new prompt.
4. **Work is lifecycle-triggered.** Use it for explicit TODO execution, durable multi-step tasks, checkpoint/resume, project context, mechanical audit, artifact tracking, and completion state.
5. **MCP is capability-triggered.** Query AgentGateway narrowly and call the exact upstream tool only when the task needs it.
6. **Work records state; executors do the work.** Files, Bash, Git, tests and deployment remain in official CodexPro/CyberKate; external-system actions remain with their upstream MCP. Results and evidence are written back to Work.

This keeps Bridge fast and composable without turning it into a heavyweight orchestration layer.

## 2.1 capabilities

### Shared Skills

- Route non-trivial work against the live Skills Manager catalog.
- Load only the selected `SKILL.md` and referenced resources when needed.
- No copied Skill index inside Bridge.

### Shared MCP

- Discover and invoke AgentGateway Core/XYDC tools through a small dispatcher.
- Dynamic upstream tools remain behind the dispatcher instead of being re-exported as hundreds of ChatGPT actions.
- Bridge does not add a second permission database or silently downgrade upstream permissions.

### Shared memory

- Direct search/list/call access to Obsidian and MemOS MCP surfaces.
- Memory consumption is independent from Skill matching.
- Bridge creates no additional memory ingest, sync, or backup path.

### Work Runtime

One public dispatcher covers:

- task lifecycle + ordered steps;
- immutable checkpoints and state-only resume;
- project context references;
- deterministic PASS / FAIL / UNCERTAIN mechanical audits;
- artifact references and verification metadata.

A PASS audit is bound to the task revision it inspected. Material task/step/artifact changes make that PASS stale, preventing false completion.

### Modular public surface

Public tools are derived from enabled capability modules. The release contract is:

- actual tools equal the union declared by enabled modules;
- `missing_tools=[]`;
- `orphan_tools=[]`;
- `surface.consistent=true`;
- duplicate module IDs/tool names fail closed.

Tool count is telemetry, **not** a permanent compatibility contract.

## Repository and live-runtime roles

This repository is the **versioned source, documentation and release history** for CodexPro Bridge.

The live MCP endpoint remains the authority for the schema that a running client should see. Clients should initialize MCP and use live `tools/list` / Bridge doctor output rather than copying a static tool list from README or an old release.

To prevent the stale-registration problem that affected the historical 0.1.x line:

- real `.app.json` files are ignored and never committed;
- connection IDs and authentication tokens are never committed;
- deployment-specific secrets/config stay outside the repository;
- a client must refresh/reconnect after module-surface changes so cached schemas are replaced.

## Source layout

```text
src/codexpro_bridge/       runtime
plugin/codexpro-bridge/    bundled bootstrap Skill + plugin template
scripts/                   smoke/manifest helpers
tests/                     regression and contract tests
docs/                      architecture, operations and Work Runtime docs
deploy/                    service templates
```

## Development

Python 3.12+ is required.

```bash
python -m venv .venv
. .venv/bin/activate
pip install -e '.[dev]'
pytest -q
```

Before a release, also verify the live deployment separately: loopback/public MCP surfaces must agree, deep doctor must pass, and the client-visible schema must match the enabled-module manifest.

## Documentation

Start with:

- [`docs/bridge-v2-blueprint.md`](docs/bridge-v2-blueprint.md)
- [`docs/bridge-v2.1-work-runtime-blueprint.md`](docs/bridge-v2.1-work-runtime-blueprint.md)
- [`docs/module-surface-governance.md`](docs/module-surface-governance.md)
- [`docs/operations.md`](docs/operations.md)
- [`docs/security-and-secrets.md`](docs/security-and-secrets.md)
- [`CHANGELOG.md`](CHANGELOG.md)

## Historical note

The public 0.1.x line used an obsolete fixed eight-tool registration model. It was deliberately decommissioned on 2026-09-08 to stop stale ChatGPT registrations from being recreated. The current 2.x design replaces that model with module-derived live schema discovery and progressive disclosure.
