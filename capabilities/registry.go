package capabilities

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

type HealthFunc func() map[string]any

type Module struct {
	ID           string
	Kind         string
	Version      string
	Dependencies []string
	Traits       []string
	Tools        []string
	Health       HealthFunc
}

type Registry struct {
	modules  []Module
	selected []string
}

func New(selected []string) *Registry {
	return &Registry{selected: append([]string(nil), selected...)}
}

func (r *Registry) Register(module Module) error {
	if module.ID == "" {
		return fmt.Errorf("registry_invalid: Bridge capability module ID cannot be empty")
	}
	if len(module.Tools) == 0 {
		return fmt.Errorf("registry_invalid: Bridge capability module must declare at least one public tool")
	}
	local := map[string]bool{}
	for _, tool := range module.Tools {
		if tool == "" {
			return fmt.Errorf("registry_invalid: Bridge public tool name cannot be empty")
		}
		if local[tool] {
			return fmt.Errorf("registry_collision: Bridge capability module declares a duplicate tool name")
		}
		local[tool] = true
	}
	for _, existing := range r.modules {
		if existing.ID == module.ID {
			return fmt.Errorf("registry_collision: Bridge capability module is duplicated")
		}
		owned := map[string]bool{}
		for _, tool := range existing.Tools {
			owned[tool] = true
		}
		for _, tool := range module.Tools {
			if owned[tool] {
				return fmt.Errorf("registry_collision: Bridge public tool name is duplicated by modules %q and %q", existing.ID, module.ID)
			}
		}
	}
	r.modules = append(r.modules, Module{
		ID: module.ID, Kind: module.Kind, Version: module.Version,
		Dependencies: append([]string(nil), module.Dependencies...),
		Traits:       append([]string(nil), module.Traits...),
		Tools:        append([]string(nil), module.Tools...), Health: module.Health,
	})
	return nil
}

func (r *Registry) ValidateSelection() error {
	known := map[string]bool{}
	for _, module := range r.modules {
		known[module.ID] = true
	}
	for _, id := range r.selected {
		if !known[id] {
			return fmt.Errorf("unknown_module: Unknown Bridge module selector: %s", id)
		}
	}
	for _, module := range r.modules {
		for _, dependency := range module.Dependencies {
			if !known[dependency] {
				return fmt.Errorf("registry_invalid: Bridge module %s references unknown dependency %s", module.ID, dependency)
			}
			if r.IsEnabled(module.ID) && !r.IsEnabled(dependency) {
				return fmt.Errorf("module_dependency_missing: %s requires %s", module.ID, dependency)
			}
		}
	}
	return nil
}

func (r *Registry) IsEnabled(moduleID string) bool {
	if len(r.selected) == 0 {
		for _, module := range r.modules {
			if module.ID == moduleID {
				return true
			}
		}
		return false
	}
	for _, id := range r.selected {
		if id == moduleID {
			return true
		}
	}
	return false
}

func (r *Registry) Enabled() []Module {
	out := []Module{}
	for _, module := range r.modules {
		if r.IsEnabled(module.ID) {
			out = append(out, module)
		}
	}
	return out
}

func (r *Registry) EnabledIDs() []string {
	out := []string{}
	for _, module := range r.Enabled() {
		out = append(out, module.ID)
	}
	return out
}

func (r *Registry) DisabledIDs() []string {
	out := []string{}
	for _, module := range r.modules {
		if !r.IsEnabled(module.ID) {
			out = append(out, module.ID)
		}
	}
	return out
}

func (r *Registry) ExpectedTools() []string {
	out := []string{}
	for _, module := range r.Enabled() {
		out = append(out, module.Tools...)
	}
	return out
}

func (r *Registry) Manifest() []map[string]any {
	out := make([]map[string]any, 0, len(r.modules))
	for _, module := range r.modules {
		out = append(out, map[string]any{
			"module": module.ID, "kind": module.Kind, "version": module.Version, "enabled": r.IsEnabled(module.ID),
			"dependencies": append([]string(nil), module.Dependencies...), "traits": append([]string(nil), module.Traits...),
			"tools": append([]string(nil), module.Tools...),
		})
	}
	return out
}

func (r *Registry) Health() []map[string]any {
	out := []map[string]any{}
	for _, module := range r.Enabled() {
		health := map[string]any{"ok": true}
		if module.Health != nil {
			func() {
				defer func() {
					if recover() != nil {
						health = map[string]any{"ok": false, "error": map[string]any{"code": "health_unavailable", "message": "Capability health check failed"}}
					}
				}()
				health = module.Health()
			}()
		}
		out = append(out, map[string]any{"module": module.ID, "version": module.Version, "health": health})
	}
	return out
}

func (r *Registry) SurfaceAudit(actual []string) map[string]any {
	expectedSet := map[string]bool{}
	for _, tool := range r.ExpectedTools() {
		expectedSet[tool] = true
	}
	actualSet := map[string]bool{}
	for _, tool := range actual {
		actualSet[tool] = true
	}
	missing := []string{}
	orphan := []string{}
	for tool := range expectedSet {
		if !actualSet[tool] {
			missing = append(missing, tool)
		}
	}
	for tool := range actualSet {
		if !expectedSet[tool] {
			orphan = append(orphan, tool)
		}
	}
	sort.Strings(missing)
	sort.Strings(orphan)
	sortedActual := make([]string, 0, len(actualSet))
	for tool := range actualSet {
		sortedActual = append(sortedActual, tool)
	}
	sort.Strings(sortedActual)
	sum := sha256.Sum256([]byte(strings.Join(sortedActual, "\n")))
	return map[string]any{
		"enabled_modules": r.EnabledIDs(), "disabled_modules": r.DisabledIDs(),
		"module_count": len(r.Enabled()), "tool_count": len(actualSet), "expected_tool_count": len(expectedSet),
		"missing_tools": missing, "orphan_tools": orphan, "consistent": len(missing) == 0 && len(orphan) == 0,
		"surface_fingerprint": fmt.Sprintf("%x", sum),
	}
}
