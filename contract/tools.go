package contract

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// pythonContract is captured directly from the production Python 2.1.1
// tools/list response. It remains the immutable legacy compatibility oracle.
// Go-only extension tools are declared separately below.
//
//go:embed python-2.1.1-tools.json
var pythonContract []byte

var moduleTools = map[string][]string{
	"shared_skills": {
		"codexpro_bridge_route_and_recall",
		"codexpro_bridge_route",
		"codexpro_bridge_skills_list",
		"codexpro_bridge_load_skill",
		"codexpro_bridge_load_skill_resource",
	},
	"shared_mcp": {
		"codexpro_bridge_mcp_list",
		"codexpro_bridge_mcp_status",
		"codexpro_bridge_mcp_call",
	},
	"shared_memory": {
		"codexpro_bridge_memory_search",
		"codexpro_bridge_memory_list",
		"codexpro_bridge_memory_call",
	},
	"work_runtime":    {"codexpro_bridge_work"},
	"web_accelerator": {"codexpro_bridge_accelerator"},
	"bridge_doctor":   {"codexpro_bridge_doctor"},
}

var byName map[string]mcp.Tool

func init() {
	legacy := LegacyTools()
	byName = make(map[string]mcp.Tool, len(legacy)+1)
	for _, tool := range legacy {
		if tool.Name == "" {
			panic("invalid embedded Bridge public contract: empty tool name")
		}
		if _, exists := byName[tool.Name]; exists {
			panic("invalid embedded Bridge public contract: duplicate tool name " + tool.Name)
		}
		byName[tool.Name] = tool
	}
	for _, tool := range extensionTools() {
		if tool.Name == "" {
			panic("invalid Bridge extension contract: empty tool name")
		}
		if _, exists := byName[tool.Name]; exists {
			panic("invalid Bridge extension contract: duplicate tool name " + tool.Name)
		}
		byName[tool.Name] = tool
	}
	for module, names := range moduleTools {
		seen := map[string]bool{}
		for _, name := range names {
			if seen[name] {
				panic("duplicate tool in module contract: " + module + "/" + name)
			}
			seen[name] = true
			if _, ok := byName[name]; !ok {
				panic("module references missing contract tool: " + module + "/" + name)
			}
		}
	}
}

func ModuleNames() []string {
	return []string{"shared_skills", "shared_mcp", "shared_memory", "work_runtime", "web_accelerator", "bridge_doctor"}
}

func ToolNames(module string) []string {
	return append([]string(nil), moduleTools[module]...)
}

func Tool(name string) (mcp.Tool, bool) {
	tool, ok := byName[name]
	return tool, ok
}

func ToolsForModule(module string) []mcp.Tool {
	names := moduleTools[module]
	out := make([]mcp.Tool, 0, len(names))
	for _, name := range names {
		if tool, ok := byName[name]; ok {
			out = append(out, tool)
		}
	}
	return out
}

func AllTools() []mcp.Tool {
	out := make([]mcp.Tool, 0, len(byName))
	for _, tool := range byName {
		out = append(out, tool)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func LegacyTools() []mcp.Tool {
	var tools []mcp.Tool
	if err := json.Unmarshal(pythonContract, &tools); err != nil {
		panic(fmt.Sprintf("invalid embedded Bridge public contract: %v", err))
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

func CanonicalJSON() []byte {
	return append([]byte(nil), pythonContract...)
}

func extensionTools() []mcp.Tool {
	trueValue := true
	return []mcp.Tool{{
		Name:        "codexpro_bridge_accelerator",
		Description: "Deterministic Web accelerator for resume capsules, conversation-task affinity, operation receipts, no-replay idempotency, bounded result references/shaping, and limited MCP batch execution. It contains no AI planner, autonomous loop, scheduler, or background agent.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: &trueValue,
			OpenWorldHint:   &trueValue,
		},
		InputSchema: map[string]any{
			"type":  "object",
			"title": "acceleratorArguments",
			"properties": map[string]any{
				"operation": map[string]any{
					"type":  "string",
					"title": "Operation",
					"enum": []any{
						"conversation.bind", "conversation.resolve", "conversation.unbind",
						"resume.capsule", "mcp.call", "mcp.batch",
						"receipt.prepare", "receipt.finalize", "receipt.get", "receipt.list",
						"result.read", "result.delete",
					},
				},
				"arguments": map[string]any{
					"type":                 "object",
					"title":                "Arguments",
					"additionalProperties": true,
				},
			},
			"required": []any{"operation"},
		},
		OutputSchema: map[string]any{
			"type":                 "object",
			"title":                "acceleratorDictOutput",
			"additionalProperties": true,
		},
	}}
}
