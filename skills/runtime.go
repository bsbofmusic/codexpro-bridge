package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/codexpro/bridge/core"
	"gopkg.in/yaml.v3"
)

var resourceRoots = map[string]bool{"references": true, "templates": true, "scripts": true, "assets": true, "agents": true, "prompts": true, "evals": true}
var sensitiveComponent = regexp.MustCompile(`(?i)(^|[_-])(env|secret|secrets|credential|credentials|password|passwd|private[_-]?key|ssh)($|[_-])`)
var tokenRE = regexp.MustCompile(`(?i)[a-z0-9][a-z0-9_-]{1,}|[\p{Han}]{2,}`)

var sensitiveSuffix = map[string]bool{".pem": true, ".key": true, ".p12": true, ".pfx": true, ".kdbx": true}

type Runtime struct {
	Root string
	CLI  string
}

func New(root string) *Runtime {
	cli := strings.TrimSpace(os.Getenv("SKILLS_MANAGER_CLI"))
	if cli == "" {
		cli = "/home/agent/.local/bin/skills-manager-cli"
	}
	return &Runtime{Root: root, CLI: cli}
}

type managerRecord struct {
	SkillID string `json:"id"`
	Enabled *bool  `json:"enabled"`
	Path    string `json:"path"`
}

func safeUnder(root, rel string) (string, error) {
	rel = strings.ReplaceAll(rel, `\`, `/`)
	if rel == "" || filepath.IsAbs(rel) {
		return "", core.Err("skills_unavailable", "Skills Manager metadata contains an invalid path")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", core.Err("skills_unavailable", "Skills Manager metadata contains an invalid path")
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", core.Err("skills_unavailable", "Skills Manager central library is unavailable")
	}
	candidate, err := filepath.Abs(filepath.Join(base, clean))
	if err != nil {
		return "", core.Err("skills_unavailable", "Skills Manager central library is unavailable")
	}
	relToBase, err := filepath.Rel(base, candidate)
	if err != nil || relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(filepath.Separator)) {
		return "", core.Err("skills_unavailable", "Skill path escapes the shared Skill root")
	}
	return candidate, nil
}

func hashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

func frontmatter(text string) map[string]any {
	if !strings.HasPrefix(text, "---") {
		return map[string]any{}
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]any{}
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func stringList(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			out = append(out, toString(item))
		}
		return out
	default:
		return []string{}
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func (r *Runtime) records() ([]map[string]any, error) {
	if stat, err := os.Stat(r.Root); err != nil || !stat.IsDir() {
		return nil, core.Err("skills_unavailable", "Skills Manager central library is unavailable")
	}
	cli := strings.TrimSpace(r.CLI)
	if cli == "" {
		return nil, core.Err("skills_unavailable", "Skills Manager CLI is unavailable")
	}
	if stat, err := os.Stat(cli); err != nil || stat.IsDir() {
		return nil, core.Err("skills_unavailable", "Skills Manager CLI is unavailable")
	}
	managerRoot := filepath.Dir(r.Root)
	cmd := exec.Command(cli, "skills", "list", "--json")
	cmd.Dir = managerRoot
	b, err := cmd.Output()
	if err != nil {
		return nil, core.Err("skills_unavailable", "Skills Manager metadata is unreadable")
	}
	var managed []managerRecord
	if json.Unmarshal(b, &managed) != nil {
		return nil, core.Err("skills_unavailable", "Skills Manager metadata is unreadable")
	}
	records := []map[string]any{}
	rootAbs, err := filepath.Abs(r.Root)
	if err != nil {
		return nil, core.Err("skills_unavailable", "Skills Manager central library is unavailable")
	}
	for _, raw := range managed {
		if raw.Enabled != nil && !*raw.Enabled {
			continue
		}
		managedPath := strings.TrimSpace(raw.Path)
		if managedPath == "" {
			continue
		}
		pathAbs, err := filepath.Abs(managedPath)
		if err != nil {
			return nil, core.Err("skills_unavailable", "Skills Manager metadata contains an invalid path")
		}
		rel, err := filepath.Rel(rootAbs, pathAbs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, core.Err("skills_unavailable", "Skill path escapes the shared Skill root")
		}
		dir, err := safeUnder(r.Root, filepath.ToSlash(rel))
		if err != nil {
			return nil, err
		}
		skillFile := filepath.Join(dir, "SKILL.md")
		textBytes, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}
		text := string(textBytes)
		meta := frontmatter(text)
		name := strings.TrimSpace(toString(meta["name"]))
		if name == "" {
			name = filepath.Base(dir)
		}
		if name == "" {
			continue
		}
		hash, err := hashFile(skillFile)
		if err != nil {
			return nil, core.Err("skills_unavailable", "A managed SKILL.md is unreadable")
		}
		rel, _ = filepath.Rel(rootAbs, dir)
		item := map[string]any{
			"id": raw.SkillID, "name": name, "description": toString(meta["description"]),
			"tags": stringList(meta["tags"]), "path": filepath.ToSlash(rel), "sha256": hash,
			"_file": skillFile, "_dir": dir,
		}
		if item["id"] == "" {
			item["id"] = name
		}
		if v, ok := meta["version"]; ok && v != nil {
			item["version"] = v
		}
		if v, ok := meta["category"]; ok && v != nil {
			item["category"] = v
		}
		records = append(records, item)
	}
	sort.Slice(records, func(i, j int) bool {
		return strings.ToLower(toString(records[i]["name"])) < strings.ToLower(toString(records[j]["name"]))
	})
	return records, nil
}

func publicRecord(record map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range record {
		if !strings.HasPrefix(key, "_") && value != nil {
			out[key] = value
		}
	}
	return out
}

func (r *Runtime) List(category string) (map[string]any, error) {
	if len(category) > 128 {
		return nil, core.Err("invalid_category", "Skill category is invalid")
	}
	records, err := r.records()
	if err != nil {
		return nil, err
	}
	skills := []map[string]any{}
	categorySet := map[string]bool{}
	for _, record := range records {
		if category != "" && toString(record["category"]) != category {
			continue
		}
		pub := publicRecord(record)
		skills = append(skills, pub)
		if c := toString(record["category"]); c != "" {
			categorySet[c] = true
		}
	}
	categories := make([]string, 0, len(categorySet))
	for c := range categorySet {
		categories = append(categories, c)
	}
	sort.Strings(categories)
	return map[string]any{"ok": true, "skills": skills, "categories": categories, "count": len(skills), "source": "skills-manager"}, nil
}

func (r *Runtime) SkillsList(category, query string, offset, limit int) (map[string]any, error) {
	if offset < 0 {
		return nil, core.Err("invalid_pagination", "offset must be a non-negative integer")
	}
	if limit < 1 || limit > 100 {
		return nil, core.Err("invalid_pagination", "limit must be between 1 and 100")
	}
	if len(query) > 256 {
		return nil, core.Err("invalid_query", "Skill query is invalid")
	}
	catalog, err := r.List(category)
	if err != nil {
		return nil, err
	}
	skills := catalog["skills"].([]map[string]any)
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle != "" {
		filtered := []map[string]any{}
		for _, item := range skills {
			hay := strings.ToLower(toString(item["name"]) + " " + toString(item["description"]) + " " + toString(item["category"]) + " " + strings.Join(stringList(item["tags"]), " "))
			if strings.Contains(hay, needle) {
				filtered = append(filtered, item)
			}
		}
		skills = filtered
	}
	end := offset + limit
	if end > len(skills) {
		end = len(skills)
	}
	page := []map[string]any{}
	if offset < len(skills) {
		page = skills[offset:end]
	}
	var next any
	if end < len(skills) {
		next = end
	}
	return map[string]any{"ok": true, "skills": page, "count": len(skills), "offset": offset, "limit": limit, "next_offset": next, "categories": catalog["categories"], "source": catalog["source"]}, nil
}

func (r *Runtime) record(name string) (map[string]any, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 || filepath.IsAbs(name) || strings.Contains(name, "/") || strings.Contains(name, `\`) || name == ".." {
		return nil, core.Err("invalid_skill", "Skill name must be a managed Skill name")
	}
	records, err := r.records()
	if err != nil {
		return nil, err
	}
	for _, item := range records {
		if toString(item["name"]) == name {
			return item, nil
		}
	}
	return nil, core.Err("skill_unavailable", "Skill is not enabled in Skills Manager")
}

func (r *Runtime) Load(name string) (map[string]any, error) {
	record, err := r.record(name)
	if err != nil {
		return nil, err
	}
	path := toString(record["_file"])
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, core.Err("skills_unavailable", "A managed SKILL.md is unreadable")
	}
	skill := publicRecord(record)
	skill["text"] = string(b)
	skill["bytes"] = len(b)
	return map[string]any{"ok": true, "skill": skill}, nil
}

func validateResourcePath(resourcePath string) (string, error) {
	raw := strings.TrimSpace(strings.ReplaceAll(resourcePath, `\`, `/`))
	if raw == "" || len(raw) > 512 || strings.HasPrefix(raw, "/") {
		return "", core.Err("invalid_resource", "Skill resource path is invalid")
	}
	parts := strings.Split(raw, "/")
	if len(parts) == 0 || !resourceRoots[parts[0]] {
		return "", core.Err("invalid_resource", "Resource must stay below an allowed Skill resource directory")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") || sensitiveComponent.MatchString(strings.ToLower(part)) {
			return "", core.Err("sensitive_resource", "Sensitive Skill resources cannot be returned")
		}
	}
	if sensitiveSuffix[strings.ToLower(filepath.Ext(raw))] {
		return "", core.Err("sensitive_resource", "Sensitive Skill resources cannot be returned")
	}
	return raw, nil
}

func (r *Runtime) Resource(name, resourcePath string) (map[string]any, error) {
	record, err := r.record(name)
	if err != nil {
		return nil, err
	}
	safe, err := validateResourcePath(resourcePath)
	if err != nil {
		return nil, err
	}
	root := toString(record["_dir"])
	target, err := safeUnder(root, safe)
	if err != nil {
		return nil, core.Err("resource_unavailable", "Requested Skill resource is unavailable")
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return nil, core.Err("resource_unavailable", "Requested Skill resource is not readable text")
	}
	hash := sha256.Sum256(b)
	return map[string]any{"ok": true, "resource": map[string]any{"skill": record["name"], "path": safe, "text": string(b), "bytes": len(b), "sha256": hex.EncodeToString(hash[:])}}, nil
}

func (r *Runtime) Route(task string, limit int) (map[string]any, error) {
	task = strings.TrimSpace(task)
	if task == "" || len(task) > 8_000 {
		return nil, core.Err("invalid_task", "task must be a non-empty string up to 8000 characters")
	}
	if limit < 1 || limit > 20 {
		return nil, core.Err("invalid_limit", "route limit must be between 1 and 20")
	}
	catalog, err := r.List("")
	if err != nil {
		return nil, err
	}
	skills := catalog["skills"].([]map[string]any)
	terms := map[string]bool{}
	for _, token := range tokenRE.FindAllString(task, -1) {
		t := strings.ToLower(token)
		if len([]rune(t)) >= 2 {
			terms[t] = true
		}
	}
	taskFolded := strings.ToLower(task)
	type ranked struct {
		score int
		name  string
		item  map[string]any
	}
	rows := []ranked{}
	for _, item := range skills {
		name := toString(item["name"])
		description := toString(item["description"])
		tags := strings.Join(stringList(item["tags"]), " ")
		hay := strings.ToLower(name + " " + description + " " + tags)
		nameFolded := strings.ToLower(name)
		score := 0
		if name != "" && strings.Contains(taskFolded, nameFolded) {
			score += 100
		}
		for term := range terms {
			if strings.Contains(nameFolded, term) {
				score += 12
			} else if strings.Contains(hay, term) {
				score += 2
			}
		}
		if score > 0 {
			rows = append(rows, ranked{score: score, name: nameFolded, item: item})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].name < rows[j].name
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	matches := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		matches = append(matches, row.item)
	}
	return map[string]any{"ok": true, "task": task, "routing": "Choose a matching managed Skill, then load its SKILL.md before acting.", "skills": matches, "match_count": len(matches), "catalog_count": catalog["count"], "source": catalog["source"]}, nil
}
