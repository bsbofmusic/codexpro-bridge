package shared_mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "1.0.0"

type Transport interface {
	ListTools(context.Context) ([]*mcp.Tool, error)
	CallTool(context.Context, string, map[string]any) (*mcp.CallToolResult, error)
}

type sdkTransport struct {
	endpoint   string
	clientName string
}

func NewSDKTransport(endpoint, clientName string) Transport {
	return &sdkTransport{endpoint: endpoint, clientName: clientName}
}

func (t *sdkTransport) connect(ctx context.Context) (*mcp.ClientSession, error) {
	clientName := strings.TrimSpace(t.clientName)
	if clientName == "" {
		clientName = "codexpro-bridge"
	}
	client := mcp.NewClient(&mcp.Implementation{Name: clientName, Version: core.RuntimeVersion}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             t.endpoint,
		HTTPClient:           http.DefaultClient,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Printf("shared_mcp connect_failed endpoint=%s error=%T:%v", t.endpoint, err, err)
		return nil, err
	}
	return session, nil
}

func (t *sdkTransport) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	session, err := t.connect(ctx)
	if err != nil {
		return nil, core.Err("mcp_unavailable", "Shared MCP gateway is unavailable")
	}
	defer session.Close()
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Printf("shared_mcp list_failed endpoint=%s error=%T:%v", t.endpoint, err, err)
		return nil, core.Err("mcp_unavailable", "Shared MCP gateway is unavailable")
	}
	if result == nil {
		return []*mcp.Tool{}, nil
	}
	return result.Tools, nil
}

func (t *sdkTransport) CallTool(ctx context.Context, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	session, err := t.connect(ctx)
	if err != nil {
		return nil, core.Err("mcp_unavailable", "Shared MCP call could not be delivered")
	}
	defer session.Close()
	// Once the MCP session is initialized, a tool request may have reached the
	// upstream even if the response path fails. Never replay after this point.
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		log.Printf("shared_mcp call_failed endpoint=%s tool=%s error=%T:%v", t.endpoint, name, err, err)
		return nil, core.Err("delivery_unknown", "MCP delivery status is unknown; the call was not replayed")
	}
	return result, nil
}

type Runtime struct {
	Primary           Transport
	Optional          []Transport
	PrimaryEndpoint   string
	OptionalEndpoints []string
	Timeout           time.Duration
	MaxOutput         int
}

type ResolvedTool struct {
	Tool  *mcp.Tool
	owner Transport
}

func (r *ResolvedTool) ReadOnly() bool {
	return r != nil && r.Tool != nil && r.Tool.Annotations != nil && r.Tool.Annotations.ReadOnlyHint
}

func New(config core.Config) *Runtime {
	clientName := fmt.Sprintf("codexpro-bridge-%d", config.Port)
	optional := make([]Transport, 0, len(config.OptionalURLs))
	for _, endpoint := range config.OptionalURLs {
		optional = append(optional, NewSDKTransport(endpoint, clientName))
	}
	return &Runtime{
		Primary: NewSDKTransport(config.MCPURL, clientName), Optional: optional,
		PrimaryEndpoint: config.MCPURL, OptionalEndpoints: append([]string(nil), config.OptionalURLs...),
		Timeout: time.Duration(config.MCPTimeoutSeconds) * time.Second, MaxOutput: config.MaxOutputChars,
	}
}

func (r *Runtime) withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	d := r.Timeout
	if d <= 0 {
		d = 180 * time.Second
	}
	// Never reuse an inbound MCP handler context for an outbound MCP client.
	// SDK/server contexts may carry transport/session values that belong only to
	// the inbound peer. Preserve a tighter caller deadline, but start from a
	// clean value context so nested MCP calls cannot leak session metadata.
	if deadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < d {
			d = remaining
		}
	}
	return context.WithTimeout(context.Background(), d)
}

func isBridgeError(err error) bool {
	_, ok := err.(core.BridgeError)
	return ok
}

