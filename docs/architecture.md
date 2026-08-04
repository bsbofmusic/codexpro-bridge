# Architecture

CodexPro Bridge is an independent, loopback-only MCP server. It deliberately
does not replace CodexPro and does not invoke a Hermes language model.

    ChatGPT
       |                 |
       |                 +-- CodexPro: files, Bash, Git, VPS changes
       |
       +-- CodexPro Bridge: Skills, Hermes MCP, Memos, doctor
                                  |
                                  +-- Hermes Skill APIs
                                  +-- Hermes Native MCP runtime
                                  +-- Hermes-managed Memos configuration

The Skill module starts a short-lived worker for each request and calls
Hermes skills_list and skill_view. It never copies or indexes the Skill
library. The MCP module loads Hermes configuration, removes Hindsight, disables
sampling and elicitation in an in-memory copy, and registers its own upstream
connections through Hermes. Memos remains an enabled Hermes MCP server; Bridge
only changes its child executable in memory when configured to do so.

All public output passes through the same redaction and output-size boundary.
The generic MCP dispatcher makes at most one upstream call. When delivery is
ambiguous it returns delivery_unknown and does not retry the call.
