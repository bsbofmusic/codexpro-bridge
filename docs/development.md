# Development

Install the editable package with its test dependency:

    python3.12 -m venv .venv
    .venv/bin/python -m pip install -e '.[dev]'
    .venv/bin/python -m pytest -q

The test suite is offline. It uses fake Hermes MCP objects and verifies the
fixed tool surface, loopback authentication, redaction, Skill-resource
validation, one-delivery dispatch behavior, Plugin manifest rendering, and
documentation links.

Do not add a copied Skill index, Hermes configuration copy, credential file,
or file/shell/root implementation to this project. Add behavior in a bounded
module under src/codexpro_bridge, document its public contract, and preserve
the existing ownership split with CodexPro.
