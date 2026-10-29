package manju

import (
	"crypto/sha256"
	"fmt"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/google/uuid"
)

func buildManjuRuleSnapshot(projectID string, lockRevision int64, createdBy string, manifest *OfficialRuleTreeManifest) (*model.ManjuRuleSnapshot, error) {
	if manifest == nil {
		return nil, fmt.Errorf("官方规则清单不能为空")
	}
	manifestJSON, err := manifest.JSON()
	if err != nil {
		return nil, fmt.Errorf("序列化官方规则清单失败: %w", err)
	}
	return &model.ManjuRuleSnapshot{
		ID:           uuid.NewString(),
		ProjectID:    projectID,
		LockRevision: lockRevision,
		RulesVersion: manifest.Version,
		SourceCommit: manifest.SourceCommit,
		MasterPath:   manifest.MasterPath,
		MasterSHA256: manifest.MasterSHA256,
		TreeSHA256:   manifest.TreeSHA256,
		ManifestJSON: manifestJSON,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now(),
	}, nil
}

func buildManjuRuleReadReceipts(snapshot *model.ManjuRuleSnapshot, runID, stageID, stageKey, requestID string, files []OfficialRuleFile) []model.ManjuRuleReadReceipt {
	if snapshot == nil {
		return nil
	}
	now := time.Now()
	receipts := make([]model.ManjuRuleReadReceipt, 0, len(files))
	for _, file := range files {
		if !file.Exact {
			continue
		}
		receipts = append(receipts, model.ManjuRuleReadReceipt{
			ID:            uuid.NewString(),
			ProjectID:     snapshot.ProjectID,
			RunID:         runID,
			StageID:       stageID,
			SnapshotID:    snapshot.ID,
			StageKey:      stageKey,
			RulePath:      file.Path,
			StartByte:     0,
			EndByte:       int64(len([]byte(file.Content))),
			ContentSHA256: file.SHA256,
			RequestID:     requestID,
			ToolName:      "official_master_dispatch",
			CreatedAt:     now,
		})
	}
	return receipts
}

func buildManjuModelExchange(projectID, runID, stageID, stageKey, requestID, requestJSON, requestSHA256 string) (*model.ManjuModelExchange, error) {
	if requestJSON == "" {
		return nil, fmt.Errorf("模型请求正文不能为空")
	}
	if requestSHA256 == "" {
		requestSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(requestJSON)))
	}
	return &model.ManjuModelExchange{
		ID:            uuid.NewString(),
		ProjectID:     projectID,
		RunID:         runID,
		StageID:       stageID,
		StageKey:      stageKey,
		RequestID:     requestID,
		RequestJSON:   requestJSON,
		RequestSHA256: requestSHA256,
		Status:        model.ManjuModelExchangeStatusQueued,
		CreatedAt:     time.Now(),
	}, nil
}

func manjuArtifactVisibility(phase, stage string) string {
	switch phase {
	case "P2":
		if stage == AssetStageCompile {
			return model.ManjuArtifactVisibilityUser
		}
	case "P3":
		if stage == ProductionStageStoryboard {
			return model.ManjuArtifactVisibilityUser
		}
	case "P4":
		if stage == ProductionStageFeed {
			return model.ManjuArtifactVisibilityUser
		}
	}
	return model.ManjuArtifactVisibilityInternal
}

func (s *Service) ensureManjuRuleSnapshot(tx *repository.Repository, projectID string, lockRevision int64, createdBy string) (*model.ManjuRuleSnapshot, error) {
	existing, err := tx.ManjuRuleSnapshot(projectID, lockRevision)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	manifest, err := defaultManjuSkillRuntime.OfficialRuleTreeManifest()
	if err != nil {
		return nil, err
	}
	snapshot, err := buildManjuRuleSnapshot(projectID, lockRevision, createdBy, manifest)
	if err != nil {
		return nil, err
	}
	if err := tx.UpsertManjuRuleSnapshot(snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *Service) recordManjuRuleReads(
	tx *repository.Repository,
	snapshot *model.ManjuRuleSnapshot,
	runID, stageID, stageKey, requestID string,
	bundle *OfficialRuleBundle,
) error {
	if snapshot == nil || bundle == nil {
		return nil
	}
	existing, err := tx.ManjuRuleReadReceiptsForRequest(requestID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	return tx.CreateManjuRuleReadReceipts(
		buildManjuRuleReadReceipts(snapshot, runID, stageID, stageKey, requestID, bundle.Files),
	)
}

func recordManjuArtifactAudit(
	tx *repository.Repository,
	projectID, runID, stageID, artifactID, phase, kind string,
	version int64,
	visibility, content, contentHash, taskID, createdBy string,
) error {
	if taskID != "" {
		if err := tx.CompleteManjuModelExchangeByTaskID(
			taskID, content, contentHash, model.ManjuModelExchangeStatusSucceeded, "",
		); err != nil {
			return err
		}
	}
	return tx.CreateManjuArtifactLedger(&model.ManjuArtifactLedger{
		ProjectID:     projectID,
		RunID:         runID,
		StageID:       stageID,
		ArtifactID:    artifactID,
		Phase:         phase,
		Kind:          kind,
		Version:       version,
		Visibility:    visibility,
		ContentSHA256: contentHash,
		CreatedBy:     createdBy,
	})
}
