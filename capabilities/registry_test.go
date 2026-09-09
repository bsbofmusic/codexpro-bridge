package capabilities

import "testing"

func TestRegistryRejectsCollisionsAndUnknownSelection(t *testing.T) {
	r := New([]string{"alpha"})
	if err := r.Register(Module{ID: "alpha", Version: "1", Tools: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Module{ID: "beta", Version: "1", Tools: []string{"a"}}); err == nil {
		t.Fatal("expected cross-module tool collision")
	}
	if err := r.Register(Module{ID: "alpha", Version: "2", Tools: []string{"b"}}); err == nil {
		t.Fatal("expected duplicate module collision")
	}
	if err := r.ValidateSelection(); err != nil {
		t.Fatalf("known selection rejected: %v", err)
	}
	bad := New([]string{"missing"})
	if err := bad.Register(Module{ID: "alpha", Version: "1", Tools: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if err := bad.ValidateSelection(); err == nil {
		t.Fatal("expected unknown selector failure")
	}
}

func TestRegistryValidatesGenericDependencies(t *testing.T) {
	r := New([]string{"vision"})
	if err := r.Register(Module{ID: "core", Kind: "transport", Version: "1", Tools: []string{"core_tool"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Module{ID: "vision", Kind: "vision", Version: "1", Dependencies: []string{"core"}, Traits: []string{"invoke", "retrieve"}, Tools: []string{"vision_tool"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateSelection(); err == nil {
		t.Fatal("enabled adapter with disabled dependency should fail closed")
	}
	ok := New([]string{"core", "vision"})
	if err := ok.Register(Module{ID: "core", Kind: "transport", Version: "1", Tools: []string{"core_tool"}}); err != nil {
		t.Fatal(err)
	}
	if err := ok.Register(Module{ID: "vision", Kind: "vision", Version: "1", Dependencies: []string{"core"}, Traits: []string{"invoke", "retrieve"}, Tools: []string{"vision_tool"}}); err != nil {
		t.Fatal(err)
	}
	if err := ok.ValidateSelection(); err != nil {
		t.Fatalf("valid dependency selection rejected: %v", err)
	}
	manifest := ok.Manifest()
	if manifest[1]["kind"] != "vision" || len(manifest[1]["traits"].([]string)) != 2 {
		t.Fatalf("provider-neutral descriptor missing from manifest: %#v", manifest[1])
	}
}

func TestRegistrySurfaceAuditAndReducedModules(t *testing.T) {
	r := New([]string{"alpha"})
	if err := r.Register(Module{ID: "alpha", Version: "1", Tools: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Module{ID: "beta", Version: "1", Tools: []string{"c"}}); err != nil {
		t.Fatal(err)
	}
	audit := r.SurfaceAudit([]string{"a", "b"})
	if audit["consistent"] != true || audit["tool_count"] != 2 || audit["module_count"] != 1 {
		t.Fatalf("unexpected reduced surface: %#v", audit)
	}
	bad := r.SurfaceAudit([]string{"a", "orphan"})
	if bad["consistent"] != false {
		t.Fatalf("expected surface mismatch: %#v", bad)
	}
	if len(bad["missing_tools"].([]string)) != 1 || len(bad["orphan_tools"].([]string)) != 1 {
		t.Fatalf("missing/orphan audit failed: %#v", bad)
	}
}
