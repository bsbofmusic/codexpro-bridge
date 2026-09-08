---
name: codexpro-bridge-router
description: Use for non-trivial tasks that may need shared VPS Skills, shared MCP tools, prior context from Obsidian/MemOS, or durable Work state through CodexPro Bridge. Skip ordinary chat and short calculations.
---

# CodexPro Bridge Router

Use Bridge with progressive disclosure: keep the default path short and expand only when the task needs another capability.

For non-trivial work, call `codexpro_bridge_route_and_recall` first. For a new standalone task, normally set memory off; enable recall when the user asks to continue, recover, compare with, or reuse prior context. A memory lookup never depends on a Skill name matching the query.

Choose the best managed PRIMARY Skill and call `codexpro_bridge_load_skill` before acting. Load only that Skill first; add one SECONDARY only when the task genuinely crosses owners. Load referenced resources only when the selected Skill or task requires them.

Use Bridge capabilities only on demand:

- **Memory** — use `codexpro_bridge_memory_search` or the direct memory list/call tools when the task explicitly depends on prior sessions or shared-memory content. Do not search memory for every new task.
- **Work** — use `codexpro_bridge_work` when lifecycle state has value: explicit TODO execution, multi-step durable work, checkpoint/resume, project context, mechanical audit, artifact tracking, completion tracking, or prompts such as “列 TODO / 逐项做 / 做完 / 机械审计 / 继续未完成任务”. Do not create Work state for ordinary one-shot answers.
- **MCP** — use `codexpro_bridge_mcp_list` / `codexpro_bridge_mcp_call` only when the task or selected Skill needs an external shared capability. Query narrowly and invoke the exact tool; do not enumerate the full AgentGateway catalog by default.

Work records state and evidence; it is not an executor. Use the separate CodexPro connection for VPS files, Bash, Git, repository inspection, tests, deployments, and writes. External-system actions stay with the corresponding upstream MCP. After real execution, write concise outcomes and mechanical evidence back to Work. If a final Work audit is required, freeze material task/step state before creating the audit so PASS stays bound to the current revision.

Bridge does not own Skill or MCP state, and it does not own memory state; Skills Manager, AgentGateway, Obsidian and MemOS remain authoritative. Never mirror their catalogs into a second index or add a hidden scheduler/router just to automate these usage rules.

For ordinary chat, short calculations, rewrites, or questions already answerable from the current conversation, skip Bridge routing and extra state. Never replay an upstream operation after a delivery-unknown result.
