# CodexPro Bridge 2.1 — Implementation TODO

Status: SERVER AND REGISTRATION SOURCES CLEAN; CHATGPT PLATFORM REGISTRATION REFRESH REQUIRED
Blueprint: `docs/bridge-v2.1-work-runtime-blueprint.md`

## Phase 0 — Baseline freeze

- [x] Confirm production package/server version is 2.0.0.
- [x] Confirm live public MCP exposes the canonical 12-tool 2.0 surface.
- [x] Confirm Skills Manager, AgentGateway, Obsidian, MemOS and Bridge doctor are healthy.
- [x] Confirm official CodexPro remains full-mode and separate from Bridge.
- [x] Freeze 2.1 blueprint before code changes.

## Phase 1 — Work Runtime core

- [x] Add isolated `work` capability package.
- [x] Add SQLite repository outside the source tree.
- [x] Enforce `PRAGMA foreign_keys = ON` on every connection.
- [x] Use deterministic `PRAGMA user_version` schema migration.
- [x] Keep DELETE journal mode; do not enable WAL.
- [x] Add projects/tasks/task_steps/checkpoints/audits/audit_checks/artifacts tables.
- [x] Implement task state machine and legal transitions.
- [x] Implement task revision increments for material mutations.
- [x] Implement immutable checkpoint creation/list/get.
- [x] Implement resume with no automatic side-effect replay.
- [x] Implement ambiguous-resume candidate return instead of guessing.
- [x] Implement project context reference pack only.
- [x] Implement deterministic audit evaluation with PASS/FAIL/UNCERTAIN.
- [x] Bind audit PASS to exact task revision.
- [x] Reject stale-audit Done transitions.
- [x] Implement artifact register/finalize/archive with declared/verified/stale verification state.
- [x] Implement compact `task.summary`.

## Phase 2 — Bridge integration

- [x] Register Work Runtime as an independent capability module.
- [x] Add exactly one public tool: `codexpro_bridge_work`.
- [x] Use a closed operation enum.
- [x] Preserve original 12 tool names and schemas.
- [x] Keep Work Runtime initialization lazy/fault-isolated.
- [x] Extend doctor with sanitized Work Runtime metadata.
- [x] Keep current hidden legacy request rewrite temporarily for cached sessions.
- [x] Update server/package version to 2.1.0 only after implementation gates pass.

## Phase 3 — Tests

- [x] Task create/get/list/update/transition tests.
- [x] Valid and invalid transition tests.
- [x] Step progression/blocking tests.
- [x] Task revision tests.
- [x] Checkpoint immutable history/latest/resume tests.
- [x] Resume no-replay test.
- [x] Ambiguous resume test.
- [x] Project reference-only context tests.
- [x] Audit PASS/FAIL/UNCERTAIN tests.
- [x] Manual evidence cannot silently satisfy required mechanical checks.
- [x] Audit revision-binding and stale-audit rejection tests.
- [x] Artifact declared/verified/stale tests.
- [x] Artifact final/archive tests.
- [x] `task.summary` aggregation tests.
- [x] Broken Work DB fault-isolation test.
- [x] Original 12 public tool schema regression tests.
- [x] Historical 2.1 release surface test passed; current/future gates use manifest-derived surface consistency instead of a fixed total.
- [x] Public unauthenticated rejection regression test.
- [x] Full pytest suite PASS: 51 passed.
- [x] compileall PASS.

## Phase 4 — Production switch

- [x] Preserve a bounded 2.0 rollback point for Bridge-owned source/config.
- [x] Initialize/migrate production Work Runtime DB outside source tree.
- [x] Restart only `codexpro-bridge.service` after code/config change.
- [x] Loopback authenticated smoke PASS; tool set matched the then-enabled module manifest.
- [x] Public authenticated smoke PASS; tool set matched the then-enabled module manifest.
- [x] Public unauthenticated rejection PASS: HTTP 401.
- [x] Skill route/load PASS.
- [x] AgentGateway list plus one harmless real call PASS.
- [x] Obsidian/MemOS consumption PASS.
- [x] Deep doctor PASS with independent Work Runtime status.
- [x] Official CodexPro self-test: zero failures and full mode unchanged.
- [x] Fresh public MCP transport Work E2E PASS with test data cleaned to task_count=0.

