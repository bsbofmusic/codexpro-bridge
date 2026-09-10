package shared_mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "1.2.0"

type Transport interface {
	ListTools(context.Context) ([]*mcp.Tool, error)
	CallTool(context.Context, string, map[string]any) (*mcp.CallToolResult, error)
}

type RouteDiscovery interface {
	Endpoints(context.Context) ([]string, error)
}

type adminRouteDiscovery struct {
	endpoint string
}

func NewAdminRouteDiscovery(endpoint string) RouteDiscovery {
	return &adminRouteDiscovery{endpoint: endpoint}
}

type configDump struct {
	Binds []struct {
		Address   string `json:"address"`
		Listeners map[string]struct {
			Routes map[string]struct {
				Matches []struct {
					Path struct {
						Exact string `json:"exact"`
					} `json:"path"`
				} `json:"matches"`
			} `json:"routes"`
		} `json:"listeners"`
	} `json:"binds"`
}

func (d *adminRouteDiscovery) Endpoints(ctx context.Context) ([]string, error) {
	if d == nil || strings.TrimSpace(d.endpoint) == "" {
		return nil, fmt.Errorf("MCP discovery endpoint is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP discovery returned HTTP %d", resp.StatusCode)
	}
	var dump configDump
	if err := json.NewDecoder(resp.Body).Decode(&dump); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	endpoints := []string{}
	for _, bind := range dump.Binds {
		_, port, err := net.SplitHostPort(bind.Address)
		if err != nil || port == "" {
			continue
		}
		for _, listener := range bind.Listeners {
			for _, route := range listener.Routes {
				for _, match := range route.Matches {
					path := match.Path.Exact
					slug := strings.TrimPrefix(path, "/mcp/")
					if !strings.HasPrefix(path, "/mcp/") || slug == "" || strings.Contains(slug, "/") {
						continue
					}
					endpoint := "http://127.0.0.1:" + port + path
					if !seen[endpoint] {
						seen[endpoint] = true
						endpoints = append(endpoints, endpoint)
					}
				}
			}
		}
	}
	sort.Strings(endpoints)
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no dedicated MCP routes discovered")
	}
	return endpoints, nil
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
	Discovery         RouteDiscovery
	DiscoveryEndpoint string
	NewTransport      func(string) Transport
	Timeout           time.Duration
	MaxOutput         int
}

type ResolvedTool struct {
	Tool         *mcp.Tool
	UpstreamName string
	Route        string
	owner        Transport
}

func (r *ResolvedTool) ReadOnly() bool {
	return r != nil && r.Tool != nil && r.Tool.Annotations != nil && r.Tool.Annotations.ReadOnlyHint
}

