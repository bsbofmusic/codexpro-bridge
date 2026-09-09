package work

import (
	"fmt"
	"strings"

	"github.com/codexpro/bridge/core"
)

func (r *Runtime) ArtifactRegister(name, pathOrRef, artifactType, taskID, projectID, sha256 string, size *int, verificationStatus, verificationEvidenceRef string, metadata map[string]any) (map[string]any, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(pathOrRef) == "" {
		return nil, core.Err("invalid_work_input", "Artifact name and path/reference are required")
	}
	if artifactType == "" {
		artifactType = "other"
	}
	if verificationStatus == "" {
		verificationStatus = "declared"
	}
	if !verificationStates[verificationStatus] {
		return nil, core.Err("invalid_work_input", "Unknown artifact verification status")
	}
	if verificationStatus == "verified" && verificationEvidenceRef == "" {
		return nil, core.Err("invalid_work_input", "Verified artifact requires verification evidence reference")
	}
	meta, err := boundedJSON(defaultMap(metadata))
	if err != nil {
		return nil, err
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if taskID != "" {
		task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
		if err != nil {
			return nil, err
		}
		if projectID == "" {
			projectID = valueString(task["project_id"])
		}
	}
	if projectID != "" {
		if _, err := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", projectID); err != nil {
			return nil, err
		}
	}
	id := newID("art")
	stamp := now()
	var sizeValue any
	if size != nil {
		sizeValue = *size
	}
	_, err = db.Exec(`INSERT INTO artifacts
(artifact_id,task_id,project_id,name,type,path_or_ref,status,sha256,size,verification_status,verification_evidence_ref,created_at,updated_at,metadata)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, nullableText(taskID), nullableText(projectID), strings.TrimSpace(name), artifactType, pathOrRef, "draft", nullableText(sha256), sizeValue, verificationStatus, nullableText(verificationEvidenceRef), stamp, stamp, meta)
	if err != nil {
		return nil, err
	}
	if taskID != "" {
		if err := touchTaskRevision(db, taskID); err != nil {
			return nil, err
		}
	}
	artifact, err := one(db, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "artifact": artifact}, nil
}

func (r *Runtime) ArtifactGet(artifactID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	artifact, err := one(db, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", artifactID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "artifact": artifact}, nil
}

func (r *Runtime) ArtifactList(taskID, projectID, status string, limit, offset int) (map[string]any, error) {
	if status != "" && !artifactStates[status] {
		return nil, core.Err("invalid_work_input", "Unknown artifact status")
	}
	clauses := []string{}
	args := []any{}
	if taskID != "" {
		clauses = append(clauses, "task_id = ?")
		args = append(args, taskID)
	}
	if projectID != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, projectID)
	}
	if status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, status)
	}
	query := "SELECT * FROM artifacts"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, clamp(limit, 1, 300), maxInt(offset, 0))
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := queryRows(db, "artifacts", query, args...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "artifacts": rows, "count": len(rows)}, nil
}

func (r *Runtime) artifactState(artifactID, target string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	artifact, err := one(db, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", artifactID)
	if err != nil {
		return nil, err
	}
	current := fmt.Sprint(artifact["status"])
	legal := (current == "draft" && (target == "final" || target == "archived")) || (current == "final" && target == "archived")
	if !legal {
		return nil, core.Err("invalid_artifact_transition", fmt.Sprintf("Artifact cannot transition from %s to %s", current, target))
	}
	if _, err := db.Exec("UPDATE artifacts SET status = ?, updated_at = ? WHERE artifact_id = ?", target, now(), artifactID); err != nil {
		return nil, err
	}
	if taskID := valueString(artifact["task_id"]); taskID != "" {
		if err := touchTaskRevision(db, taskID); err != nil {
			return nil, err
		}
		if target == "final" {
			if _, err := checkpointCreateDB(db, taskID, nil, []any{map[string]any{"event": "artifact.finalized", "artifact_id": artifactID}}, "", []string{artifactID}, "", "artifact.finalized", ""); err != nil {
				return nil, err
			}
		}
	}
	artifact, err = one(db, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", artifactID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "artifact": artifact}, nil
}

func (r *Runtime) ArtifactFinalize(artifactID string) (map[string]any, error) {
	return r.artifactState(artifactID, "final")
}

func (r *Runtime) ArtifactArchive(artifactID string) (map[string]any, error) {
	return r.artifactState(artifactID, "archived")
}
