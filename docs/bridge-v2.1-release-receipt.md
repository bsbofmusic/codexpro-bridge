# CodexPro Bridge 2.1 Release Receipt

Status: PRE-CLOSEOUT — SERVER AND REGISTRATION SOURCES CLEAN; CHATGPT PLATFORM REGISTRATION REFRESH REQUIRED
Date: 2026-09-08

## Baseline

Bridge 2.0 verification: PASS

Accepted 2.0 baseline:

- production package/server version 2.0.0
- live public MCP 12-tool surface
- Skills Manager healthy
- AgentGateway healthy
- Obsidian/MemOS healthy
- official CodexPro full mode, 29/29 registered tools, zero failed self-test checks

Bounded 2.0 rollback point:

`/opt/gpt-workspace/codexpro-bridge-rollback-2.0-20260908T112339Z`

## Implementation

Bridge runtime 2.1.0 adds exactly one public dispatcher:

`codexpro_bridge_work`

It carries five lightweight state modules:

- Task Runtime
- Checkpoint / Resume
- Project Context Pack
- Mechanical Audit
- Artifact Registry

Work Runtime remains a state layer. It does not own or execute files, Bash, Git, deployments, sends, purchases, browser actions, schedulers, reminders, ToolHive parallel runs, or agent orchestration.

Key implemented controls:

- explicit task/step state machines
- task revision increments on material task/step/artifact mutations
- audit PASS bound to exact audited task revision
- stale audit blocks Done
- audit status limited to PASS / FAIL / UNCERTAIN
- manual evidence cannot silently satisfy mechanical checks
- immutable checkpoint history
- state-only resume with no mutation replay
- ambiguous resume returns candidates instead of guessing
- project context stores references/selectors only
- artifact verification uses declared / verified / stale metadata
- Work Runtime DB failure is isolated from the original Bridge Core

## Public Contract

Bridge 2.0: 12 public tools

Bridge 2.1: 13 public tools

New tool:

`codexpro_bridge_work`

Mechanical rollback-vs-candidate schema comparison:

- old_count: 12
- new_count: 13
- shared_count: 12
- changed_original_schemas: []
- new_only: [`codexpro_bridge_work`]

Result: PASS

## Persistence

Database:

`/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`

Schema version: 1

Tables:

- projects
- tasks
- task_steps
- checkpoints
- audits
- audit_checks
- artifacts

SQLite controls:

- `PRAGMA foreign_keys = ON`
- deterministic `PRAGMA user_version` migration
- DELETE journal mode
- WAL not enabled in 2.1
- database outside source tree

Current post-E2E production state:

- task_count: 0
- open_task_count: 0
- latest_checkpoint: null

Result: PASS; no Work E2E residue remains in the production database.

## Version Consistency

- package metadata: 2.1.0
- MCPServer version attribute: 2.1.0
- plugin manifest: 2.1.0
- public tool count: 13

Result: PASS

## Mechanical Audit

Offline suite:

- pytest: 48 passed after removal of the three legacy-compatibility tests
- compileall: PASS

Coverage includes:

- task lifecycle and invalid transitions
- step progression/blocking
- revision increments
- immutable checkpoint history
- state-only resume
- ambiguous resume
- project reference-only context
- audit PASS/FAIL/UNCERTAIN
- manual evidence restriction
- audit revision binding / stale-audit rejection
- artifact verification and lifecycle
- task.summary aggregation
- Work DB fault isolation
- historical release surface matched the then-enabled module manifest (observed count: 13)
- original 12 schema compatibility
- public unauthenticated rejection contract

Result: PASS

## Production Smoke

Service after 2.1 switch:

- active/running
- MainPID changed from the 2.0 process to the 2.1 process
- NRestarts: 0 after switch

Loopback authenticated MCP:

- tool_count: 13
- route: PASS
- AgentGateway status: PASS
- Memory list: PASS
- Work task.list: PASS

Public authenticated MCP:

- tool_count: 13
- route: PASS
- AgentGateway status: PASS
- Memory list: PASS
- Work task.list: PASS

Public unauthenticated MCP:

- HTTP 401

Deep doctor:

- shared_skills: healthy
- shared_mcp: healthy
- shared_memory: healthy
- work_runtime: healthy
- bridge_doctor: healthy

AgentGateway:

- live list: PASS
- one harmless real EPO CQL help call: PASS

Official CodexPro regression:

- registered tools: 29/29
- tool_mode: full
- write_mode: workspace
- failed self-test checks: 0

Result: PASS

## Public Work Runtime E2E

A fresh MCP transport was opened against the public production endpoint and exercised the Work dispatcher through real MCP calls.

Scenarios:

- project/task/four-step creation: PASS
- first two steps completed and checkpoint saved: PASS
- new MCP transport state-only resume: PASS
- `mutation_replayed=false`: PASS
- manual evidence leaves audit UNCERTAIN: PASS
- audit gate blocks Done: PASS
- verified structured evidence produces PASS: PASS
- artifact mutation makes prior audit stale: PASS
- stale audit blocks Done: PASS
- artifact finalization: PASS
- refreshed final audit unlocks Done: PASS
- task.summary contains final artifact: PASS
- disposable E2E data cleanup: PASS

Result: PASS

This proves fresh transport/state semantics. It does not substitute for the separate ChatGPT connector schema-registration gate below.

## Skills Governance

`codexpro-bridge-operations`:

