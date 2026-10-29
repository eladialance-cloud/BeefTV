package manju

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type ProjectStateView struct {
	ID            string          `json:"id"`
	LockRevision  int64           `json:"lockRevision"`
	StageKey      string          `json:"stageKey"`
	SchemaVersion string          `json:"schemaVersion"`
	State         json.RawMessage `json:"state"`
	ContentHash   string          `json:"contentHash"`
	CreatedBy     string          `json:"createdBy"`
	CreatedAt     string          `json:"createdAt"`
}

type RuleLibraryStatus struct {
	Version         string            `json:"version"`
	Embedded        bool              `json:"embedded"`
	FileCount       int               `json:"fileCount"`
	ContentHash     string            `json:"contentHash"`
	Files           []string          `json:"files"`
	OfficialScripts []string          `json:"officialScripts"`
	ProjectState    *ProjectStateView `json:"projectState,omitempty"`
	RuleSnapshot    *RuleSnapshotView `json:"ruleSnapshot,omitempty"`
}

type RuleSnapshotView struct {
	ID           string `json:"id"`
	LockRevision int64  `json:"lockRevision"`
	RulesVersion string `json:"rulesVersion"`
	SourceCommit string `json:"sourceCommit"`
	MasterPath   string `json:"masterPath"`
	MasterSHA256 string `json:"masterSha256"`
	TreeSHA256   string `json:"treeSha256"`
	ManifestJSON string `json:"manifestJson"`
	CreatedAt    string `json:"createdAt"`
}

func (s *Service) RuleLibraryStatus(actorID, projectID string) (*RuleLibraryStatus, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(access.Role) == "" {
		return nil, ErrForbidden
	}
	files, err := defaultManjuSkillRuntime.EmbeddedFiles()
	if err != nil {
		return nil, err
	}
	contentHash, err := defaultManjuSkillRuntime.EmbeddedRuleHash()
	if err != nil {
		return nil, err
	}
	officialScripts := make([]string, 0)
	for _, path := range files {
		if strings.Contains(path, "/scripts/") && strings.HasSuffix(path, ".py") {
			officialScripts = append(officialScripts, path)
		}
	}
	status := &RuleLibraryStatus{
		Version:         manjuSkillVersionLabel(),
		Embedded:        true,
		FileCount:       len(files),
		ContentHash:     contentHash,
		Files:           files,
		OfficialScripts: officialScripts,
	}
	receipt, err := s.repo.LatestManjuLockReceipt(projectID)
	if err != nil {
		return nil, err
	}
	if receipt == nil {
		return status, nil
	}
	ruleSnapshot, err := s.repo.ManjuRuleSnapshot(projectID, receipt.Revision)
	if err != nil {
		return nil, err
	}
	if ruleSnapshot != nil {
		status.RuleSnapshot = &RuleSnapshotView{
			ID:           ruleSnapshot.ID,
			LockRevision: ruleSnapshot.LockRevision,
			RulesVersion: ruleSnapshot.RulesVersion,
			SourceCommit: ruleSnapshot.SourceCommit,
			MasterPath:   ruleSnapshot.MasterPath,
			MasterSHA256: ruleSnapshot.MasterSHA256,
			TreeSHA256:   ruleSnapshot.TreeSHA256,
			ManifestJSON: ruleSnapshot.ManifestJSON,
			CreatedAt:    ruleSnapshot.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}
	snapshot, err := s.repo.LatestManjuProjectState(projectID, receipt.Revision)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return status, nil
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(snapshot.StateJSON), &state); err != nil {
		return nil, ErrInvalidProjectState
	}
	status.ProjectState = &ProjectStateView{
		ID:            snapshot.ID,
		LockRevision:  snapshot.LockRevision,
		StageKey:      snapshot.StageKey,
		SchemaVersion: snapshot.SchemaVersion,
		State:         json.RawMessage(snapshot.StateJSON),
		ContentHash:   snapshot.ContentHash,
		CreatedBy:     snapshot.CreatedBy,
		CreatedAt:     snapshot.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	return status, nil
}