func New(config core.Config) *Runtime {
	clientName := fmt.Sprintf("codexpro-bridge-%d", config.Port)
	return &Runtime{
		Discovery: NewAdminRouteDiscovery(config.MCPDiscoveryURL), DiscoveryEndpoint: config.MCPDiscoveryURL,
		NewTransport: func(endpoint string) Transport { return NewSDKTransport(endpoint, clientName) },
		Timeout:      time.Duration(config.MCPTimeoutSeconds) * time.Second, MaxOutput: config.MaxOutputChars,
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

func (r *Runtime) activeTransports(ctx context.Context) ([]Transport, []string, error) {
	if r.Discovery == nil || r.NewTransport == nil {
		return nil, nil, core.Err("mcp_unavailable", "Shared MCP discovery is unavailable")
	}
	endpoints, err := r.Discovery.Endpoints(ctx)
	if err != nil || len(endpoints) == 0 {
		log.Printf("shared_mcp discovery_failed endpoint=%s error=%v", r.DiscoveryEndpoint, err)
		return nil, nil, core.Err("mcp_unavailable", "Shared MCP route discovery is unavailable")
	}
	transports := make([]Transport, 0, len(endpoints))
	for _, endpoint := range endpoints {
		transports = append(transports, r.NewTransport(endpoint))
	}
	return transports, endpoints, nil
}

type catalogEntry struct {
	tool         *mcp.Tool
	upstreamName string
	route        string
	owner        Transport
}

func routeSlug(endpoint string) string {
	const marker = "/mcp/"
	i := strings.LastIndex(endpoint, marker)
	if i < 0 {
		return "route"
	}
	slug := strings.Trim(endpoint[i+len(marker):], "/")
	if slug == "" || strings.Contains(slug, "/") {
		return "route"
	}
	return slug
}

func (r *Runtime) TransportForRoute(ctx context.Context, slug string) (Transport, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" || strings.Contains(slug, "/") || r.Discovery == nil || r.NewTransport == nil {
		return nil, core.Err("mcp_unavailable", "Shared MCP route is unavailable")
	}
	endpoints, err := r.Discovery.Endpoints(ctx)
	if err != nil {
		return nil, core.Err("mcp_unavailable", "Shared MCP route discovery is unavailable")
	}
	var matched string
	for _, endpoint := range endpoints {
		if routeSlug(endpoint) != slug {
			continue
		}
		if matched != "" {
			return nil, core.Err("mcp_catalog_collision", "Shared MCP route identity is ambiguous")
		}
		matched = endpoint
	}
	if matched == "" {
		return nil, core.Err("mcp_unavailable", "Shared MCP route is unavailable")
	}
	return r.NewTransport(matched), nil
}

func (r *Runtime) catalog(parent context.Context) ([]catalogEntry, []string, []string, error) {
	ctx, cancel := r.withTimeout(parent)
	defer cancel()
	all, endpoints, err := r.activeTransports(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	failed := []string{}
	succeeded := 0
	type candidate struct {
		tool     *mcp.Tool
		owner    Transport
		endpoint string
	}
	candidates := []candidate{}
	nameCounts := map[string]int{}
	for i, transport := range all {
		items, listErr := transport.ListTools(ctx)
		if listErr != nil {
			failed = append(failed, endpoints[i])
			continue
		}
		succeeded++
		for _, tool := range items {
			if tool == nil || tool.Name == "" {
				continue
			}
			candidates = append(candidates, candidate{tool: tool, owner: transport, endpoint: endpoints[i]})
			nameCounts[tool.Name]++
		}
	}
	if succeeded == 0 {
		return nil, endpoints, failed, core.Err("mcp_unavailable", "Shared MCP routes are unavailable")
	}
	entries := make([]catalogEntry, 0, len(candidates))
	canonicalSeen := map[string]bool{}
	for _, candidate := range candidates {
		upstreamName := candidate.tool.Name
		canonicalName := upstreamName
		route := routeSlug(candidate.endpoint)
		if nameCounts[upstreamName] > 1 {
			canonicalName = route + "__" + upstreamName
		}
		if canonicalSeen[canonicalName] {
			return nil, endpoints, failed, core.Err("mcp_catalog_collision", "Shared MCP canonical tool names collide across routes")
		}
		canonicalSeen[canonicalName] = true
		toolCopy := *candidate.tool
		toolCopy.Name = canonicalName
		entries = append(entries, catalogEntry{tool: &toolCopy, upstreamName: upstreamName, route: route, owner: candidate.owner})
	}
	return entries, endpoints, failed, nil
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
	catalog, endpoints, failed, err := r.catalog(ctx)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	items := make([]map[string]any, 0, len(catalog))
	for _, entry := range catalog {
		tool := entry.tool
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
		"route_count": len(endpoints), "degraded": len(failed) > 0, "failed_routes": failed,
	}, nil
}

func (r *Runtime) Status(ctx context.Context) map[string]any {
	catalog, endpoints, failed, err := r.catalog(ctx)
	if err != nil {
		return core.ErrorMap(err)
	}
	return map[string]any{
		"ok": true, "source": "agentgateway", "tool_count": len(catalog),
		"discovery_endpoint": r.DiscoveryEndpoint, "route_endpoints": endpoints, "route_count": len(endpoints),
		"degraded": len(failed) > 0, "failed_routes": failed,
	}
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
	catalog, _, failed, err := r.catalog(ctx)
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
	for _, entry := range catalog {
		tool := entry.tool
		if tool != nil && wanted[tool.Name] {
			resolved[tool.Name] = &ResolvedTool{Tool: tool, UpstreamName: entry.upstreamName, Route: entry.route, owner: entry.owner}
		}
	}
	if len(resolved) != len(wanted) && len(failed) > 0 {
		return nil, core.Err("mcp_unavailable", "Shared MCP catalog is degraded; the requested tool may be on an unavailable route")
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
	upstreamName := selected.UpstreamName
	if upstreamName == "" {
		upstreamName = selected.Tool.Name
	}
	result, err := selected.owner.CallTool(callCtx, upstreamName, arguments)
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
		"ok": true, "transport": "streamable-http", "source": "agentgateway", "discovery_endpoint": r.DiscoveryEndpoint,
	}
}
