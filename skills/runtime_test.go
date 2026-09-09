package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFakeSkillsManagerCLI(t *testing.T, records any) string {
	t.Helper()
	payload, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "skills-manager-cli")
	script := "#!/bin/sh\ncat <<'JSON'\n" + string(payload) + "\nJSON\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRecordsUseCanonicalCLIWithoutNestedShadowMetadata(t *testing.T) {
	managerRoot := t.TempDir()
	root := filepath.Join(managerRoot, "skills")
	skillDir := filepath.Join(root, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: demo skill\nversion: 1.2.3\n---\n\n# Demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cli := writeFakeSkillsManagerCLI(t, []map[string]any{{
		"id": "skill-1", "name": "demo", "path": skillDir, "enabled": true,
	}})
	r := &Runtime{Root: root, CLI: cli}

	got, err := r.SkillsList("", "", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got["count"] != 1 {
		t.Fatalf("count=%v want 1", got["count"])
	}
	if _, err := os.Stat(filepath.Join(root, ".skills-manager")); !os.IsNotExist(err) {
		t.Fatalf("nested shadow metadata should not be required")
	}
}

func TestRecordsRejectManagedPathOutsideSharedRoot(t *testing.T) {
	managerRoot := t.TempDir()
	root := filepath.Join(managerRoot, "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	cli := writeFakeSkillsManagerCLI(t, []map[string]any{{
		"id": "skill-1", "name": "escape", "path": outside, "enabled": true,
	}})
	r := &Runtime{Root: root, CLI: cli}

	if _, err := r.List(""); err == nil {
		t.Fatal("expected out-of-root managed path to fail closed")
	}
}

func TestRouteHonorsFrontmatterTriggersInsideNaturalChineseText(t *testing.T) {
	managerRoot := t.TempDir()
	root := filepath.Join(managerRoot, "skills")
	lifeDir := filepath.Join(root, "life-experience")
	otherDir := filepath.Join(root, "last30days")
	for _, dir := range []string{lifeDir, otherDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	lifeSkill := "---\nname: life-experience\ndescription: Retrieve lived-experience cases.\ntriggers:\n  - 人生经验\n  - 人生导师\nversion: 1.0.0\n---\n\n# Life Experience\n"
	otherSkill := "---\nname: last30days\ndescription: Research recent community discussion and what people say.\nversion: 1.0.0\n---\n\n# Last 30 Days\n"
	if err := os.WriteFile(filepath.Join(lifeDir, "SKILL.md"), []byte(lifeSkill), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "SKILL.md"), []byte(otherSkill), 0o644); err != nil {
		t.Fatal(err)
	}
	cli := writeFakeSkillsManagerCLI(t, []map[string]any{
		{"id": "skill-life", "name": "life-experience", "path": lifeDir, "enabled": true},
		{"id": "skill-other", "name": "last30days", "path": otherDir, "enabled": true},
	})
	r := &Runtime{Root: root, CLI: cli}

	got, err := r.Route("帮我找一些30多岁转行的人生经验，尤其是后来后悔没后悔、几年后过得怎么样", 10)
	if err != nil {
		t.Fatal(err)
	}
	skills := got["skills"].([]map[string]any)
	if len(skills) == 0 || skills[0]["name"] != "life-experience" {
		t.Fatalf("first routed skill=%v want life-experience", skills)
	}
	if _, exposed := skills[0]["_triggers"]; exposed {
		t.Fatal("private trigger metadata must not be exposed in public route output")
	}
}
