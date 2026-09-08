# CodexPro Bridge 2.1 — Lightweight Work Runtime Blueprint

Status: FROZEN FOR IMPLEMENTATION
Baseline: CodexPro Bridge 2.0.0 production, 12 stable public tools
Target: CodexPro Bridge 2.1.0 production with a manifest-derived modular public tool surface

## 1. Release intent

Bridge 2.1 adds a small durable work-state layer while preserving Bridge 2.0 as a thin ChatGPT-facing adapter. It must not become a second agent framework, executor, scheduler, file proxy, MCP registry, Skill registry, memory store, browser runner, or parallel runner.

The only new public MCP tool is:

`codexpro_bridge_work`

The original twelve public tools keep their names, required arguments, ownership, permissions, and behavior.

## 2. Ownership boundaries

Unchanged ownership:

- Shared Skills → Skills Manager
- Shared MCP → AgentGateway Core + XYDC
- Memory consumption → direct Obsidian MCP + MemOS MCP
- Memory ingestion/data plane → AgentsView → Obsidian → Git/MemOS
- Workspace/files/Bash/Git/edits/tests → official CodexPro connection
- Parallel execution → ToolHive, not integrated in 2.1

New ownership:

- Task lifecycle state → Bridge Work Runtime
- Checkpoint state → Bridge Work Runtime
- Project context references → Bridge Work Runtime
- Audit definitions/evidence references/deterministic verdicts → Bridge Work Runtime
- Artifact references/verification metadata → Bridge Work Runtime

## 3. Public contract

Bridge 2.0: 12 stable tools.
Bridge 2.1 introduced `codexpro_bridge_work` additively; the current public surface is derived from enabled module declarations rather than a fixed global total.

`codexpro_bridge_work` input:

```json
{
  "operation": "task.create",
  "arguments": {}
}
```

`operation` is a closed enum. No arbitrary module/action string combination is accepted.

Initial operation set:

- task.create
- task.get
- task.list
- task.update
- task.transition
- task.step_add
- task.step_update
- task.summary
- task.resume
- checkpoint.create
- checkpoint.get
- checkpoint.list
- project.create
- project.get
- project.list
- project.context
- project.refresh
- audit.create
- audit.record
- audit.evaluate
- audit.get
- audit.list
- artifact.register
- artifact.get
- artifact.list
- artifact.finalize
- artifact.archive

CRUD completeness is not a goal; agent-value operations are.

## 4. Task Runtime

Task states:

- pending
- running
- blocked
- audit
- done
- failed
- cancelled

Step states:

- todo
- doing
- blocked
- done
- failed
- skipped

State transitions are explicit and validated. `audit_required=true` forbids `running → done`. Completion requires `audit → done` and the latest applicable audit must PASS.

Every task has an integer `revision`, starting at 1. Material task/step mutations increment the revision. An audit records `audited_task_revision`. A PASS for an older revision is stale and cannot unlock Done.

## 5. Checkpoint / Resume

Checkpoint is an immutable execution-state snapshot, not chat backup and not memory replacement.

Meaningful checkpoint boundaries include step completion/blocking, audit entry/completion, high-risk stage boundaries, explicit pause, and task completion.

Resume means state recovery only:

- identify task
- load latest valid checkpoint
- restore structured task/project state
- return next action
- return context references that should be reloaded

Resume never replays mutation calls, especially create/update/delete/send/deploy/purchase/external mutations. Delivery-unknown side effects are never retried automatically.

Ambiguous resume behavior is fail-closed: multiple candidate tasks return candidates instead of guessing.

## 6. Project Context Pack

Project Context stores references/selectors/hints only:

- workspace_ref
- relevant_skills
- relevant_mcp
- memory_queries
- project_rules
- known_risks
- open/recent task IDs
- latest checkpoint ID
- artifact IDs

It never copies Skill bodies, the MCP registry, Memory contents, workspace files, or Git data. `project.context` is the primary read operation; it does not open a CodexPro workspace itself.

## 7. Mechanical Audit

Bridge Work Runtime does not execute tests or commands. Execution remains with official CodexPro, AgentGateway MCP, Bridge doctor, and existing safe tools.

Work Runtime stores:

- audit definition
- evidence references
- deterministic check evaluation
- audit receipt

Audit statuses are exactly PASS / FAIL / UNCERTAIN.

