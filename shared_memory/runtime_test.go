package shared_memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/codexpro/bridge/shared_mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type memoryDiscovery struct {
	endpoints []string
}

func (d memoryDiscovery) Endpoints(context.Context) ([]string, error) {
	return append([]string(nil), d.endpoints...), nil
}

type memoryContextKey struct{}

type memoryFake struct {
	tools            []*mcp.Tool
	listErr          error
	callErr          error
	lastTool         string
	lastArgs         map[string]any
	lastContextValue any
	lastDeadline     time.Time
	hasDeadline      bool
	callResult       *mcp.CallToolResult
}

func (f *memoryFake) ListTools(context.Context) ([]*mcp.Tool, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tools, nil
}

func (f *memoryFake) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	f.lastTool, f.lastArgs = name, args
	f.lastContextValue = ctx.Value(memoryContextKey{})
	f.lastDeadline, f.hasDeadline = ctx.Deadline()
	if f.callErr != nil {
		return nil, f.callErr
	}
	if f.callResult == nil {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	}
	return f.callResult, nil
}

func TestNewUsesSharedMCPDynamicRoutesForMemorySources(t *testing.T) {
	obsidian := &memoryFake{tools: []*mcp.Tool{{Name: "search_content"}}}
	memos := &memoryFake{tools: []*mcp.Tool{{Name: "search_memory"}}}
	endpoints := []string{"http://127.0.0.1:19090/mcp/memos", "http://127.0.0.1:19090/mcp/obsidian"}
	shared := &shared_mcp.Runtime{
		Discovery: memoryDiscovery{endpoints: endpoints},
		NewTransport: func(endpoint string) shared_mcp.Transport {
			if endpoint == endpoints[0] {
				return memos
			}
			return obsidian
		},
		Timeout: time.Second,
	}
	r := New(core.Config{MemoryTimeoutSeconds: 1}, shared)
	got, err := r.ListTools(context.Background(), "", "", false, 0, 100)
	if err != nil || got["count"] != 2 {
		t.Fatalf("shared MCP memory transport did not resolve dynamically: %#v err=%v", got, err)
	}
}

func TestListToolsDegradesOneMemorySource(t *testing.T) {
	obsidian := &memoryFake{tools: []*mcp.Tool{{Name: "search_content", Description: "search"}}}
	memos := &memoryFake{listErr: errors.New("offline")}
	r := &Runtime{Transports: map[string]shared_mcp.Transport{"obsidian": obsidian, "memos": memos}, Timeout: time.Second}
	got, err := r.ListTools(context.Background(), "", "", false, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["count"] != 1 {
		t.Fatalf("unexpected degraded result: %#v", got)
	}
	degraded := got["degraded_sources"].([]string)
	if len(degraded) != 1 || degraded[0] != "memos" {
		t.Fatalf("wrong degraded source: %#v", degraded)
	}
}

func TestMemorySearchMapsToCanonicalUpstreamTools(t *testing.T) {
	obsidian := &memoryFake{}
	memos := &memoryFake{}
	r := &Runtime{
		Transports: map[string]shared_mcp.Transport{"obsidian": obsidian, "memos": memos},
		SearchBindings: map[string]SearchBinding{
			"obsidian": {Tool: "search_content", Arguments: func(query, _ string, limit int) map[string]any {
				return map[string]any{"query": query, "max_results": limit}
			}},
			"memos": {Tool: "search_memory", Arguments: func(query, first string, limit int) map[string]any {
				return map[string]any{"query": query, "conversation_first_message": first, "memory_limit_number": limit}
			}},
		},
		Timeout: time.Second,
	}
	got, err := r.Search(context.Background(), "bridge migration", "auto", 4, "first message")
	if err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || obsidian.lastTool != "search_content" || memos.lastTool != "search_memory" {
		t.Fatalf("search dispatch drifted: %#v", got)
	}
	if obsidian.lastArgs["max_results"] != 4 {
		t.Fatalf("obsidian args drifted: %#v", obsidian.lastArgs)
	}
	if memos.lastArgs["memory_limit_number"] != 4 || memos.lastArgs["conversation_first_message"] != "first message" {
		t.Fatalf("memos args drifted: %#v", memos.lastArgs)
	}
}

func TestOutboundMemoryCallDropsInboundContextValuesAndKeepsDeadline(t *testing.T) {
	fake := &memoryFake{}
	r := &Runtime{Transports: map[string]shared_mcp.Transport{"memos": fake}, Timeout: time.Second}
	parent := context.WithValue(context.Background(), memoryContextKey{}, "inbound-session")
	parent, cancel := context.WithTimeout(parent, 100*time.Millisecond)
	defer cancel()

	if _, err := r.CallTool(parent, "memos", "search_memory", map[string]any{"query": "x"}); err != nil {
		t.Fatal(err)
	}
	if fake.lastContextValue != nil {
		t.Fatalf("inbound context value leaked into outbound memory transport: %#v", fake.lastContextValue)
	}
	if !fake.hasDeadline {
		t.Fatal("outbound memory transport lost caller deadline")
	}
	if remaining := time.Until(fake.lastDeadline); remaining <= 0 || remaining > 150*time.Millisecond {
		t.Fatalf("unexpected outbound deadline remaining: %v", remaining)
	}
}

func TestRecallSignalAndSourceValidation(t *testing.T) {
	if !ShouldRecall("继续我们之前聊过的 Bridge") {
		t.Fatal("Chinese recall signal was missed")
	}
	if ShouldRecall("calculate 2+2") {
		t.Fatal("ordinary query should not trigger memory")
	}
	r := &Runtime{Transports: map[string]shared_mcp.Transport{}, Timeout: time.Second}
	if _, err := r.ListTools(context.Background(), "invalid", "", false, 0, 100); err == nil {
		t.Fatal("expected invalid source rejection")
	}
}

func TestDynamicMemorySourceRegistryNeedsNoCoreBranch(t *testing.T) {
	vector := &memoryFake{tools: []*mcp.Tool{{Name: "semantic_search", Description: "vector search"}}}
	r := &Runtime{
		Transports: map[string]shared_mcp.Transport{"vector": vector},
		SearchBindings: map[string]SearchBinding{
			"vector": {Tool: "semantic_search", Arguments: func(query, first string, limit int) map[string]any {
				return map[string]any{"text": query, "context": first, "top_k": limit}
			}},
		},
		Timeout: time.Second,
	}
	listed, err := r.ListTools(context.Background(), "", "", false, 0, 100)
	if err != nil || listed["count"] != 1 {
		t.Fatalf("dynamic source was not discovered by list: %#v %v", listed, err)
	}
	searched, err := r.Search(context.Background(), "bridge", "auto", 3, "first")
	if err != nil || searched["ok"] != true || vector.lastTool != "semantic_search" {
		t.Fatalf("dynamic source search failed: %#v %v", searched, err)
	}
	if vector.lastArgs["text"] != "bridge" || vector.lastArgs["context"] != "first" || vector.lastArgs["top_k"] != 3 {
		t.Fatalf("dynamic search binding drifted: %#v", vector.lastArgs)
	}
	status := r.Status(context.Background())
	if status["ok"] != true || status["degraded"] != false {
		t.Fatalf("dynamic source status failed: %#v", status)
	}
}
