package core

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Host                 string
	Port                 int
	Token                string
	AllowAnonymous       bool
	MaxRequestBytes      int
	SkillsRoot           string
	MCPURL               string
	OptionalURLs         []string
	MCPTimeoutSeconds    int
	ObsidianMCPCommand   string
	MemosMCPCommand      string
	MemoryTimeoutSeconds int
	WorkDBPath           string
	MaxOutputChars       int
	Modules              []string
}

func envInt(key string, fallback, min, max int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid_config: %s must be an integer", key)
	}
	if value < min || value > max {
		return 0, fmt.Errorf("invalid_config: %s must be between %d and %d", key, min, max)
	}
	return value, nil
}

func envBool(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envModules() []string {
	raw := strings.TrimSpace(os.Getenv("CODEXPRO_BRIDGE_MODULES"))
	if raw == "" || raw == "*" || strings.EqualFold(raw, "all") || strings.EqualFold(raw, "auto") {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func envList(key string) []string {
	out := []string{}
	for _, item := range strings.Split(os.Getenv(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func FromEnv() (Config, error) {
	port, err := envInt("CODEXPRO_BRIDGE_PORT", 18787, 1024, 65535)
	if err != nil {
		return Config{}, err
	}
	mcpTimeout, err := envInt("CODEXPRO_BRIDGE_MCP_TIMEOUT", 180, 5, 600)
	if err != nil {
		return Config{}, err
	}
	memoryTimeout, err := envInt("CODEXPRO_BRIDGE_MEMORY_TIMEOUT", 180, 5, 600)
	if err != nil {
		return Config{}, err
	}
	maxOutput, err := envInt("CODEXPRO_BRIDGE_MAX_OUTPUT_CHARS", 120_000, 4_096, 500_000)
	if err != nil {
		return Config{}, err
	}
	maxRequestBytes, err := envInt("CODEXPRO_BRIDGE_MAX_REQUEST_BYTES", 4*1024*1024, 64*1024, 16*1024*1024)
	if err != nil {
		return Config{}, err
	}
	token := os.Getenv("CODEXPRO_BRIDGE_HTTP_TOKEN")
	if token == "" {
		token = os.Getenv("CODEXPRO_HTTP_TOKEN")
	}
	c := Config{
		Host:                 envDefault("CODEXPRO_BRIDGE_HOST", "127.0.0.1"),
		Port:                 port,
		Token:                token,
		AllowAnonymous:       envBool("CODEXPRO_BRIDGE_ALLOW_ANONYMOUS", false),
		MaxRequestBytes:      maxRequestBytes,
		SkillsRoot:           envDefault("CODEXPRO_BRIDGE_SKILLS_ROOT", "/home/agent/.skills-manager/skills"),
		MCPURL:               envDefault("CODEXPRO_BRIDGE_MCP_URL", "http://127.0.0.1:19090/mcp"),
		OptionalURLs:         envList("CODEXPRO_BRIDGE_MCP_OPTIONAL_URLS"),
		MCPTimeoutSeconds:    mcpTimeout,
		ObsidianMCPCommand:   envDefault("CODEXPRO_BRIDGE_OBSIDIAN_MCP_COMMAND", "/home/agent/.local/share/agent-stack/memory/obsidian-mcp.sh"),
		MemosMCPCommand:      envDefault("CODEXPRO_BRIDGE_MEMOS_MCP_COMMAND", "/home/agent/.local/share/agent-stack/memory/memos-api-mcp.sh"),
		MemoryTimeoutSeconds: memoryTimeout,
		WorkDBPath:           envDefault("CODEXPRO_BRIDGE_WORK_DB_PATH", "/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3"),
		MaxOutputChars:       maxOutput,
		Modules:              envModules(),
	}
	return c, c.Validate()
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (c Config) Validate() error {
	if c.Host != "127.0.0.1" && c.Host != "::1" && c.Host != "localhost" {
		return fmt.Errorf("invalid_config: CodexPro Bridge must listen on loopback only")
	}
	if !c.AllowAnonymous && c.Token == "" {
		return fmt.Errorf("missing_auth: Bridge HTTP authentication token is not configured")
	}
	if c.Token != "" && len([]byte(c.Token)) < 24 {
		return fmt.Errorf("weak_auth: Bridge HTTP authentication token is too short")
	}
	if !filepath.IsAbs(c.SkillsRoot) {
		return fmt.Errorf("invalid_config: Shared Skill root must be an absolute path")
	}
	for _, endpoint := range append([]string{c.MCPURL}, c.OptionalURLs...) {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !isLoopbackHost(u.Hostname()) {
			return fmt.Errorf("invalid_config: Shared MCP endpoints must be loopback HTTP(S) URLs")
		}
	}
	for _, command := range []string{c.ObsidianMCPCommand, c.MemosMCPCommand} {
		if !filepath.IsAbs(command) {
			return fmt.Errorf("invalid_config: Shared memory MCP commands must be absolute paths")
		}
	}
	if !filepath.IsAbs(c.WorkDBPath) {
		return fmt.Errorf("invalid_config: Work Runtime database path must be absolute")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func (c Config) Public() map[string]any {
	modules := any("auto")
	if c.Modules != nil {
		modules = append([]string(nil), c.Modules...)
	}
	return map[string]any{
		"host": c.Host, "port": c.Port, "auth_required": !c.AllowAnonymous,
		"max_request_bytes": c.MaxRequestBytes,
		"skills_root":       c.SkillsRoot, "mcp_url": c.MCPURL, "mcp_optional_urls": append([]string(nil), c.OptionalURLs...),
		"mcp_timeout_seconds": c.MCPTimeoutSeconds, "obsidian_mcp_command": c.ObsidianMCPCommand,
		"memos_mcp_command": c.MemosMCPCommand, "memory_timeout_seconds": c.MemoryTimeoutSeconds,
		"work_db_path": c.WorkDBPath, "enabled_modules": modules, "max_output_chars": c.MaxOutputChars,
	}
}