Evidence provenance is explicit:

- verified_structured
- manual
- external_reference

Manual evidence alone cannot satisfy a required mechanical check unless that check explicitly declares manual evidence acceptable.

Done gate:

- `task.audit_required` false: normal legal transition rules apply.
- `task.audit_required` true: latest audit must PASS and `audited_task_revision == task.revision`.

## 8. Artifact Registry

Registry stores references; it does not move/store blobs.

Artifact status:

- draft
- final
- archived

Verification status:

- declared
- verified
- stale

A provided SHA-256/size is metadata unless linked to verification evidence. Bridge must not claim it personally read a file it cannot access.

## 9. Persistence

Database path:

`/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`

State metadata path may use:

`/home/agent/.local/state/codexpro-bridge/`

Database stays outside the source tree.

SQLite 2.1 policy:

- `PRAGMA foreign_keys = ON` on every connection
- deterministic migration via `PRAGMA user_version`
- short transactions
- `busy_timeout`
- default DELETE journal mode; WAL is deliberately not enabled in 2.1

Business tables:

- projects
- tasks
- task_steps
- checkpoints
- audits
- audit_checks
- artifacts

No Postgres/Redis/Mongo/vector DB.

## 10. Fault isolation

Work Runtime is an independent capability module. Failure to open/migrate/query its database must not prevent the original Bridge Core from starting or serving Skills/MCP/Memory/Doctor.

Expected degraded shape:

- Bridge Core healthy
- Work Runtime degraded

The Work module opens database connections per operation or lazily; it must not make Bridge startup depend on successful database initialization.

Doctor reports sanitized Work Runtime metadata only:

- enabled
- healthy
- db_path
- schema_version
- task_count
- open_task_count
- latest_checkpoint

No secrets, full private task bodies, or artifact contents.

## 11. Compatibility / connector cache

The server live schema and ChatGPT cached schema are separate evidence surfaces.

During implementation, keep the existing hidden legacy request rewrite only as a temporary bridge for already-cached ChatGPT sessions. It is not advertised in `tools/list`.

Removal order:

1. Release candidate exposes a tool set exactly matching the enabled module manifest on loopback and public MCP.
2. Fresh/reconnected ChatGPT connector obtains the same live enabled-module schema.
3. Fresh-session Work Runtime black-box scenarios pass.
4. Only then remove the hidden Hermes request rewrite.
5. Re-run the complete regression suite and live smokes.

No legacy aliases may be advertised to satisfy stale clients.

## 12. Required tests

Task: create/read/list, valid/invalid transitions, step progression, blocked state, revision increments, stale-audit rejection, done gate.

Checkpoint: immutable history, latest checkpoint, resume, ambiguous candidate handling, no mutation replay.

Project: create/context, reference-only semantics, no copied Skill/Memory bodies.

Audit: PASS/FAIL/UNCERTAIN, evidence provenance, deterministic evaluation, revision binding, audit-required done gate.

Artifact: register/list/finalize/archive, task association, declared/verified/stale semantics.

Compatibility: protected pre-existing tool schemas remain unchanged, enabled additive modules are present, and unauthenticated public access is rejected.

Fault isolation: broken/unavailable Work DB degrades Work only; Bridge Core still serves the original 12.

## 13. Release gate

Release is allowed only when all are true:

- Bridge 2.0 baseline PASS
- blueprint frozen
- live endpoint tools exactly match enabled module declarations, with zero missing/orphan tools
- original 12 tool regression PASS
- Work Runtime tests PASS
- task revision/audit binding PASS
- checkpoint/resume no-replay PASS
- project reference-only semantics PASS
- artifact verification semantics PASS
- DB outside source tree
- foreign keys enabled
- deterministic migration PASS
- Work Runtime DB failure isolation PASS
- deep doctor reports Work separately
- complete pytest PASS
- compileall PASS
- loopback smoke PASS
- public authenticated smoke PASS
- public unauthenticated rejection PASS
- Skills load PASS
- AgentGateway list/call PASS
- Obsidian/MemOS consumption PASS
- official CodexPro self-test has zero failures
- no retired agent-runtime dependency
- no secret leakage
- fresh/reconnected ChatGPT sees the current enabled-module surface; retired names are never preserved through hidden rewrites
- residue audit PASS

Until all release gates pass, status is NOT RELEASED.