func (r *Runtime) catalog(parent context.Context) ([]*mcp.Tool, []Transport, error) {
	ctx, cancel := r.withTimeout(parent)
	defer cancel()
	tools := []*mcp.Tool{}
	owners := []Transport{}
	seen := map[string]bool{}
	all := append([]Transport{r.Primary}, r.Optional...)
	for i, transport := range all {
		items, err := transport.ListTools(ctx)
		if err != nil {
			if i > 0 {
				continue
			}
			if isBridgeError(err) {
				return nil, nil, err
			}
			return nil, nil, core.Err("mcp_unavailable", "Shared MCP gateway is unavailable")
		}
		for _, tool := range items {
			if tool == nil || tool.Name == "" || seen[tool.Name] {
				continue
			}
			seen[tool.Name] = true
			tools = append(tools, tool)
			owners = append(owners, transport)
		}
	}
	return tools, owners, nil
}

func (r *Runtime) ListTools(ctx context.Context, query string, includeSchema bool, offset, limit int) (map[string]any, error) {
	if len(query) > 256 {
		return nil, core.Err("invalid_query", "MCP tool query is invalid")
	}
	if offset < 0 {
		return nil, core.Err("invalid_pagination", "offset must be a non-negative integer")
	}
	if limit < 1 || limit > 100 {
		return nil, core.Err("invalid_pagination", "limit must be between 1 and 100")
	}
	catalog, _, err := r.catalog(ctx)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	items := make([]map[string]any, 0, len(catalog))
	for _, tool := range catalog {
		if needle != "" && !strings.Contains(strings.ToLower(tool.Name), needle) && !strings.Contains(strings.ToLower(tool.Description), needle) {
			continue
		}
		item := map[string]any{"name": tool.Name, "description": tool.Description}
		if includeSchema {
			if tool.InputSchema == nil {
				item["input_schema"] = map[string]any{"type": "object"}
			} else {
				item["input_schema"] = tool.InputSchema
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["name"].(string) < items[j]["name"].(string) })
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
		"ok": true, "tools": page, "count": total, "offset": offset,
		"limit": limit, "next_offset": next, "source": "agentgateway",
	}, nil
}

func (r *Runtime) Status(ctx context.Context) map[string]any {
	catalog, _, err := r.catalog(ctx)
	if err != nil {
		return core.ErrorMap(err)
	}
	result := map[string]any{
		"ok": true, "source": "agentgateway", "endpoint": r.PrimaryEndpoint, "tool_count": len(catalog),
	}
	if len(r.OptionalEndpoints) > 0 {
		result["optional_endpoints"] = append([]string(nil), r.OptionalEndpoints...)
	}
	return result
}

func (r *Runtime) CallTool(ctx context.Context, name string, arguments map[string]any) (map[string]any, error) {
	resolved, err := r.ResolveTools(ctx, []string{name})
	if err != nil {
		return nil, err
	}
	selected := resolved[name]
	if selected == nil {
		return nil, core.Err("mcp_denied", "MCP tool is not exposed by the shared gateway")
	}
	return r.CallResolved(ctx, selected, arguments)
}

func (r *Runtime) ResolveTools(ctx context.Context, names []string) (map[string]*ResolvedTool, error) {
	catalog, owners, err := r.catalog(ctx)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 256 {
			return nil, core.Err("invalid_tool", "MCP tool is invalid")
		}
		wanted[name] = true
	}
	resolved := make(map[string]*ResolvedTool, len(wanted))
	for i, tool := range catalog {
		if tool != nil && wanted[tool.Name] {
			resolved[tool.Name] = &ResolvedTool{Tool: tool, owner: owners[i]}
		}
	}
	return resolved, nil
}

func (r *Runtime) CallResolved(ctx context.Context, selected *ResolvedTool, arguments map[string]any) (map[string]any, error) {
	if selected == nil || selected.Tool == nil || selected.owner == nil {
		return nil, core.Err("mcp_denied", "MCP tool is not exposed by the shared gateway")
	}
	if arguments == nil {
		arguments = map[string]any{}
	}
	callCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	result, err := selected.owner.CallTool(callCtx, selected.Tool.Name, arguments)
	if err != nil {
		if isBridgeError(err) {
			return nil, err
		}
		return nil, core.Err("delivery_unknown", "MCP delivery status is unknown; the call was not replayed")
	}
	return map[string]any{"ok": true, "tool": selected.Tool.Name, "result": NormalizeCallResult(result)}, nil
}

func (r *Runtime) Compatibility() map[string]any {
	return map[string]any{
		"ok": true, "transport": "streamable-http", "source": "agentgateway", "endpoint": r.PrimaryEndpoint,
	}
}
