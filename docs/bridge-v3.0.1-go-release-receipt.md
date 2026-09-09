# CodexPro Bridge 3.0.1 Go Release Receipt

Date: 2026-09-09

## Verdict

PASS — the active production Bridge runtime is Go 3.0.1. Version 3.0.0 established the Go runtime baseline; 3.0.1 removes the retired nested Skills metadata dependency and consumes the canonical Skills Manager CLI contract. The Python 2.1.1 public MCP contract and Work schema remain preserved, public/loopback/full-permission smokes pass, AgentGateway resource usage converges after repeated real calls, and the active source tree no longer contains a Python runtime.

## Runtime and build

- Runtime version: `3.0.1`
- Go toolchain: `go1.27.1 linux/amd64`
- MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.7.0`
- SQLite: `modernc.org/sqlite v1.58.0`
- Build mode: `CGO_ENABLED=0`, `-trimpath`, stripped symbols
- Production artifact: `/home/agent/.local/bin/codexpro-bridge`
- Binary SHA-256: `b85757e3fc138f1e28d60128794405393c80a3f8acbbac5716ebe539b658cf23`
- Binary size: `13,947,040` bytes (`13.30 MiB`)
- File audit: statically linked ELF x86-64; `ldd` reports no dynamic executable dependencies
- Production listener: `127.0.0.1:18787`
- Public endpoint: `https://codexpro-bridge.cosymart.top/mcp`

Observed Go process memory after cutover was approximately 18–24 MiB RSS depending on workload. The prior Python production process was observed around 84 MiB RSS before cutover. Go VSZ/VIRT is intentionally not used as the RAM signal because the runtime reserves a large virtual address range.

## Public contract parity

The Python 2.1.1 production `tools/list` response was captured before cutover and embedded as the Go compatibility oracle at `contract/python-2.1.1-tools.json`.

Go 3.0 preserves all current public tool names, descriptions, annotations, input/output schemas, the five capability modules, module hot-plug semantics, fail-closed unknown module selection, and the zero missing/orphan tool invariant.

Current default release telemetry:

- public tools: `13`
- modules: `5`
- surface fingerprint: `8d357d81b7782f600db07715eedc4e6252e140ee25fe7f0374d830eacd55cda4`

Tool count remains telemetry, not a permanent API contract.

## Architecture preserved

Five modules remain explicit and independently owned: `shared_skills`, `shared_mcp`, `shared_memory`, `work_runtime`, and `bridge_doctor`.

Ownership boundaries remain unchanged:

- Skills Manager owns shared Skill membership/content.
- AgentGateway owns shared general MCP routing; Core and XYDC remain separate fault domains.
- Obsidian/MemOS remain the memory plane; Bridge is only a direct consumer.
- Work Runtime is the only Bridge-owned persistent state.
- CodexPro/CyberKate owns workspace files, Bash, Git, edits, tests, and deployments.
- Bridge does not add a scheduler, browser runner, ToolHive orchestration layer, copied MCP registry, copied Skill index, or second memory DB.

## MCP transport lifecycle

Bridge→AgentGateway uses official Go MCP Streamable HTTP clients with `DisableStandaloneSSE=true`, `MaxRetries=-1`, deterministic session Close, and no replay after delivery becomes uncertain.

The public Go MCP server is stateless and propagates request cancellation. `DisableLocalhostProtection=true` is intentionally set because Cloudflare Tunnel reaches the loopback origin while preserving the public Host header. This is acceptable only while the listener remains loopback-only, `/mcp` token authentication remains mandatory, and public ingress remains the existing Cloudflare Tunnel.

Unauthenticated MCP access was verified rejected after cutover.

## Work Runtime compatibility

Production Work DB remains `/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3`.

No database migration was required. Go 3.0 preserves schema version `1`, foreign keys, busy timeout 5000 ms, DELETE journal mode, task revision binding, current-revision audit gating, immutable checkpoints, state-only resume, no replay of external mutations, and artifact lifecycle/verification metadata.

Skills membership is read from the canonical Skills Manager CLI (`skills-manager-cli skills list --json`) with `SKILL.md` as content authority. The Go runtime no longer depends on a nested `skills/.skills-manager/` metadata shadow; a regression test proves Skills discovery works when that shadow directory does not exist.

The migration corrected one documentation/counting mistake found during testing: the production Work dispatcher contains 27 operations, not 28. The operation set is contract-tested instead of relying on a prose count.

