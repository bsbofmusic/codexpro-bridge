package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/codexpro/bridge/accelerator"
	"github.com/codexpro/bridge/capabilities"
	"github.com/codexpro/bridge/core"
	"github.com/codexpro/bridge/shared_mcp"
	"github.com/codexpro/bridge/shared_memory"
	"github.com/codexpro/bridge/skills"
	"github.com/codexpro/bridge/work"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const RuntimeVersion = core.RuntimeVersion

type Capabilities struct {
	Config          core.Config
	Registry        *capabilities.Registry
	Skills          *skills.Runtime
	MCP             *shared_mcp.Runtime
	Memory          *shared_memory.Runtime
	Work            *work.Runtime
	Accelerator     *accelerator.Runtime
	Adapters        []capabilities.Adapter
	Operations      map[string]capabilities.Operation
	ToolDefinitions map[string]mcp.Tool
	ActualTools     []string
}

func Build(config core.Config) (*mcp.Server, *Capabilities, error) {
	return BuildWithAdapters(config)
}

func BuildWithAdapters(config core.Config, extras ...capabilities.Adapter) (*mcp.Server, *Capabilities, error) {
	mcpRuntime := shared_mcp.New(config)
	workRuntime := work.New(config.WorkDBPath)
	caps := &Capabilities{
		Config:      config,
		Registry:    capabilities.New(config.Modules),
		Skills:      skills.New(config.SkillsRoot),
		MCP:         mcpRuntime,
		Memory:      shared_memory.New(config, mcpRuntime),
		Work:        workRuntime,
		Accelerator: accelerator.New(mcpRuntime, workRuntime),
	}
	adapters := append(caps.defaultAdapters(), extras...)
	if err := caps.registerAdapters(adapters); err != nil {
		return nil, nil, err
	}
	if err := caps.Registry.ValidateSelection(); err != nil {
		return nil, nil, err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "CodexPro Bridge", Version: RuntimeVersion}, &mcp.ServerOptions{Instructions: buildInstructions(caps.Registry.EnabledIDs())})
	for _, module := range caps.Registry.Enabled() {
		for _, name := range module.Tools {
			tool, ok := caps.ToolDefinitions[name]
			if !ok {
				return nil, nil, fmt.Errorf("registry_surface_mismatch: no tool definition for %s", name)
			}
			operation, ok := caps.Operations[name]
			if !ok || operation == nil {
				return nil, nil, fmt.Errorf("registry_surface_mismatch: no operation for %s", name)
			}
			toolDef := tool
			server.AddTool(&toolDef, caps.operationHandler(name, operation))
			caps.ActualTools = append(caps.ActualTools, name)
		}
	}
	audit := caps.Registry.SurfaceAudit(caps.ActualTools)
	if consistent, _ := audit["consistent"].(bool); !consistent {
		return nil, nil, fmt.Errorf("registry_surface_mismatch: missing=%v orphan=%v", audit["missing_tools"], audit["orphan_tools"])
	}
	return server, caps, nil
}

func (c *Capabilities) skillsHealth() map[string]any {
	result, err := c.Skills.SkillsList("", "", 0, 1)
	if err != nil {
		return core.ErrorMap(err)
	}
	return map[string]any{"ok": true, "skill_count": result["count"], "source": result["source"]}
}

func buildInstructions(enabled []string) string {
	set := map[string]bool{}
	for _, id := range enabled {
		set[id] = true
	}
	if set["shared_skills"] {
		memory := ""
		if set["shared_memory"] {
			memory = " Shared memory recall is available when the memory module is enabled."
		}
		return "For non-trivial work, call codexpro_bridge_route_and_recall first. Choose a matching managed Skill and load its SKILL.md before acting." + memory + " Use the separate CodexPro application for VPS files, Bash, Git and edits."
	}
	return "Use the enabled Bridge modules for shared capabilities. Use the separate CodexPro application for VPS files, Bash, Git and edits."
}

func (c *Capabilities) operationHandler(name string, operation capabilities.Operation) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return c.safe(ctx, name, req, func(args map[string]any) (map[string]any, error) {
			return operation(ctx, args)
		})
	}
}

func parseArgs(req *mcp.CallToolRequest) (map[string]any, error) {
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return map[string]any{}, nil
	}
	args := map[string]any{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, core.Err("invalid_arguments", "Tool arguments must be an object")
	}
	return args, nil
}

func (c *Capabilities) safe(ctx context.Context, toolName string, req *mcp.CallToolRequest, operation func(map[string]any) (map[string]any, error)) (*mcp.CallToolResult, error) {
	log.Printf("tool_call name=%s", toolName)
	args, err := parseArgs(req)
	var payload any
	if err == nil {
		payload, err = operation(args)
	}
	if err != nil {
		payload = core.ErrorMap(err)
	}
	payload = core.Bounded(payload, c.Config.MaxOutputChars)
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		payload = map[string]any{"ok": false, "error": map[string]any{"code": "internal_error", "message": "Bridge operation failed"}}
		encoded, _ = json.Marshal(payload)
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		StructuredContent: payload,
	}, nil
}

