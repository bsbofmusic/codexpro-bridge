package shared_mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeTransport struct {
	tools      []*mcp.Tool
	listErr    error
	callErr    error
	callResult *mcp.CallToolResult
	calls      int
	lastName   string
	lastArgs   map[string]any
}

func (f *fakeTransport) ListTools(context.Context) ([]*mcp.Tool, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tools, nil
}

func (f *fakeTransport) CallTool(_ context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	f.calls++
	f.lastName, f.lastArgs = name, args
	if f.callErr != nil {
		return nil, f.callErr
	}
	return f.callResult, nil
}

type fakeDiscovery struct {
	endpoints []string
	err       error
}

func (f fakeDiscovery) Endpoints(context.Context) ([]string, error) {
	return append([]string(nil), f.endpoints...), f.err
}

func runtimeWith(endpoints []string, transports map[string]*fakeTransport) *Runtime {
	return &Runtime{
		Discovery:         fakeDiscovery{endpoints: endpoints},
		DiscoveryEndpoint: "http://127.0.0.1:19091/config_dump",
		NewTransport: func(endpoint string) Transport {
			return transports[endpoint]
		},
		Timeout: time.Second,
	}
}

func TestListToolsUsesDiscoveredRoutesAndReportsPartialFailure(t *testing.T) {
	a := &fakeTransport{tools: []*mcp.Tool{
		{Name: "zeta", Description: "z", InputSchema: map[string]any{"type": "object"}},
		{Name: "alpha", Description: "route-a"},
	}}
	b := &fakeTransport{tools: []*mcp.Tool{
		{Name: "alpha", Description: "route-b duplicate"},
		{Name: "beta", Description: "b"},
	}}
	c := &fakeTransport{listErr: errors.New("offline")}
	endpoints := []string{"http://127.0.0.1/mcp/a", "http://127.0.0.1/mcp/b", "http://127.0.0.1/mcp/c"}
	r := runtimeWith(endpoints, map[string]*fakeTransport{endpoints[0]: a, endpoints[1]: b, endpoints[2]: c})

	got, err := r.ListTools(context.Background(), "", true, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	items := got["tools"].([]map[string]any)
	if len(items) != 4 || items[0]["name"] != "a__alpha" || items[1]["name"] != "b__alpha" || items[2]["name"] != "beta" || items[3]["name"] != "zeta" {
		t.Fatalf("unexpected catalog: %#v", items)
	}
	if items[0]["description"] != "route-a" || items[1]["description"] != "route-b duplicate" {
		t.Fatalf("route-qualified duplicate tools drifted: %#v", items)
	}
	if got["degraded"] != true || got["route_count"] != 3 {
		t.Fatalf("partial route failure not surfaced: %#v", got)
	}
	failed := got["failed_routes"].([]string)
	if len(failed) != 1 || failed[0] != endpoints[2] {
		t.Fatalf("unexpected failed routes: %#v", failed)
	}
}

func TestDuplicateToolNamesAreRouteQualifiedAndDispatchToCorrectOwner(t *testing.T) {
	a := &fakeTransport{
		tools:      []*mcp.Tool{{Name: "search_content", Description: "a"}},
		callResult: &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "a-ok"}}},
	}
	b := &fakeTransport{
		tools:      []*mcp.Tool{{Name: "search_content", Description: "b"}},
		callResult: &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "b-ok"}}},
	}
	endpoints := []string{"http://127.0.0.1/mcp/obsidian", "http://127.0.0.1/mcp/sessions"}
	r := runtimeWith(endpoints, map[string]*fakeTransport{endpoints[0]: a, endpoints[1]: b})

	got, err := r.ListTools(context.Background(), "search_content", false, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	items := got["tools"].([]map[string]any)
	if len(items) != 2 || items[0]["name"] != "obsidian__search_content" || items[1]["name"] != "sessions__search_content" {
		t.Fatalf("duplicate tools were not independently exposed: %#v", items)
	}
	if _, err := r.CallTool(context.Background(), "search_content", nil); err == nil {
		t.Fatal("ambiguous bare duplicate unexpectedly resolved")
	}
	if _, err := r.CallTool(context.Background(), "sessions__search_content", map[string]any{"q": "x"}); err != nil {
		t.Fatal(err)
	}
	if b.calls != 1 || a.calls != 0 || b.lastName != "search_content" {
		t.Fatalf("qualified tool dispatched to wrong upstream: a=%d b=%d last=%q", a.calls, b.calls, b.lastName)
	}
}

