# CodexPro Bridge 2.1

## Goal

Provide ChatGPT with a thin, modular bridge to shared Skills, MCP tools, memory, and durable work state without becoming a second general-purpose execution platform.

## Sources of truth

- Skills: live Skills Manager catalog.
- Shared MCP: live AgentGateway Core/XYDC catalog.
- Shared memory: direct Obsidian/MemOS MCP surfaces.
- File/Bash/Git/tests/deployment: separate official CodexPro/CyberKate execution plane.
- Work lifecycle state: Bridge Work Runtime database outside the source tree.

## Public-surface contract

The public tool surface is derived from enabled capability modules. Do not maintain a fixed tool-count contract in this file.

A healthy deployment requires:

- actual tools equal the union declared by enabled modules;
- no missing or orphan tools;
- duplicate module/tool registrations rejected;
- live client schema refreshed after module-surface changes.

## Daily-use contract

- Ordinary one-shot tasks stay stateless.
- Non-trivial tasks route first and load the best PRIMARY Skill on demand.
- Shared memory is used when prior-session continuity is relevant.
- Work Runtime is used when durable TODO/checkpoint/audit/artifact state adds value.
- AgentGateway tools are discovered narrowly only when an external shared capability is needed.
- Work records lifecycle state and evidence; executors perform real workspace or external-system actions.

## Release line

Runtime 2.1 adds Work Runtime and modular surface governance on top of the 2.0 Skills/MCP/Memory architecture. See `README.md`, `CHANGELOG.md`, and `docs/` for the maintained contract.