## Live functional evidence

Pre-cutover shadow comparison on `127.0.0.1:18788` verified Python=Go for the public contract, Skills list and Skill content hash, routing results, both AgentGateway domains, Memory surfaces, and Work reads against a copied production DB.

Post-cutover production evidence verified:

- loopback `/health` PASS
- authenticated loopback MCP PASS
- authenticated public MCP through Cloudflare PASS
- public unauthenticated rejection PASS
- deep doctor `ok=true`
- AgentGateway catalog healthy with 168 observed tools at release time
- Obsidian and MemOS both healthy with 10 observed tools each
- Work schema version 1 healthy
- reversible Obsidian write/read/update/delete PASS with no residue
- reversible MemOS create/remove PASS
- memory-intent recall PASS
- simple non-memory prompts do not trigger unnecessary recall

The 168/10/10 counts are release observations only; upstream catalogs remain dynamic.

## Resource convergence evidence

### Shadow stress

Three repeated rounds of real `mcp_list → mcp_status → harmless mcp_call → doctor` activity followed by a 30-second settle window passed.

Core: 11 tasks / 1 process before and after; memory changed from 185,139,200 B to 176,615,424 B. XYDC: 13 tasks / 1 process before and after; memory changed from 15,134,720 B to 14,024,704 B. No persistent process/task growth remained.

Temporary Core peaks were traced to expected session-owned PCA/Reddit/SearXNG/Web Search/Shopify stdio targets and disappeared after session cleanup.

### Production stress

Two repeated production rounds followed by a 30-second settle window also passed.

Core: 11 tasks / 1 process before and after; memory delta +90,112 B; swap delta 0. XYDC: 13 tasks / 1 process before and after; memory delta +196,608 B; swap delta 0.

Resource convergence is PASS. A cgroup limit was not used as proof.

## CodeGraph structural audit

CodeGraph `1.5.0` was used as a workspace-local structural audit overlay after the Python runtime was removed from the staged source tree.

Final index: 28 files, including 27 Go files + 1 YAML file, 418 nodes, 1,364 edges, zero parse/read errors, and zero stale Python nodes.

Findings:

- `Build` remains a composition root: config → registry → five modules → tool handlers.
- `Build` blast radius contains only the server entry path plus its three server tests; it does not cross into ownership of AgentGateway, Memory storage, or Work data.
- Work `Dispatch` remains a thin closed-operation dispatcher over task/project/checkpoint/audit/artifact modules.
- MCP/Memory share only a narrow transport interface; Memory does not become part of AgentGateway ownership.
- No new scheduler/orchestrator/browser/execution layer was introduced.

CodeGraph's test-affect mapping did not identify a test set for the wholesale staged Go replacement, so it was not treated as test-coverage proof. `go test ./...` remains the mechanical test authority.

## Quality gates

PASS gates executed during migration included `gofmt`, `go test ./...`, `go vet ./...`, `CGO_ENABLED=0 go test ./...`, stripped static release build, public contract comparison, module selection/collision regression tests, Work behavior/state tests, shadow black-box parity, real AgentGateway calls, reversible full-permission Memory smoke, loopback/public authentication checks, deep doctor, shadow and production resource convergence, and the CodeGraph structural audit.

## Cleanup and rollback

The active source tree no longer contains the Python runtime, Python tests, venv, egg metadata, or migration-period Python smoke scripts. The temporary `18788` shadow listener is stopped. `.codegraph/` and `bin/` are build/audit artifacts ignored by Git.

Python remains available only for explicit rollback/history:

- service cutover backup: `/home/agent/.local/share/codexpro-bridge/rollback-go-cutover-20260909T063554Z/`
- Python 2.1.1 runtime archive: `/home/agent/.local/share/codexpro-bridge/python-2.1.1-runtime-rollback-20260909T065543Z.tar.gz`
- rollback archive SHA-256: `d57164287e7c549d9355d67645afde7bb869182c3577faba4c8a063780b7bad7`

The Work DB is intentionally not part of the runtime archive because it is live persistent state shared by both compatible runtime versions.

## Release conclusion

CodexPro Bridge 3.0 satisfies the intended small-cannon profile: one static binary, low resident memory, no production language runtime, explicit capability modules, thin ownership boundaries, dynamic upstream discovery, deterministic cleanup, and verified rollback.
