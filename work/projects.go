package work

import (
	"strings"

	"github.com/codexpro/bridge/core"
)

func (r *Runtime) ProjectCreate(name, description, workspaceRef string, relevantSkills, relevantMCP, memoryQueries, projectRules, knownRisks []string) (map[string]any, error) {
	if strings.TrimSpace(name) == "" {
		return nil, core.Err("invalid_work_input", "Project name is required")
	}
	lists := [][]string{relevantSkills, relevantMCP, memoryQueries, projectRules, knownRisks}
	encoded := make([]string, len(lists))
	for i, list := range lists {
		if list == nil {
			list = []string{}
		}
		var err error
		encoded[i], err = boundedJSON(list)
		if err != nil {
			return nil, err
		}
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	id := newID("prj")
	stamp := now()
	_, err = db.Exec(`INSERT INTO projects
(project_id,name,description,workspace_ref,relevant_skills,relevant_mcp,memory_queries,project_rules,known_risks,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?)`, id, strings.TrimSpace(name), description, sqlNullableString(workspaceRef), encoded[0], encoded[1], encoded[2], encoded[3], encoded[4], stamp, stamp)
	if err != nil {
		return nil, err
	}
	project, err := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "project": project}, nil
}

func (r *Runtime) ProjectGet(projectID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	project, err := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", projectID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "project": project}, nil
}

func (r *Runtime) ProjectList(limit, offset int) (map[string]any, error) {
	limit = clamp(limit, 1, 200)
	offset = maxInt(offset, 0)
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	projects, err := queryRows(db, "projects", "SELECT * FROM projects ORDER BY updated_at DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "projects": projects, "count": len(projects)}, nil
}

func (r *Runtime) ProjectRefresh(projectID string, changes map[string]any) (map[string]any, error) {
	allowed := setOf("name", "description", "workspace_ref", "relevant_skills", "relevant_mcp", "memory_queries", "project_rules", "known_risks")
	jsonFields := setOf("relevant_skills", "relevant_mcp", "memory_queries", "project_rules", "known_risks")
	if len(changes) == 0 {
		return r.ProjectGet(projectID)
	}
	sets := []string{}
	args := []any{}
	for key, value := range changes {
		if !allowed[key] {
			return nil, core.Err("invalid_work_input", "Unsupported project fields")
		}
		if jsonFields[key] {
			if value == nil {
				value = []any{}
			}
			encoded, err := boundedJSON(value)
			if err != nil {
				return nil, err
			}
			value = encoded
		}
		sets = append(sets, key+" = ?")
		args = append(args, value)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, now(), projectID)
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	res, err := db.Exec("UPDATE projects SET "+strings.Join(sets, ", ")+" WHERE project_id = ?", args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return nil, core.Err("work_not_found", "Project not found")
	}
	project, err := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", projectID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "project": project}, nil
}

func (r *Runtime) ProjectContext(projectID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	project, err := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", projectID)
	if err != nil {
		return nil, err
	}
	tasks, err := queryRows(db, "tasks", "SELECT * FROM tasks WHERE project_id = ? ORDER BY updated_at DESC LIMIT 50", projectID)
	if err != nil {
		return nil, err
	}
	latest, err := queryRows(db, "checkpoints", `SELECT c.* FROM checkpoints c JOIN tasks t ON t.task_id = c.task_id
WHERE t.project_id = ? ORDER BY c.created_at DESC LIMIT 1`, projectID)
	if err != nil {
		return nil, err
	}
	artifacts, err := queryRows(db, "artifacts", "SELECT * FROM artifacts WHERE project_id = ? ORDER BY updated_at DESC LIMIT 100", projectID)
	if err != nil {
		return nil, err
	}
	openTaskIDs := []any{}
	recentTaskIDs := []any{}
	for i, task := range tasks {
		if i < 20 {
			recentTaskIDs = append(recentTaskIDs, task["task_id"])
		}
		status := valueString(task["status"])
		if status == "pending" || status == "running" || status == "blocked" || status == "audit" {
			openTaskIDs = append(openTaskIDs, task["task_id"])
		}
	}
	artifactIDs := make([]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		artifactIDs = append(artifactIDs, artifact["artifact_id"])
	}
	var latestCheckpoint any
	if len(latest) > 0 {
		latestCheckpoint = latest[0]
	}
	return map[string]any{
		"ok": true, "project": project,
		"open_task_ids": openTaskIDs, "recent_task_ids": recentTaskIDs,
		"latest_checkpoint": latestCheckpoint, "artifact_ids": artifactIDs,
		"workspace_action": "Use the separate official CodexPro connection for workspace/files/Bash/Git/edits/tests.",
	}, nil
}
