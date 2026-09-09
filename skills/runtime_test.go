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
