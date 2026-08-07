---
name: codexpro-bridge-router
description: Use in ordinary Chat for every non-trivial task that may need CodexPro VPS operations or live Hermes Skills, MCP, Memos, and knowledge recall through CodexPro Bridge. Never use Work for this workflow. Route before acting; do not use for ordinary conversation or short calculations.
---

# CodexPro Bridge Router

Run this workflow in ordinary **Chat**, never **Work**. CodexPro and CodexPro
Bridge are MCP applications and do not need Work quota. If an application page
opens `?surface=work`, switch the ChatGPT surface to **Chat before sending the
first message**. A Work quota warning is a product-surface issue, not evidence
that either MCP server is unavailable.

For a non-trivial task, first call `codexpro_bridge_route_and_recall` on the
CodexPro Bridge connection. Choose a relevant Hermes Skill from its live
catalog and call `codexpro_bridge_load_skill` before acting. Load a referenced
resource only when that Skill or the task requires it.

Use the existing CodexPro connection for VPS files, Bash, Git, and writes. Use
CodexPro Bridge for live Hermes Skills, Hermes MCP tools, Memos, and doctor
status. Treat Memos results as retrieval leads; verify important knowledge in
its source of truth when the loaded Skill requires that.

For ordinary chat and short calculations, do not route or recall memory. Do
not use CodexPro Bridge to edit files, and do not copy or modify Hermes Skill
content outside its actual Hermes path. Before changing a Hermes Skill, load
the target Skill, its relevant resources, the Skill-maintenance guidance, and
the applicable `AGENTS.md` chain.