- proposal gate: PASS
- apply: PASS
- live Skill version: 2.4.0
- rollback: `/home/agent/.skills-manager/rollback/20260908T130202Z-codexpro-bridge-operations`
- support-file deletions: 0

`shared-agent-stack-operations`:

- proposal gate: PASS
- apply: PASS
- live Skill version: 1.4.0
- rollback: `/home/agent/.skills-manager/rollback/20260908T114754Z-shared-agent-stack-operations`

Whole-library Skills Manager check:

- command exit: 0
- target Skills `last_check_error`: null
- returned live library entries have no reported `last_check_error`

Result: PASS

## Modular Tool-Surface Governance Closeout

The public tool surface is now module-derived rather than count-locked.

Mechanical evidence:

- server core owns no fixed global tool list or literal total;
- public decorators live in capability-module installers;
- `CODEXPRO_BRIDGE_MODULES` supports deployment-level module enable/disable with `auto/*/all` default;
- unknown module selectors fail closed;
- duplicate module IDs and duplicate tool names are rejected;
- installers that add undeclared tools or omit declared tools are rejected;
- final surface audit exposes enabled/disabled modules, count telemetry, missing/orphan sets, and a SHA-256 fingerprint;
- reduced-surface test proved disabled module tools disappear with zero residue;
- default pre-refactor vs post-refactor public tool metadata comparison returned `name_delta=[]` and `schema_changed=[]`;
- current executable fixed-count scan in `src/`, `tests/`, and `scripts/` returned zero hits;
- current operational docs contain no fixed global tool-total release gate;
- pytest: 53 passed;
- compileall: PASS;
- loopback and public surface smoke: `consistent=true`, `missing_tools=[]`, `orphan_tools=[]`, identical fingerprint;
- public unauthenticated MCP: HTTP 401;
- deep doctor: PASS for every enabled module;
- official CodexPro self-test: 29/29 registered tools, full mode, zero failures;
- live `codexpro-bridge-operations` Skill: 2.4.0 via proposal gate and independent rollback;
- Skills Manager whole-library check: PASS with no reported `last_check_error`.

Operational invariant: **tool count is telemetry, not a contract**. Releases are gated by module ownership and surface consistency, not by a literal number.

Result: PASS — PHASE 7 CLOSED

## Security / Residue

Observed secret scan results in project/release artifacts:

- no `ghp_` hits
- no `sk-` hits
- Bearer occurrences are runtime variable interpolation, redaction tests, or explicit `<TOKEN>` placeholders
- no real token value was printed or copied into release documentation

Executable legacy-alias scan is zero in production `src/`, `tests/`, and `plugin/`. Historical/diagnostic documentation may still name the retired eight-tool fingerprint, but no current executable path advertises or rewrites it.

The obsolete public 0.1.x repository was retired on GitHub `main` by PR #2. Its installable MCP package, Plugin/App manifests, fixed eight-tool contract, tests, renderer, and final `.env.example` deployment template were removed from the active branch while historical source remains available through Git history/tags. The final remote root contains only `.gitignore`, `CHANGELOG.md`, `LICENSE`, `README.md`, and `SECURITY.md`. A full pre-decommission Git bundle is stored under `/opt/gpt-workspace/rollback/`.

The local `/home/agent/code/codexpro-bridge-public` working copy was then deleted to remove the second physical source of truth.

The production stale-session request rewrite and its compatibility test were also removed after preserving a bounded 2.1 rollback. The stale current ChatGPT connector now receives `Unknown tool` for retired `codexpro_bridge_hermes_mcp_*` names instead of being silently translated.

The live public `tools/list` matches the enabled module manifest; the current observed count is telemetry, not a release invariant.

## ChatGPT Connector Gate

SERVER LIVE SCHEMA: PASS — enabled-module manifest consistent

CURRENT CHATGPT CONNECTION SCHEMA: FAIL — stale pre-2.1 schema

Direct current-connector discovery for `work` does not expose `codexpro_bridge_work`, even though the live public MCP endpoint returns it.

Classification:

`STALE CHATGPT MCP APPLICATION REGISTRATION`

Root-cause fingerprint: the current ChatGPT connector's eight tool names and descriptions match the retired public 0.1.1 application surface exactly. No old Bridge process, tunnel, service, active public-repo working copy, legacy server alias, or old current-source Plugin manifest remains on the VPS.

Completed remediation:

1. retire the old public 0.1.x install/registration source on GitHub main
2. preserve Git history/tags plus a full rollback bundle
3. delete the local duplicate public-repo working copy
4. remove the production legacy request rewrite and compatibility test
5. restart only Bridge
6. rerun pytest + compileall + loopback/public manifest-matching smoke + 401
7. prove retired old-name calls now return `Unknown tool`
8. rescan current sources and require zero `codexpro_bridge_hermes_mcp_*` aliases

Remaining platform-only closeout gate:

1. remove/refresh the stale CodexPro Bridge MCP application registration in ChatGPT
2. reconnect it against the live production endpoint
3. establish a fresh ChatGPT session
4. confirm the ChatGPT-visible schema exactly matches the live enabled-module manifest and exposes `codexpro_bridge_work` when `work_runtime` is enabled
5. run fresh-ChatGPT Work scenarios A-D
6. mark TODO complete and change this receipt status to RELEASED

Until that platform registration is refreshed, the correct declaration is:

`SERVER/VPS CLEAN — CHATGPT PLATFORM REGISTRATION STILL STALE`
