package server

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/codexpro/bridge/capabilities"
	"github.com/codexpro/bridge/contract"
	"github.com/codexpro/bridge/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testConfig(modules []string) core.Config {
	return core.Config{
		Host: "127.0.0.1", Port: 18788, Token: strings.Repeat("x", 32),
		SkillsRoot: "/tmp/skills", MCPURL: "http://127.0.0.1:19090/mcp",
		MCPTimeoutSeconds: 5, ObsidianMCPCommand: "/bin/true", MemosMCPCommand: "/bin/true",
		MemoryTimeoutSeconds: 5, WorkDBPath: "/tmp/work-test.sqlite3", MaxOutputChars: 120000,
		Modules: modules,
	}
}

func TestBuildDefaultSurfaceMatchesModuleUnion(t *testing.T) {
	_, caps, err := Build(testConfig(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{}
	for _, module := range contract.ModuleNames() {
		want = append(want, contract.ToolNames(module)...)
	}
	got := append([]string(nil), caps.ActualTools...)
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("default surface drifted\nwant=%v\ngot=%v", want, got)
	}
	audit := caps.Registry.SurfaceAudit(caps.ActualTools)
	if audit["consistent"] != true {
		t.Fatalf("default surface inconsistent: %#v", audit)
	}
}

func TestBuildReducedModuleSurface(t *testing.T) {
	_, caps, err := Build(testConfig([]string{"shared_skills", "work_runtime"}))
	if err != nil {
		t.Fatal(err)
	}
	want := append(contract.ToolNames("shared_skills"), contract.ToolNames("work_runtime")...)
	got := append([]string(nil), caps.ActualTools...)
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("reduced surface drifted\nwant=%v\ngot=%v", want, got)
	}
	if caps.Registry.IsEnabled("shared_mcp") || caps.Registry.IsEnabled("shared_memory") || caps.Registry.IsEnabled("bridge_doctor") {
		t.Fatal("disabled modules still reported enabled")
	}
}

func TestBuildUnknownModuleFailsClosed(t *testing.T) {
	if _, _, err := Build(testConfig([]string{"shared_skills", "does_not_exist"})); err == nil {
		t.Fatal("unknown module selector should fail closed")
	}
}

func TestAcceleratorModuleRequiresMCPAndWork(t *testing.T) {
	if _, _, err := Build(testConfig([]string{"web_accelerator"})); err == nil {
		t.Fatal("accelerator without dependencies should fail closed")
	}
	if _, _, err := Build(testConfig([]string{"web_accelerator", "shared_mcp", "work_runtime"})); err != nil {
		t.Fatalf("accelerator with explicit dependencies failed: %v", err)
	}
}

func TestRouteAndRecallGracefullyDegradesSkillsUnavailable(t *testing.T) {
	_, caps, err := Build(testConfig([]string{"shared_skills"}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := caps.routeAndRecall(context.Background(), "continue the bridge job", false, "", 8, 6)
	if err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["degraded"] != true {
		t.Fatalf("route_and_recall did not degrade safely: %#v", got)
	}
	routing := got["routing"].(map[string]any)
	if routing["ok"] != false || routing["degraded"] != true {
		t.Fatalf("skills degradation was not isolated: %#v", routing)
	}
}

func TestRouteAndRecallStillRejectsInvalidTask(t *testing.T) {
	_, caps, err := Build(testConfig([]string{"shared_skills"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := caps.routeAndRecall(context.Background(), "", false, "", 8, 6); err == nil {
		t.Fatal("invalid task must not be hidden as graceful degradation")
	}
}

func TestBuildWithVisionAdapterDoesNotRequireCoreRuntimeChanges(t *testing.T) {
	vision := capabilities.NewStaticAdapter(
		capabilities.Descriptor{
			ID: "mock_vision", Kind: "vision", Version: "1.0.0",
			Traits: []string{"invoke", "retrieve"},
		},
		[]mcp.Tool{{
			Name:        "mock_vision_describe",
			Description: "Test-only Vision adapter proving provider-neutral Bridge attachment.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"asset_ref": map[string]any{"type": "string"},
				},
				"required": []any{"asset_ref"},
			},
		}},
		map[string]capabilities.Operation{
			"mock_vision_describe": func(_ context.Context, args map[string]any) (map[string]any, error) {
				return map[string]any{"ok": true, "kind": "vision", "asset_ref": args["asset_ref"]}, nil
			},
		},
		func(context.Context) map[string]any { return map[string]any{"ok": true, "provider": "mock"} },
	)

	_, caps, err := BuildWithAdapters(testConfig(nil), vision)
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Registry.IsEnabled("mock_vision") {
		t.Fatal("Vision adapter was not enabled through the generic registry")
	}
	if len(caps.ActualTools) != len(contract.AllTools())+1 {
		t.Fatalf("unexpected public surface after Vision adapter: got=%d want=%d", len(caps.ActualTools), len(contract.AllTools())+1)
	}
	if _, ok := caps.ToolDefinitions["mock_vision_describe"]; !ok {
		t.Fatal("Vision tool definition was not registered generically")
	}
	operation := caps.Operations["mock_vision_describe"]
	if operation == nil {
		t.Fatal("Vision operation was not registered generically")
	}
	result, err := operation(context.Background(), map[string]any{"asset_ref": "asset://demo"})
	if err != nil || result["ok"] != true || result["kind"] != "vision" || result["asset_ref"] != "asset://demo" {
		t.Fatalf("generic Vision adapter invocation failed: %#v %v", result, err)
	}
	manifest := caps.Registry.Manifest()
	found := false
	for _, item := range manifest {
		if item["module"] == "mock_vision" {
			found = true
			if item["kind"] != "vision" || !reflect.DeepEqual(item["traits"], []string{"invoke", "retrieve"}) {
				t.Fatalf("Vision descriptor drifted: %#v", item)
			}
		}
	}
	if !found {
		t.Fatal("Vision adapter missing from provider-neutral manifest")
	}
	for _, module := range []string{"shared_skills", "shared_mcp", "shared_memory", "work_runtime", "web_accelerator", "bridge_doctor"} {
		if !caps.Registry.IsEnabled(module) {
			t.Fatalf("built-in module %s was displaced by Vision adapter", module)
		}
	}
	if audit := caps.Registry.SurfaceAudit(caps.ActualTools); audit["consistent"] != true {
		t.Fatalf("Vision adapter broke surface consistency: %#v", audit)
	}
}
