package shared_memory

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/codexpro/bridge/shared_mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "1.2.1"

var memorySignal = regexp.MustCompile(`(?i)(\bremember\b|\bmemory\b|\brecall\b|\bprevious(?:ly)?\b|\bprior\b|\bhistory\b|\bknowledge\s*base\b|\bsecond\s*brain\b|\bpreferences?\b|记忆|回忆|之前|以前|过去|历史|第二大脑|知识库|偏好|上次|此前|先前|说过|聊过)`)

type routeTransport struct {
	shared *shared_mcp.Runtime
	route  string
}

func (t *routeTransport) transport(ctx context.Context) (shared_mcp.Transport, error) {
	if t == nil || t.shared == nil {
		return nil, core.Err("memory_unavailable", "Shared memory MCP source is unavailable")
	}
	transport, err := t.shared.TransportForRoute(ctx, t.route)
	if err != nil {
		return nil, core.Err("memory_unavailable", "Shared memory MCP source is unavailable")
	}
	return transport, nil
}

func (t *routeTransport) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	transport, err := t.transport(ctx)
	if err != nil {
		return nil, err
	}
	return transport.ListTools(ctx)
}

func (t *routeTransport) CallTool(ctx context.Context, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	transport, err := t.transport(ctx)
	if err != nil {
		return nil, err
	}
	return transport.CallTool(ctx, name, arguments)
}

type Runtime struct {
	Transports     map[string]shared_mcp.Transport
	SearchBindings map[string]SearchBinding
	Timeout        time.Duration
	MaxOutput      int
}

type SearchBinding struct {
	Tool      string
	Arguments func(query, conversationFirstMessage string, limit int) map[string]any
}

func New(config core.Config, mcpRuntime *shared_mcp.Runtime) *Runtime {
	return &Runtime{
		Transports: map[string]shared_mcp.Transport{
			"obsidian": &routeTransport{shared: mcpRuntime, route: "obsidian"},
			"memos":    &routeTransport{shared: mcpRuntime, route: "memos"},
		},
		SearchBindings: map[string]SearchBinding{
			"obsidian": {
				Tool: "search_content",
				Arguments: func(query, _ string, limit int) map[string]any {
					return map[string]any{"query": query, "max_results": limit}
				},
			},
			"memos": {
				Tool: "search_memory",
				Arguments: func(query, conversationFirstMessage string, limit int) map[string]any {
					return map[string]any{
						"query": query, "conversation_first_message": conversationFirstMessage, "memory_limit_number": limit,
					}
				},
			},
		},
		Timeout:   time.Duration(config.MemoryTimeoutSeconds) * time.Second,
		MaxOutput: config.MaxOutputChars,
	}
}

func (r *Runtime) withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	d := r.Timeout
	if d <= 0 {
		d = 180 * time.Second
	}
	// Memory calls reuse Shared MCP transports, so outbound MCP sessions must
	// not inherit transport/session values from the inbound Bridge MCP handler.
	// Preserve only a tighter caller deadline and start from a clean context.
	if deadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < d {
			d = remaining
		}
	}
	return context.WithTimeout(context.Background(), d)
}

