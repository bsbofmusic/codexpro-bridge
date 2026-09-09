package server

import (
	"context"
	"fmt"

	"github.com/codexpro/bridge/accelerator"
	"github.com/codexpro/bridge/capabilities"
	"github.com/codexpro/bridge/contract"
	"github.com/codexpro/bridge/shared_mcp"
	"github.com/codexpro/bridge/shared_memory"
	"github.com/codexpro/bridge/work"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (c *Capabilities) defaultAdapters() []capabilities.Adapter {
	return []capabilities.Adapter{
		capabilities.NewStaticAdapter(
			capabilities.Descriptor{ID: "shared_skills", Kind: "skills", Version: "1.0.0", Traits: []string{"catalog", "route", "retrieve"}},
			contract.ToolsForModule("shared_skills"),
			map[string]capabilities.Operation{
				"codexpro_bridge_route_and_recall": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return c.routeAndRecall(ctx, str(args, "task"), boolean(args, "include_memory", true), str(args, "conversation_first_message"), integer(args, "skill_limit", 8), integer(args, "memory_limit", 6))
				},
				"codexpro_bridge_route": func(_ context.Context, args map[string]any) (map[string]any, error) {
					return c.Skills.Route(str(args, "task"), integer(args, "limit", 8))
				},
				"codexpro_bridge_skills_list": func(_ context.Context, args map[string]any) (map[string]any, error) {
					return c.Skills.SkillsList(str(args, "category"), str(args, "query"), integer(args, "offset", 0), integer(args, "limit", 50))
				},
				"codexpro_bridge_load_skill": func(_ context.Context, args map[string]any) (map[string]any, error) {
					return c.Skills.Load(str(args, "name"))
				},
				"codexpro_bridge_load_skill_resource": func(_ context.Context, args map[string]any) (map[string]any, error) {
					return c.Skills.Resource(str(args, "name"), str(args, "resource_path"))
				},
			},
			func(context.Context) map[string]any { return c.skillsHealth() },
		),
		capabilities.NewStaticAdapter(
			capabilities.Descriptor{ID: "shared_mcp", Kind: "mcp", Version: shared_mcp.Version, Traits: []string{"catalog", "invoke"}},
			contract.ToolsForModule("shared_mcp"),
			map[string]capabilities.Operation{
				"codexpro_bridge_mcp_list": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return c.MCP.ListTools(ctx, str(args, "query"), boolean(args, "include_schema", false), integer(args, "offset", 0), integer(args, "limit", 100))
				},
				"codexpro_bridge_mcp_status": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
					return c.MCP.Status(ctx), nil
				},
				"codexpro_bridge_mcp_call": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return c.MCP.CallTool(ctx, str(args, "tool"), object(args, "arguments"))
				},
			},
			func(ctx context.Context) map[string]any { return c.MCP.Status(ctx) },
		),
		capabilities.NewStaticAdapter(
			capabilities.Descriptor{ID: "shared_memory", Kind: "memory", Version: shared_memory.Version, Traits: []string{"catalog", "search", "invoke"}},
			contract.ToolsForModule("shared_memory"),
			map[string]capabilities.Operation{
				"codexpro_bridge_memory_search": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					source := str(args, "source")
					if source == "" {
						source = "auto"
					}
					return c.Memory.Search(ctx, str(args, "query"), source, integer(args, "limit", 6), str(args, "conversation_first_message"))
				},
				"codexpro_bridge_memory_list": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return c.Memory.ListTools(ctx, str(args, "source"), str(args, "query"), boolean(args, "include_schema", false), integer(args, "offset", 0), integer(args, "limit", 100))
				},
				"codexpro_bridge_memory_call": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return c.Memory.CallTool(ctx, str(args, "source"), str(args, "tool"), object(args, "arguments"))
				},
			},
			func(ctx context.Context) map[string]any { return c.Memory.Status(ctx) },
		),
		capabilities.NewStaticAdapter(
			capabilities.Descriptor{ID: "work_runtime", Kind: "state", Version: work.Version, Traits: []string{"state", "checkpoint", "audit"}},
			contract.ToolsForModule("work_runtime"),
			map[string]capabilities.Operation{
				"codexpro_bridge_work": func(_ context.Context, args map[string]any) (map[string]any, error) {
					return c.Work.Dispatch(str(args, "operation"), object(args, "arguments"))
				},
			},
			func(context.Context) map[string]any { return c.Work.Health() },
		),
		capabilities.NewStaticAdapter(
			capabilities.Descriptor{
				ID: "web_accelerator", Kind: "accelerator", Version: accelerator.Version,
				Dependencies: []string{"shared_mcp", "work_runtime"},
				Traits:       []string{"resume", "receipt", "batch", "result_ref"},
			},
			contract.ToolsForModule("web_accelerator"),
			map[string]capabilities.Operation{
				"codexpro_bridge_accelerator": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return c.Accelerator.Dispatch(ctx, str(args, "operation"), object(args, "arguments"))
				},
			},
			func(context.Context) map[string]any { return c.Accelerator.Health() },
		),
		capabilities.NewStaticAdapter(
			capabilities.Descriptor{ID: "bridge_doctor", Kind: "core", Version: "1.0.0", Traits: []string{"health", "audit"}},
			contract.ToolsForModule("bridge_doctor"),
			map[string]capabilities.Operation{
				"codexpro_bridge_doctor": func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return c.Doctor(ctx, boolean(args, "deep", false)), nil
				},
			},
			func(context.Context) map[string]any { return map[string]any{"ok": true} },
		),
	}
}

func (c *Capabilities) registerAdapters(adapters []capabilities.Adapter) error {
	if c.Operations == nil {
		c.Operations = map[string]capabilities.Operation{}
	}
	if c.ToolDefinitions == nil {
		c.ToolDefinitions = map[string]mcp.Tool{}
	}
	for _, adapter := range adapters {
		if adapter == nil {
			return fmt.Errorf("adapter_invalid: Bridge capability adapter cannot be nil")
		}
		descriptor := adapter.Descriptor()
		if descriptor.ID == "" || descriptor.Kind == "" || descriptor.Version == "" {
			return fmt.Errorf("adapter_invalid: Bridge capability adapter identity is incomplete")
		}
		tools := adapter.Tools()
		operations := adapter.Operations()
		if len(tools) == 0 {
			return fmt.Errorf("adapter_invalid: Bridge capability adapter must expose at least one tool")
		}
		names := make([]string, 0, len(tools))
		seen := map[string]bool{}
		for _, tool := range tools {
			if tool.Name == "" || seen[tool.Name] {
				return fmt.Errorf("adapter_invalid: Bridge capability adapter has an invalid or duplicate tool")
			}
			operation, ok := operations[tool.Name]
			if !ok || operation == nil {
				return fmt.Errorf("adapter_invalid: no operation binding for %s", tool.Name)
			}
			seen[tool.Name] = true
			names = append(names, tool.Name)
			c.Operations[tool.Name] = operation
			c.ToolDefinitions[tool.Name] = tool
		}
		for name := range operations {
			if !seen[name] {
				return fmt.Errorf("adapter_invalid: operation %s has no public tool definition", name)
			}
		}
		adapter := adapter
		if err := c.Registry.Register(capabilities.Module{
			ID: descriptor.ID, Kind: descriptor.Kind, Version: descriptor.Version,
			Dependencies: descriptor.Dependencies, Traits: descriptor.Traits, Tools: names,
			Health: func() map[string]any { return adapter.Health(context.Background()) },
		}); err != nil {
			return err
		}
		c.Adapters = append(c.Adapters, adapter)
	}
	return nil
}