func str(args map[string]any, key string) string {
	if value, ok := args[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func integer(args map[string]any, key string, fallback int) int {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch x := value.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	default:
		return fallback
	}
}

func boolean(args map[string]any, key string, fallback bool) bool {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	if result, ok := value.(bool); ok {
		return result
	}
	return fallback
}

func object(args map[string]any, key string) map[string]any {
	if value, ok := args[key].(map[string]any); ok {
		return value
	}
	return nil
}

func (c *Capabilities) routeAndRecall(ctx context.Context, task string, includeMemory bool, conversationFirstMessage string, skillLimit, memoryLimit int) (map[string]any, error) {
	routing, routeErr := c.Skills.Route(task, skillLimit)
	degraded := false
	if routeErr != nil {
		be, ok := routeErr.(core.BridgeError)
		if !ok || be.Code != "skills_unavailable" {
			return nil, routeErr
		}
		degraded = true
		routing = map[string]any{
			"ok":          false,
			"degraded":    true,
			"source":      "skills-manager",
			"skills":      []map[string]any{},
			"match_count": 0,
			"error":       map[string]any{"code": be.Code, "message": be.Message},
		}
	}

	memory := map[string]any{"requested": includeMemory, "attempted": false, "reason": "disabled"}
	if includeMemory && c.Registry.IsEnabled("shared_memory") {
		memory = c.Memory.RecallIfNeeded(ctx, task, conversationFirstMessage, memoryLimit)
		if ok, exists := memory["ok"].(bool); exists && !ok {
			degraded = true
		}
		if state, _ := memory["degraded"].(bool); state {
			degraded = true
		}
	} else if includeMemory {
		memory = map[string]any{"requested": true, "attempted": false, "reason": "module_disabled"}
	}
	return map[string]any{"ok": true, "degraded": degraded, "task": task, "routing": routing, "memory": memory}, nil
}

func (c *Capabilities) Doctor(ctx context.Context, deep bool) map[string]any {
	surface := c.Registry.SurfaceAudit(c.ActualTools)
	healthEntries := c.Registry.Health()
	healthByModule := make(map[string]map[string]any, len(healthEntries))
	for _, entry := range healthEntries {
		module := fmt.Sprint(entry["module"])
		if health, ok := entry["health"].(map[string]any); ok {
			healthByModule[module] = health
		}
	}
	result := map[string]any{
		"ok": surface["consistent"], "configuration": c.Config.Public(), "modules": c.Registry.Manifest(),
		"surface": surface, "health": healthEntries,
	}
	if deep {
		if c.Registry.IsEnabled("shared_skills") {
			listed, err := c.Skills.SkillsList("", "", 0, 1)
			if err != nil {
				result["skill_read"] = core.ErrorMap(err)
				result["ok"] = false
			} else if items, ok := listed["skills"].([]map[string]any); ok && len(items) > 0 {
				name := fmt.Sprint(items[0]["name"])
				loaded, loadErr := c.Skills.Load(name)
				if loadErr != nil {
					result["skill_read"] = core.ErrorMap(loadErr)
					result["ok"] = false
				} else {
					result["skill_read"] = map[string]any{"ok": loaded["ok"], "name": name}
				}
			} else {
				result["skill_read"] = map[string]any{"ok": false, "message": "No managed Skills are enabled"}
				result["ok"] = false
			}
		}
		if c.Registry.IsEnabled("shared_mcp") {
			status := healthByModule["shared_mcp"]
			result["mcp_status"] = status
			if ok, _ := status["ok"].(bool); !ok {
				result["ok"] = false
			}
		}
		if c.Registry.IsEnabled("shared_memory") {
			status := healthByModule["shared_memory"]
			result["memory_status"] = status
			if ok, _ := status["ok"].(bool); !ok {
				result["ok"] = false
			}
		}
		if c.Registry.IsEnabled("work_runtime") {
			status := healthByModule["work_runtime"]
			result["work_runtime"] = status
			if ok, _ := status["ok"].(bool); !ok {
				result["ok"] = false
			}
		}
	}
	for _, entry := range result["health"].([]map[string]any) {
		health, _ := entry["health"].(map[string]any)
		if ok, _ := health["ok"].(bool); !ok {
			result["ok"] = false
		}
		if degraded, _ := health["degraded"].(bool); degraded {
			result["degraded"] = true
		}
	}
	bounded := core.Bounded(result, c.Config.MaxOutputChars)
	if output, ok := bounded.(map[string]any); ok {
		return output
	}
	return map[string]any{"ok": false, "error": map[string]any{"code": "internal_error", "message": "Bridge operation failed"}}
}

func (c *Capabilities) HealthResponse() map[string]any {
	surface := c.Registry.SurfaceAudit(c.ActualTools)
	return map[string]any{
		"ok": surface["consistent"], "service": "codexpro-bridge", "auth_required": !c.Config.AllowAnonymous,
		"modules": surface["enabled_modules"], "tool_count": surface["tool_count"], "surface_fingerprint": surface["surface_fingerprint"],
	}
}