func (r *Runtime) sourceNames() []string {
	names := make([]string, 0, len(r.Transports))
	for name, transport := range r.Transports {
		if strings.TrimSpace(name) != "" && transport != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (r *Runtime) searchableSourceNames() []string {
	names := []string{}
	for _, name := range r.sourceNames() {
		binding, ok := r.SearchBindings[name]
		if ok && strings.TrimSpace(binding.Tool) != "" && binding.Arguments != nil {
			names = append(names, name)
		}
	}
	return names
}

func (r *Runtime) validSource(source string) bool {
	transport, ok := r.Transports[source]
	return ok && transport != nil
}

func (r *Runtime) source(source string) (shared_mcp.Transport, error) {
	if !r.validSource(source) {
		return nil, core.Err("invalid_memory_source", "Memory source is not registered")
	}
	transport := r.Transports[source]
	if transport == nil {
		return nil, core.Err("memory_unavailable", "Requested memory source is unavailable")
	}
	return transport, nil
}

func toolAnnotations(tool *mcp.Tool) map[string]any {
	if tool == nil || tool.Annotations == nil {
		return nil
	}
	b, err := json.Marshal(tool.Annotations)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	if json.Unmarshal(b, &out) != nil || len(out) == 0 {
		return nil
	}
	return out
}

func (r *Runtime) ListTools(parent context.Context, source, query string, includeSchema bool, offset, limit int) (map[string]any, error) {
	if source != "" && !r.validSource(source) {
		return nil, core.Err("invalid_memory_source", "Memory source is not registered")
	}
	if len(query) > 256 {
		return nil, core.Err("invalid_query", "Memory tool query is invalid")
	}
	if offset < 0 {
		return nil, core.Err("invalid_pagination", "offset must be a non-negative integer")
	}
	if limit < 1 || limit > 100 {
		return nil, core.Err("invalid_pagination", "limit must be between 1 and 100")
	}
	wanted := r.sourceNames()
	if source != "" {
		wanted = []string{source}
	}
	ctx, cancel := r.withTimeout(parent)
	defer cancel()
	needle := strings.ToLower(strings.TrimSpace(query))
	items := []map[string]any{}
	degraded := []string{}
	for _, current := range wanted {
		transport, err := r.source(current)
		if err != nil {
			return nil, err
		}
		tools, err := transport.ListTools(ctx)
		if err != nil {
			if source != "" {
				if _, ok := err.(core.BridgeError); ok {
					return nil, err
				}
				return nil, core.Err("memory_unavailable", "Shared memory MCP source is unavailable")
			}
			degraded = append(degraded, current)
			continue
		}
		for _, tool := range tools {
			if tool == nil || tool.Name == "" {
				continue
			}
			if needle != "" && !strings.Contains(strings.ToLower(tool.Name), needle) && !strings.Contains(strings.ToLower(tool.Description), needle) {
				continue
			}
			item := map[string]any{"source": current, "name": tool.Name, "description": tool.Description}
			if includeSchema {
				if tool.InputSchema == nil {
					item["input_schema"] = map[string]any{"type": "object"}
				} else {
					item["input_schema"] = tool.InputSchema
				}
				if annotations := toolAnnotations(tool); annotations != nil {
					item["annotations"] = annotations
				}
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i]["source"] != items[j]["source"] {
			return items[i]["source"].(string) < items[j]["source"].(string)
		}
		return items[i]["name"].(string) < items[j]["name"].(string)
	})
	total := len(items)
	end := offset + limit
	if end > total {
		end = total
	}
	page := []map[string]any{}
	if offset < total {
		page = items[offset:end]
	}
	var next any
	if end < total {
		next = end
	}
	return map[string]any{
		"ok": total > 0 || len(degraded) == 0, "source": "shared-memory", "tools": page,
		"count": total, "offset": offset, "limit": limit, "next_offset": next, "degraded_sources": degraded,
	}, nil
}

func (r *Runtime) CallTool(parent context.Context, source, name string, arguments map[string]any) (map[string]any, error) {
	if name == "" || len(name) > 256 {
		return nil, core.Err("invalid_tool", "Memory MCP tool is invalid")
	}
	transport, err := r.source(source)
	if err != nil {
		return nil, err
	}
	if arguments == nil {
		arguments = map[string]any{}
	}
	ctx, cancel := r.withTimeout(parent)
	defer cancel()
	result, err := transport.CallTool(ctx, name, arguments)
	if err != nil {
		if _, ok := err.(core.BridgeError); ok {
			return nil, err
		}
		return nil, core.Err("delivery_unknown", "Memory MCP delivery status is unknown; the call was not replayed")
	}
	return map[string]any{
		"ok": true, "source": source, "tool": name, "result": shared_mcp.NormalizeCallResult(result),
	}, nil
}

func (r *Runtime) Search(parent context.Context, query, source string, limit int, conversationFirstMessage string) (map[string]any, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 8_000 {
		return nil, core.Err("invalid_query", "Memory search query is invalid")
	}
	if source == "" {
		source = "auto"
	}
	if source != "auto" && !r.validSource(source) {
		return nil, core.Err("invalid_memory_source", "Memory source is not registered")
	}
	if limit < 1 || limit > 25 {
		return nil, core.Err("invalid_limit", "Memory search limit must be between 1 and 25")
	}
	first := conversationFirstMessage
	if first == "" {
		first = query
	}
	wanted := r.searchableSourceNames()
	if source != "auto" {
		wanted = []string{source}
	}
	results := []map[string]any{}
	degraded := []string{}
	for _, current := range wanted {
		binding, ok := r.SearchBindings[current]
		if !ok || strings.TrimSpace(binding.Tool) == "" || binding.Arguments == nil {
			err := core.Err("memory_search_unavailable", "Requested memory source does not expose a search binding")
			if source != "auto" {
				return nil, err
			}
			degraded = append(degraded, current)
			errMap := core.ErrorMap(err)
			results = append(results, map[string]any{"ok": false, "source": current, "error": errMap["error"]})
			continue
		}
		item, err := r.CallTool(parent, current, binding.Tool, binding.Arguments(query, first, limit))
		if err != nil {
			degraded = append(degraded, current)
			errMap := core.ErrorMap(err)
			results = append(results, map[string]any{"ok": false, "source": current, "error": errMap["error"]})
			continue
		}
		results = append(results, item)
	}
	ok := false
	for _, item := range results {
		if value, _ := item["ok"].(bool); value {
			ok = true
			break
		}
	}
	return map[string]any{
		"ok": ok, "source": "shared-memory", "query": query, "results": results, "degraded_sources": degraded,
	}, nil
}

func ShouldRecall(task string) bool {
	return memorySignal.MatchString(task)
}

func (r *Runtime) RecallIfNeeded(ctx context.Context, task, conversationFirstMessage string, limit int) map[string]any {
	if !ShouldRecall(task) {
		return map[string]any{"requested": true, "attempted": false, "reason": "no_strong_memory_signal"}
	}
	result, err := r.Search(ctx, task, "auto", limit, conversationFirstMessage)
	if err != nil {
		out := core.ErrorMap(err)
		out["requested"] = true
		out["attempted"] = true
		return out
	}
	result["requested"] = true
	result["attempted"] = true
	return result
}

func (r *Runtime) Status(parent context.Context) map[string]any {
	sources := []map[string]any{}
	healthy := 0
	for _, source := range r.sourceNames() {
		transport, err := r.source(source)
		if err != nil {
			sources = append(sources, map[string]any{"source": source, "ok": false, "error": core.ErrorMap(err)["error"]})
			continue
		}
		ctx, cancel := r.withTimeout(parent)
		tools, err := transport.ListTools(ctx)
		cancel()
		if err != nil {
			sources = append(sources, map[string]any{"source": source, "ok": false, "error": core.ErrorMap(err)["error"]})
			continue
		}
		healthy++
		sources = append(sources, map[string]any{"source": source, "ok": true, "tool_count": len(tools)})
	}
	registered := len(r.sourceNames())
	return map[string]any{
		"ok": healthy > 0, "degraded": healthy != registered, "source": "shared-memory", "sources": sources,
	}
}