## Phase 5 — Fresh ChatGPT black-box acceptance

- [ ] Fresh/reconnected ChatGPT connector schema exactly matches the live enabled-module manifest. Current conversation still exposes a stale pre-2.1 registration and must not be judged against a fixed total.
- [ ] Scenario A in fresh ChatGPT: create project/task/steps and checkpoint. Equivalent public fresh-MCP transport scenario already PASS.
- [ ] Scenario B in fresh ChatGPT: resume without old-session context. Equivalent fresh-MCP transport resume already PASS with `mutation_replayed=false` and `resume_semantics=state_only`.
- [ ] Scenario C in fresh ChatGPT: Audit FAIL/UNCERTAIN blocks Done; PASS after valid evidence unlocks Done. Equivalent public E2E already PASS.
- [ ] Scenario D in fresh ChatGPT: artifact summary distinguishes registered/final artifact state. Equivalent public E2E already PASS.
- [x] Remove the hidden stale-session request rewrite after root-cause confirmation and creation of a bounded 2.1 rollback. Current stale eight-tool connector now receives `Unknown tool` for retired names instead of being silently rewritten.
- [x] Re-run complete regression and live smokes after legacy rewrite removal: 48 pytest PASS, compileall PASS, loopback/public manifest-matching smoke PASS, public unauthenticated 401 PASS.

## Phase 6 — Documentation / cleanup / release receipt

- [x] Update CHANGELOG.
- [x] Update README.
- [x] Update architecture/capability-contract/operations/testing/deployment/known-limitations docs.
- [x] Update `codexpro-bridge-operations` Skill through proposal gate; live Skill version 2.4.0.
- [x] Update shared ownership map through proposal gate; `shared-agent-stack-operations` version 1.4.0.
- [x] Keep Skill version separate from Bridge runtime version.
- [x] Retire the obsolete public 0.1.x eight-tool registration source on GitHub main; remove the local `/home/agent/code/codexpro-bridge-public` working copy after preserving a full Git bundle rollback.
- [x] Final executable legacy-alias scan is zero in production `src/`, `tests/`, and `plugin/`, and the old public working copy is absent. Historical/diagnostic documentation may name the retired eight-tool fingerprint but cannot expose or rewrite it.
- [x] Scan changed release artifacts for obvious secret leakage; only runtime variables, test fixtures, and token placeholders found.
- [x] Skills Manager whole-library check PASS with no `last_check_error`.
- [x] Remove temporary `.release` working artifacts; proposal rollbacks and E2E evidence are preserved in managed rollback paths and the release receipt.
- [x] Produce pre-closeout `CodexPro Bridge 2.1 Release Receipt` with the remaining external gate explicit.

## Phase 7 — Modular / hot-pluggable tool-surface governance

- [x] Remove fixed global tool-count assertions from executable tests and current operational contracts.
- [x] Move public tool decorators out of server core into capability-module installers.
- [x] Make Registry mount enabled module descriptors and derive the public surface automatically.
- [x] Add `CODEXPRO_BRIDGE_MODULES` allowlist with `auto/*` default and fail-closed unknown module IDs.
- [x] Add duplicate module/tool collision rejection.
- [x] Add per-installer missing/orphan registration rejection.
- [x] Add final surface audit with enabled/disabled modules, missing/orphan tools, count telemetry, and fingerprint.
- [x] Make `/health`, doctor, and live smoke module-aware.
- [x] Prove a reduced module set starts with only its declared tools and zero residue.
- [x] Complete production restart and loopback/public manifest-matching smoke after this refactor; fingerprints match and missing/orphan sets are empty.
- [x] Update managed operations Skill with the no-fixed-count / hot-plug / no-residue governance pattern; live `codexpro-bridge-operations` is 2.4.0 via proposal gate with independent rollback.
- [x] Re-run whole-library Skill audit and final executable stale-count/stale-alias scan; all `last_check_error=null`, fixed-count executable hits=0, legacy executable aliases=0.

- [x] Phase 7 modular tool-surface governance is mechanically complete.
- [ ] Mark the entire 2.1 TODO complete only when the separate Phase 5 ChatGPT application-registration gate is mechanically proven.
