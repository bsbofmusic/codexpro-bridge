from __future__ import annotations

import os

import pytest

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import REDACTED, bounded_result, redact_text, redact_value


def _clear_bridge_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in tuple(os.environ):
        if name.startswith("CODEXPRO_BRIDGE_") or name == "CODEXPRO_HTTP_TOKEN":
            monkeypatch.delenv(name, raising=False)


def test_config_requires_loopback_and_a_nontrivial_token(monkeypatch: pytest.MonkeyPatch) -> None:
    _clear_bridge_environment(monkeypatch)
    with pytest.raises(BridgeError, match="authentication token"):
        BridgeConfig.from_env()

    monkeypatch.setenv("CODEXPRO_BRIDGE_HTTP_TOKEN", "a" * 32)
    monkeypatch.setenv("CODEXPRO_BRIDGE_HOST", "0.0.0.0")
    with pytest.raises(BridgeError, match="loopback"):
        BridgeConfig.from_env()

    monkeypatch.setenv("CODEXPRO_BRIDGE_HOST", "127.0.0.1")
    monkeypatch.setenv("CODEXPRO_BRIDGE_HTTP_TOKEN", "short")
    with pytest.raises(BridgeError, match="too short"):
        BridgeConfig.from_env()


def test_config_public_view_and_repr_do_not_disclose_token(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    _clear_bridge_environment(monkeypatch)
    token = "test-only-token-material-that-must-not-escape"
    monkeypatch.setenv("CODEXPRO_BRIDGE_HTTP_TOKEN", token)

    config = BridgeConfig.from_env()

    assert token not in repr(config)
    assert token not in str(config.public_dict())
    assert config.public_dict()["auth_required"] is True
    assert config.public_dict()["host"] == "127.0.0.1"


def test_recursive_redaction_handles_keys_and_credential_shaped_text() -> None:
    bearer = "Bearer example-token-abcdefghijklmnopqrstuvwxyz"
    key = "cpk_abcdefghijklmnopqrstuvwxyz0123456789"
    payload = {
        "Authorization": bearer,
        "nested": {
            "api_key": key,
            "url": "https://bridge.invalid/mcp?codexpro_token=" + key,
        },
        "list": ["token=" + key, b"private bytes"],
        "ordinary": "no credential here",
    }

    redacted = redact_value(payload)

    assert redacted["Authorization"] == REDACTED
    assert redacted["nested"]["api_key"] == REDACTED
    assert key not in repr(redacted)
    assert bearer not in repr(redacted)
    assert redacted["ordinary"] == "no credential here"
    assert redacted["list"][1] == "[binary data: 13 bytes]"
    # The input is not modified in-place because callers can reuse request data.
    assert payload["nested"]["api_key"] == key


@pytest.mark.parametrize(
    "raw",
    [
        "Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
        "api_key=sk_abcdefghijklmnopqrstuvwxyz0123456789",
        "https://example.invalid/?token=cpk_abcdefghijklmnopqrstuvwxyz0123456789",
        "-----BEGIN PRIVATE KEY-----\nnot-a-real-key\n-----END PRIVATE KEY-----",
    ],
)
def test_redact_text_removes_secret_shaped_values(raw: str) -> None:
    result = redact_text(raw)

    assert REDACTED in result
    assert "abcdefghijklmnopqrstuvwxyz" not in result
    assert "BEGIN PRIVATE KEY" not in result


def test_bounded_result_redacts_before_constructing_preview() -> None:
    token = "cpk_abcdefghijklmnopqrstuvwxyz0123456789"
    result = bounded_result({"description": ("x" * 1000) + token}, max_chars=300)

    assert result["ok"] is False
    assert result["error"]["code"] == "output_truncated"
    assert token not in repr(result)
    assert len(result["preview"]) <= 300
