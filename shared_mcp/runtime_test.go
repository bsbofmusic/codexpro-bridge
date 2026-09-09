package shared_mcp

import (
	"context"
	"errors"
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

func TestListToolsDedupesPrimaryFirstAndDegradesOptional(t *testing.T) {
	primary := &fakeTransport{tools: []*mcp.Tool{
		{Name: "zeta", Description: "z", InputSchema: map[string]any{"type": "object"}},
		{Name: "alpha", Description: "primary"},
	}}
	optional := &fakeTransport{tools: []*mcp.Tool{
		{Name: "alpha", Description: "optional duplicate"},
		{Name: "beta", Description: "b"},
	}}
	degraded := &fakeTransport{listErr: errors.New("offline")}
	r := &Runtime{Primary: primary, Optional: []Transport{optional, degraded}, PrimaryEndpoint: "http://127.0.0.1/mcp", Timeout: time.Second}
	got, err := r.ListTools(context.Background(), "", true, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	items := got["tools"].([]map[string]any)
	if len(items) != 3 || items[0]["name"] != "alpha" || items[1]["name"] != "beta" || items[2]["name"] != "zeta" {
		t.Fatalf("unexpected catalog: %#v", items)
	}
	if items[0]["description"] != "primary" {
		t.Fatalf("optional endpoint won duplicate: %#v", items[0])
	}
	if _, ok := got["_transports"]; ok {
		t.Fatal("internal transport leaked into public result")
	}
}

func TestCallToolUsesOwningTransportAndPreservesDeliveryUnknown(t *testing.T) {
	primary := &fakeTransport{tools: []*mcp.Tool{{Name: "alpha"}}}
	optional := &fakeTransport{
		tools:      []*mcp.Tool{{Name: "beta"}},
		callResult: &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}},
	}
	r := &Runtime{Primary: primary, Optional: []Transport{optional}, Timeout: time.Second}
	got, err := r.CallTool(context.Background(), "beta", map[string]any{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if optional.calls != 1 || primary.calls != 0 || optional.lastName != "beta" {
		t.Fatalf("wrong dispatch primary=%d optional=%d", primary.calls, optional.calls)
	}
	result := got["result"].(map[string]any)
	content := result["content"].([]map[string]any)
	if len(content) != 1 || content[0]["text"] != "ok" {
		t.Fatalf("normalization failed: %#v", result)
	}

	optional.callErr = core.Err("delivery_unknown", "MCP delivery status is unknown; the call was not replayed")
	_, err = r.CallTool(context.Background(), "beta", nil)
	be, ok := err.(core.BridgeError)
	if !ok || be.Code != "delivery_unknown" {
		t.Fatalf("delivery_unknown lost: %#v", err)
	}
}

func TestResolveToolsCatalogsOnceAndExposesReadOnlyHint(t *testing.T) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	primary := &fakeTransport{tools: []*mcp.Tool{{Name: "read", Annotations: readOnly}, {Name: "write"}}}
	r := &Runtime{Primary: primary, Timeout: time.Second}
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
	if primary.calls != 1 {
		t.Fatalf("resolved dispatch did not use owning transport: %d", primary.calls)
	}
}

func TestPrimaryListFailureIsFatal(t *testing.T) {
	r := &Runtime{Primary: &fakeTransport{listErr: errors.New("offline")}, Timeout: time.Second}
	_, err := r.ListTools(context.Background(), "", false, 0, 100)
	be, ok := err.(core.BridgeError)
	if !ok || be.Code != "mcp_unavailable" {
		t.Fatalf("unexpected primary failure: %#v", err)
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
