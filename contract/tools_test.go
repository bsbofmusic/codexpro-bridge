package contract

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/codexpro/bridge/accelerator"
	"github.com/codexpro/bridge/work"
)

func TestEmbeddedPythonContractRoundTrips(t *testing.T) {
	var want []map[string]any
	if err := json.Unmarshal(CanonicalJSON(), &want); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(LegacyTools())
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	sort.Slice(want, func(i, j int) bool { return want[i]["name"].(string) < want[j]["name"].(string) })
	sort.Slice(got, func(i, j int) bool { return got[i]["name"].(string) < got[j]["name"].(string) })
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("Go public tool model drifted from captured Python contract\nwant=%#v\ngot=%#v", want, got)
	}
}

func TestLegacyPythonContractRemainsSeparateFromGoExtensions(t *testing.T) {
	legacy := LegacyTools()
	all := AllTools()
	if len(all) != len(legacy)+1 {
		t.Fatalf("unexpected extension count: legacy=%d all=%d", len(legacy), len(all))
	}
	tool, ok := Tool("codexpro_bridge_accelerator")
	if !ok || tool.Name == "" {
		t.Fatal("accelerator extension tool missing")
	}
	for _, item := range legacy {
		if item.Name == tool.Name {
			t.Fatal("accelerator leaked into immutable Python compatibility oracle")
		}
	}
}

func TestModuleContractCoversEveryToolExactlyOnce(t *testing.T) {
	seen := map[string]string{}
	for _, module := range ModuleNames() {
		for _, name := range ToolNames(module) {
			if owner, exists := seen[name]; exists {
				t.Fatalf("tool %s owned by both %s and %s", name, owner, module)
			}
			seen[name] = module
		}
	}
	if len(seen) != len(AllTools()) {
		t.Fatalf("module union=%d contract=%d", len(seen), len(AllTools()))
	}
	for _, tool := range AllTools() {
		if seen[tool.Name] == "" {
			t.Fatalf("unowned tool: %s", tool.Name)
		}
	}
}

func TestAcceleratorOperationCatalogMatchesPublicEnum(t *testing.T) {
	tool, ok := Tool("codexpro_bridge_accelerator")
	if !ok {
		t.Fatal("accelerator tool missing")
	}
	schema := tool.InputSchema.(map[string]any)
	properties := schema["properties"].(map[string]any)
	operation := properties["operation"].(map[string]any)
	rawEnum := operation["enum"].([]any)
	got := make([]string, 0, len(rawEnum))
	for _, item := range rawEnum {
		got = append(got, item.(string))
	}
	if !reflect.DeepEqual(got, accelerator.Operations) {
		t.Fatalf("accelerator operation catalog drifted\nwant=%#v\ngot=%#v", accelerator.Operations, got)
	}
}

func TestWorkOperationCatalogMatchesCapturedPythonEnum(t *testing.T) {
	tool, ok := Tool("codexpro_bridge_work")
	if !ok {
		t.Fatal("work tool missing from contract")
	}
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("unexpected work input schema: %#v", tool.InputSchema)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("work schema properties missing: %#v", schema)
	}
	operation, ok := properties["operation"].(map[string]any)
	if !ok {
		t.Fatalf("work operation schema missing: %#v", properties)
	}
	rawEnum, ok := operation["enum"].([]any)
	if !ok {
		t.Fatalf("work operation enum missing: %#v", operation)
	}
	got := make([]string, 0, len(rawEnum))
	for _, item := range rawEnum {
		got = append(got, item.(string))
	}
	if !reflect.DeepEqual(got, work.WorkOperations) {
		t.Fatalf("Work operation catalog drifted from Python contract\nwant=%#v\ngot=%#v", got, work.WorkOperations)
	}
}
