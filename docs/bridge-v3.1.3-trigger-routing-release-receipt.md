# CodexPro Bridge 3.1.3 Trigger-Routing Patch Receipt

Date: 2026-09-09
Runtime: `3.1.3`
Status: **PASS — production patch deployed and mechanically verified**

## Scope

Bridge 3.1.3 is a narrow `shared_skills` routing patch. It makes the existing managed Skill frontmatter field `triggers:` usable as private routing metadata so exact trigger phrases can match inside natural-language text, including continuous Chinese text.

This patch does **not** add a second router, copied Skill index, provider binding, AI logic, MCP tool, capability module, Work schema change, public endpoint change, or credential change.

## Root cause

The previous Go router tokenized continuous Han text into long runs. A sentence such as `30多岁转行的人生经验` could therefore fail to produce `人生经验` as an independent token. The managed Skill already carried an explicit `triggers:` list, but Bridge catalog records did not retain that field for routing.

The observable failure was that a natural request containing `人生经验` could rank unrelated community Skills ahead of the intended `life-experience` Skill even though an explicit shorter request could find it.

## Fix

The patch is confined to the existing `shared_skills` runtime:

- read managed Skill frontmatter `triggers:` into private internal record metadata;
- when a full trigger phrase occurs inside the user task, apply one bounded trigger-match score bonus;
- strip private trigger metadata from public route/list results;
- keep existing name/description/tag scoring and dynamic Skills Manager discovery intact.

No provider-specific or `life-experience`-specific branch was added to Bridge code.

## Changed runtime files

- `skills/runtime.go`
- `skills/runtime_test.go`
- `core/version.go`
- `CHANGELOG.md`

Current documentation was also updated to identify runtime 3.1.3 and the patch rollback path.

## Verification

Source/build gates passed before production closeout:

- `gofmt`
- `go test ./skills`
- `go test -race ./skills`
- `go test ./...`
- `go vet ./...`
- `CGO_ENABLED=0 go test ./...`
- `systemd-analyze verify deploy/codexpro-bridge.service` (PASS; only an unrelated host `tat_agent.service` legacy `/var/run` warning)
- stripped static build with `CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags=-s`
- CodeGraph rebuilt for the final source tree: `42` files, `624` nodes, `2,098` edges; `impact Route` stayed within shared-Skill composition/routing/tests, then `.codegraph` was removed with `codegraph uninit --force`

The new regression test uses a natural Chinese request containing `人生经验` and requires `life-experience` to rank first while `_triggers` remains absent from the public result.

## Production artifact

Production binary:

`/home/agent/.local/bin/codexpro-bridge`

Release artifact used for cutover:

`/tmp/codexpro-bridge-3.1.3`

SHA-256 for both installed and release artifacts:

`0f5ad91032e38f9c1a6a6dac98a07ceb78c9a3a9ba036b82a7183254f88186b3`

The pre-patch 3.1.2 production binary matched the historical 3.1.2 receipt SHA before replacement.

## Live service evidence

After Bridge-only restart:

- `ActiveState=active`
- `SubState=running`
- `NRestarts=0`
- public unauthenticated `/mcp` returns HTTP `401`

Deep doctor passed:

- Skills Manager healthy with observed `141` enabled Skills;
- AgentGateway healthy with observed `123` upstream tools, which remains runtime telemetry;
- MemOS healthy with `10` tools;
- Obsidian healthy with `10` tools;
- Work Runtime healthy at schema `2`;
- Web Accelerator healthy and deterministic;
- all six Bridge capability modules healthy.

Public Bridge surface remained unchanged:

- `surface.consistent=true`
- `missing_tools=[]`
- `orphan_tools=[]`
- observed tool count `14` (telemetry only)
- surface fingerprint `8354d70afc527c8e0d518045da59eda01182122122bc3295c6c7dba1e7132781`

The fingerprint is identical to the pre-patch 3.1.2 surface because this patch changes routing internals only.

## Black-box route acceptance

Natural request:

`帮我找一些30多岁转行的人生经验，尤其是后来后悔没后悔、几年后过得怎么样，优先真实过来人经历。`

Result: `life-experience` ranked first.

Second request:

`我最近有点迷茫，想参考一些人生导师和过来人的真实经历，不要只讲大道理。`

Result: `life-experience` was the sole routed match.

The temporary explicit `life-experience` entry that had been added to `skill-router` during diagnosis was removed before closeout. The final behavior therefore comes from generic managed Skill trigger discovery rather than a hard-coded route.

## Rollback

Immediate bounded rollback binary:

`/home/agent/.local/share/codexpro-bridge/rollback-3.1.3-trigger-routing-20260909T145000Z/codexpro-bridge-3.1.2`

Because 3.1.3 does not change Work schema, provider configuration, MCP schema, public URL, or credentials, rollback is limited to restoring the saved 3.1.2 binary and restarting only `codexpro-bridge.service`, followed by health/surface/routing verification.

The deeper 3.1 cutover rollback package remains unchanged for schema-level rollback to 3.0.1.

## Result

**PASS.** Bridge 3.1.3 keeps the small-cannon architecture intact while making managed Skill triggers actually usable for natural multilingual routing. No duplicate router, public-schema drift, provider coupling, or compatibility residue remains.