func TestCallToolUsesOwningDiscoveredRouteAndPreservesDeliveryUnknown(t *testing.T) {
	a := &fakeTransport{tools: []*mcp.Tool{{Name: "alpha"}}}
	b := &fakeTransport{
		tools:      []*mcp.Tool{{Name: "beta"}},
		callResult: &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}},
	}
	endpoints := []string{"http://127.0.0.1/mcp/a", "http://127.0.0.1/mcp/b"}
	r := runtimeWith(endpoints, map[string]*fakeTransport{endpoints[0]: a, endpoints[1]: b})

	got, err := r.CallTool(context.Background(), "beta", map[string]any{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if b.calls != 1 || a.calls != 0 || b.lastName != "beta" {
		t.Fatalf("wrong dispatch a=%d b=%d", a.calls, b.calls)
	}
	result := got["result"].(map[string]any)
	content := result["content"].([]map[string]any)
	if len(content) != 1 || content[0]["text"] != "ok" {
		t.Fatalf("normalization failed: %#v", result)
	}

	b.callErr = core.Err("delivery_unknown", "MCP delivery status is unknown; the call was not replayed")
	_, err = r.CallTool(context.Background(), "beta", nil)
	be, ok := err.(core.BridgeError)
	if !ok || be.Code != "delivery_unknown" {
		t.Fatalf("delivery_unknown lost: %#v", err)
	}
}

func TestResolveToolsCatalogsOnceAndExposesReadOnlyHint(t *testing.T) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	route := &fakeTransport{tools: []*mcp.Tool{{Name: "read", Annotations: readOnly}, {Name: "write"}}}
	endpoint := "http://127.0.0.1/mcp/route"
	r := runtimeWith([]string{endpoint}, map[string]*fakeTransport{endpoint: route})

	resolved, err := r.ResolveTools(context.Background(), []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 2 || !resolved["read"].ReadOnly() || resolved["write"].ReadOnly() {
		t.Fatalf("resolved tool annotations drifted: %#v", resolved)
	}
	if _, err := r.CallResolved(context.Background(), resolved["read"], map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if route.calls != 1 {
		t.Fatalf("resolved dispatch did not use owning route: %d", route.calls)
	}
}

func TestDiscoveryFailureIsFatalWithoutLegacyFallback(t *testing.T) {
	r := &Runtime{
		Discovery:         fakeDiscovery{err: errors.New("admin unavailable")},
		DiscoveryEndpoint: "http://127.0.0.1:19091/config_dump",
		NewTransport:      func(string) Transport { return &fakeTransport{} },
		Timeout:           time.Second,
	}
	_, err := r.ListTools(context.Background(), "", false, 0, 100)
	be, ok := err.(core.BridgeError)
	if !ok || be.Code != "mcp_unavailable" {
		t.Fatalf("unexpected discovery failure: %#v", err)
	}
}

func TestMissingToolDuringPartialFailureReturnsUnavailable(t *testing.T) {
	good := &fakeTransport{tools: []*mcp.Tool{{Name: "alpha"}}}
	bad := &fakeTransport{listErr: errors.New("offline")}
	endpoints := []string{"http://127.0.0.1/mcp/good", "http://127.0.0.1/mcp/bad"}
	r := runtimeWith(endpoints, map[string]*fakeTransport{endpoints[0]: good, endpoints[1]: bad})
	_, err := r.CallTool(context.Background(), "missing", nil)
	be, ok := err.(core.BridgeError)
	if !ok || be.Code != "mcp_unavailable" {
		t.Fatalf("degraded catalog should not misreport missing tool as denied: %#v", err)
	}
}

func TestTransportForRouteUsesDynamicDiscoveryAndRejectsAmbiguity(t *testing.T) {
	a := &fakeTransport{}
	b := &fakeTransport{}
	endpoints := []string{"http://127.0.0.1:19090/mcp/memos", "http://127.0.0.1:19090/mcp/obsidian"}
	r := runtimeWith(endpoints, map[string]*fakeTransport{endpoints[0]: a, endpoints[1]: b})

	got, err := r.TransportForRoute(context.Background(), "obsidian")
	if err != nil || got != b {
		t.Fatalf("dynamic route transport resolution failed: got=%#v err=%v", got, err)
	}
	if _, err := r.TransportForRoute(context.Background(), "missing"); err == nil {
		t.Fatal("missing route unexpectedly resolved")
	}

	duplicate := []string{"http://127.0.0.1:19090/mcp/memos", "http://127.0.0.1:19091/mcp/memos"}
	r = runtimeWith(duplicate, map[string]*fakeTransport{duplicate[0]: a, duplicate[1]: b})
	if _, err := r.TransportForRoute(context.Background(), "memos"); err == nil {
		t.Fatal("ambiguous route identity unexpectedly resolved")
	}
}

func TestAdminRouteDiscoveryExtractsDedicatedRoutesOnly(t *testing.T) {
	var bindAddress string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"binds":[{"address":%q,"listeners":{"gateway/default":{"routes":{`+
			`"compat":{"matches":[{"path":{"exact":"/mcp"}}]},`+
			`"evomap":{"matches":[{"path":{"exact":"/mcp/evomap"}}]},`+
			`"nested":{"matches":[{"path":{"exact":"/mcp/foo/bar"}}]}`+
			`}}}}]}`, bindAddress)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	bindAddress = "[::]:" + port

	endpoints, err := NewAdminRouteDiscovery(server.URL).Endpoints(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := "http://127.0.0.1:" + port + "/mcp/evomap"
	if len(endpoints) != 1 || endpoints[0] != want {
		t.Fatalf("unexpected dedicated-route discovery: %#v", endpoints)
	}
}

func TestNormalizeCallResultMarksUnsupportedContent(t *testing.T) {
	got := NormalizeCallResult(&mcp.CallToolResult{Content: []mcp.Content{
		&mcp.ImageContent{MIMEType: "image/png"},
	}})
	content := got["content"].([]map[string]any)
	if content[0]["type"] != "image" || content[0]["supported"] != false {
		t.Fatalf("unsupported media handling drifted: %#v", content)
	}
}

func TestNormalizeCallResultPreservesPython21ErrorFlagCompatibility(t *testing.T) {
	got := NormalizeCallResult(&mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: "upstream tool error"}},
	})
	if got["is_error"] != false {
		t.Fatalf("Python 2.1.1 compatibility drifted: %#v", got)
	}
}
