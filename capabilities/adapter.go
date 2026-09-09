package capabilities

import (
	"context"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Operation is the smallest Bridge-facing execution contract. Adapters bind
// explicit public tool names to deterministic operations; Bridge core does not
// interpret provider-specific business semantics.
type Operation func(context.Context, map[string]any) (map[string]any, error)

// Descriptor is the provider-neutral module identity consumed by Bridge core.
// Traits are descriptive capability vocabulary (for example catalog, invoke,
// search, retrieve, state); they are not a workflow DSL or dynamic permission
// system.
type Descriptor struct {
	ID           string
	Kind         string
	Version      string
	Dependencies []string
	Traits       []string
}

// Adapter is the only interface a capability system must implement to join the
// Bridge public surface. Existing systems and future systems such as Vision are
// connected through thin adapters rather than by editing Bridge core logic.
type Adapter interface {
	Descriptor() Descriptor
	Tools() []mcp.Tool
	Operations() map[string]Operation
	Health(context.Context) map[string]any
}

// StaticAdapter is a small convenience implementation for built-in and simple
// external providers. It deliberately owns no retries, planning, caching,
// scheduling, memory, or workflow state.
type StaticAdapter struct {
	descriptor Descriptor
	tools      []mcp.Tool
	operations map[string]Operation
	health     func(context.Context) map[string]any
}

func NewStaticAdapter(descriptor Descriptor, tools []mcp.Tool, operations map[string]Operation, health func(context.Context) map[string]any) *StaticAdapter {
	return &StaticAdapter{
		descriptor: cloneDescriptor(descriptor),
		tools:      append([]mcp.Tool(nil), tools...),
		operations: cloneOperations(operations),
		health:     health,
	}
}

func (a *StaticAdapter) Descriptor() Descriptor {
	if a == nil {
		return Descriptor{}
	}
	return cloneDescriptor(a.descriptor)
}

func (a *StaticAdapter) Tools() []mcp.Tool {
	if a == nil {
		return nil
	}
	return append([]mcp.Tool(nil), a.tools...)
}

func (a *StaticAdapter) Operations() map[string]Operation {
	if a == nil {
		return map[string]Operation{}
	}
	return cloneOperations(a.operations)
}

func (a *StaticAdapter) Health(ctx context.Context) map[string]any {
	if a == nil || a.health == nil {
		return map[string]any{"ok": true}
	}
	return a.health(ctx)
}

func ToolNames(adapter Adapter) []string {
	if adapter == nil {
		return nil
	}
	names := make([]string, 0, len(adapter.Tools()))
	for _, tool := range adapter.Tools() {
		if tool.Name != "" {
			names = append(names, tool.Name)
		}
	}
	sort.Strings(names)
	return names
}

func cloneDescriptor(in Descriptor) Descriptor {
	return Descriptor{
		ID: in.ID, Kind: in.Kind, Version: in.Version,
		Dependencies: append([]string(nil), in.Dependencies...),
		Traits:       append([]string(nil), in.Traits...),
	}
}

func cloneOperations(in map[string]Operation) map[string]Operation {
	out := make(map[string]Operation, len(in))
	for name, operation := range in {
		out[name] = operation
	}
	return out
}
