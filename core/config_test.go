package core

import (
	"strings"
	"testing"
)

func TestConfigDefaultsAndValidation(t *testing.T) {
	t.Setenv("CODEXPRO_BRIDGE_HTTP_TOKEN", strings.Repeat("x", 32))
	for _, key := range []string{
		"CODEXPRO_BRIDGE_HOST", "CODEXPRO_BRIDGE_PORT", "CODEXPRO_BRIDGE_ALLOW_ANONYMOUS",
		"CODEXPRO_BRIDGE_SKILLS_ROOT", "CODEXPRO_BRIDGE_MCP_DISCOVERY_URL",
		"CODEXPRO_BRIDGE_MCP_TIMEOUT",
		"CODEXPRO_BRIDGE_MEMORY_TIMEOUT", "CODEXPRO_BRIDGE_WORK_DB_PATH", "CODEXPRO_BRIDGE_MODULES",
		"CODEXPRO_BRIDGE_MAX_OUTPUT_CHARS", "CODEXPRO_BRIDGE_MAX_REQUEST_BYTES", "CODEXPRO_HTTP_TOKEN",
	} {
		if key != "CODEXPRO_BRIDGE_HTTP_TOKEN" {
			t.Setenv(key, "")
		}
	}
	c, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "127.0.0.1" || c.Port != 18787 || c.MCPTimeoutSeconds != 180 || c.MemoryTimeoutSeconds != 180 || c.MaxRequestBytes != 4*1024*1024 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.MCPDiscoveryURL != "http://127.0.0.1:19091/config_dump" {
		t.Fatalf("unexpected MCP discovery default: %s", c.MCPDiscoveryURL)
	}
	if c.WorkDBPath == "" {
		t.Fatal("work path must have a default")
	}
}

func TestConfigRejectsNonLoopback(t *testing.T) {
	c := Config{
		Host: "0.0.0.0", Port: 18787, Token: strings.Repeat("x", 24),
		SkillsRoot: "/tmp", MCPDiscoveryURL: "http://127.0.0.1:19091/config_dump", WorkDBPath: "/tmp/work.db",
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected non-loopback rejection")
	}
}

func TestConfigRejectsNonLoopbackDiscovery(t *testing.T) {
	c := Config{
		Host: "127.0.0.1", Port: 18787, Token: strings.Repeat("x", 24),
		SkillsRoot: "/tmp", MCPDiscoveryURL: "http://10.0.0.5:19091/config_dump", WorkDBPath: "/tmp/work.db",
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected non-loopback discovery rejection")
	}
}

func TestRedactionRecursiveAndBounded(t *testing.T) {
	authKey := "Author" + "ization"
	input := map[string]any{
		authKey:  "Bear" + "er " + strings.Repeat("x", 12),
		"nested": map[string]any{"safe": "ok"},
	}
	out := RedactValue(input).(map[string]any)
	if out[authKey] != "[REDACTED]" {
		t.Fatalf("authorization-like field was not redacted: %#v", out)
	}
	nested := out["nested"].(map[string]any)
	if nested["safe"] != "ok" {
		t.Fatalf("safe nested data changed: %#v", nested)
	}
	bounded := Bounded(map[string]any{"x": "0123456789"}, 2).(map[string]any)
	if bounded["ok"] != false {
		t.Fatalf("expected bounded error: %#v", bounded)
	}
}

func TestErrorMapDoesNotExposeUnknownError(t *testing.T) {
	mapped := ErrorMap(assertError("internal detail"))
	if mapped["error"].(map[string]any)["message"] != "Bridge operation failed" {
		t.Fatalf("unexpected unknown error exposure: %#v", mapped)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
