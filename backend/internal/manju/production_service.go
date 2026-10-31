package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ProductionStageDecisionAnchor = model.ManjuProductionStageDecisionAnchor
	ProductionStageStoryboard     = model.ManjuProductionStageStoryboard
	ProductionStageFeed           = model.ManjuProductionStageFeed
	ProductionStageQC             = model.ManjuProductionStageQC

	ProductionStageStatusNotStarted           = model.ManjuProductionStageStatusNotStarted
	ProductionStageStatusInProgress           = model.ManjuProductionStageStatusInProgress
	ProductionStageStatusAwaitingConfirmation = model.ManjuProductionStageStatusAwaitingConfirmation
	ProductionStageStatusLocked               = model.ManjuProductionStageStatusLocked
	ProductionStageStatusBlocked              = model.ManjuProductionStageStatusBlocked
	ProductionStageStatusStale                = model.ManjuProductionStageStatusStale
)

var (
	ErrProductionRunNotFound              = errors.New("manju production run not found")
	ErrProductionStageNotFound            = errors.New("manju production stage not found")
	ErrProductionAssetNotReady            = errors.New("manju P2 asset run is not completed and locked")
	ErrProductionStageNotReady            = errors.New("manju production stage is not ready")
	ErrInvalidProductionRun               = errors.New("invalid manju production run")
	ErrProductionArtifactInvalid          = errors.New("invalid manju production artifact")
	ErrProductionPrevisRequired           = errors.New("manju P4.5 previs storyboard is required")
	ErrProductionMachineCheckProofInvalid = errors.New("manju P4 machine check proof is invalid")
)

const (
	ManjuProductionMachineCheckProofVersion = "V7.4.26"
	ManjuProductionMachineCheckPipeline     = "compile_feed.py -> validate_prompt.py -> gate_check.py -> script_feed_diff.py"
	ManjuProductionMachineCheckCompiler     = "V7.4.26 compile_feed.py"
	ManjuProductionMachineCheckValidator    = "V7.4.26 validate_prompt.py"
	ManjuProductionMachineCheckGate         = "V7.4.26 gate_check.py"
	ManjuProductionMachineCheckDialogueDiff = "V7.4.26 script_feed_diff.py"
	manjuProductionLegacyMachineCheckNote   = "历史非合规状态：未通过 V7.4.26 机检，不能作为当前锁定依据。"
)

type ManjuProductionMachineCheckProof struct {
	Version                    string   `json:"version"`
	Pipeline                   string   `json:"pipeline"`
	Compiler                   string   `json:"compiler"`
	Validator                  string   `json:"validator"`
	Gate                       string   `json:"gate"`
	DialogueDiff               string   `json:"dialogueDiff"`
	DialogueBaselineArtifactID string   `json:"dialogueBaselineArtifactId"`
	Status                     string   `json:"status"`
	Warnings                   []string `json:"warnings"`
}

type productionStageRuntimeMetadata struct {
	MachineCheckProof *ManjuProductionMachineCheckProof `json:"machineCheckProof,omitempty"`
	MachineCheckNote  string                            `json:"machineCheckNote,omitempty"`
}

type ProductionRunInput struct {
	AssetRunID string `json:"assetRunId"`
}

type ProductionStageRunInput struct {
	Instruction string `json:"instruction"`
}

type ProductionConfirmInput struct {
	Note string `json:"note,omitempty"`
}

type ProductionRunView struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	AssetRunID      string    `json:"assetRunId"`
	Status          string    `json:"status"`
	CurrentStage    string    `json:"currentStage"`
	LockRevision    int64     `json:"lockRevision"`
	HasDeliverables bool      `json:"hasDeliverables"`
	CreatedBy       string    `json:"createdBy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type ProductionStageView struct {
	ID                        string               `json:"id"`
	Key                       string               `json:"key"`
	Title                     string               `json:"title"`
	Position                  int                  `json:"position"`
	Status                    string               `json:"status"`
	Version                   int64                `json:"version"`
	TaskID                    string               `json:"taskId,omitempty"`
	ArtifactID                string               `json:"artifactId,omitempty"`
	Error                     string               `json:"error,omitempty"`
	ConfirmedBy               string               `json:"confirmedBy,omitempty"`
	ConfirmedAt               *time.Time           `json:"confirmedAt,omitempty"`
	StartedAt                 *time.Time           `json:"startedAt,omitempty"`
	CompletedAt               *time.Time           `json:"completedAt,omitempty"`
	UpdatedAt                 time.Time            `json:"updatedAt"`
	Draft                     *ManjuStageDraftView `json:"draft,omitempty"`
	MachineCheckStatus        string               `json:"machineCheckStatus,omitempty"`
	MachineCheckNote          string               `json:"machineCheckNote,omitempty"`
	MachineCheckWarnings      []string             `json:"machineCheckWarnings,omitempty"`
	MachineCheckProofVerified bool                 `json:"machineCheckProofVerified,omitempty"`
}

type ProductionArtifactView struct {
	ID          string    `json:"id"`
	StageID     string    `json:"stageId"`
	Kind        string    `json:"kind"`
	Version     int64     `json:"version"`
	Content     string    `json:"content"`
	ContentHash string    `json:"contentHash"`
	TaskID      string    `json:"taskId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ProductionPrevisView struct {
	ID         string    `json:"id"`
	RunID      string    `json:"runId"`
	TaskID     string    `json:"taskId,omitempty"`
	ArtifactID string    `json:"artifactId,omitempty"`
	Status     string    `json:"status"`
	Version    int64     `json:"version"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ProductionRunDetail struct {
	Run       ProductionRunView        `json:"run"`
	AssetRun  AssetRunView             `json:"assetRun"`
	ScriptRun ScriptRunView            `json:"scriptRun"`
	Stages    []ProductionStageView    `json:"stages"`
	Artifacts []ProductionArtifactView `json:"artifacts,omitempty"`
	Previs    *ProductionPrevisView    `json:"previs,omitempty"`
	Images    []ProductionImageView    `json:"images,omitempty"`
	Videos    []ProductionVideoView    `json:"videos,omitempty"`
}

type productionStageDefinition struct {
	Key   string
	Title string
}

func productionStageDefinitions() []productionStageDefinition {
	return []productionStageDefinition{
		{Key: ProductionStageDecisionAnchor, Title: "P0B 决策锚点"},
		{Key: ProductionStageStoryboard, Title: "P3 分镜与故事板"},
		{Key: ProductionStageFeed, Title: "P4 投喂封装"},
		{Key: ProductionStageQC, Title: "P5 质检与交付"},
	}
}

func repairLegacyProductionDecisionStage(tx *repository.Repository, run *model.ManjuProductionRun) error {
	if run == nil || run.Status != model.ManjuProductionRunStatusActive {
		return nil
	}
	stages, err := tx.ManjuProductionStages(run.ID)
	if err != nil {
		return err
	}
	if productionStageIndex(stages, ProductionStageDecisionAnchor) >= 0 {
		return nil
	}
	now := time.Now()
	storyboardIndex := productionStageIndex(stages, ProductionStageStoryboard)
	if err := tx.DB().Model(&model.ManjuProductionStage{}).
		Where("run_id = ? AND position >= 1", run.ID).
		UpdateColumn("position", gorm.Expr("position + 1")).Error; err != nil {
		return err
	}
	decision := model.ManjuProductionStage{
		ID:        uuid.NewString(),
		RunID:     run.ID,
		Key:       ProductionStageDecisionAnchor,
		Title:     "P0B 决策锚点",
		Position:  1,
		Status:    model.ManjuProductionStageStatusInProgress,
		StartedAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.CreateManjuProductionStages([]model.ManjuProductionStage{decision}); err != nil {
		return err
	}
	if storyboardIndex >= 0 {
		storyboard := &stages[storyboardIndex]
		if strings.TrimSpace(storyboard.ArtifactID) != "" ||
			storyboard.Status == model.ManjuProductionStageStatusLocked ||
			storyboard.Status == model.ManjuProductionStageStatusAwaitingConfirmation {
			if err := tx.DB().Model(&model.ManjuProductionStage{}).
				Where("id = ?", storyboard.ID).
				Updates(map[string]any{
					"status":       model.ManjuProductionStageStatusStale,
					"task_id":      "",
					"started_at":   nil,
					"completed_at": nil,
					"confirmed_by": "",
					"confirmed_at": nil,
					"input_json":   "",
					"error":        "旧 P3 创建时没有 P0B 决策锚点，必须重新生成 P3；旧工件与媒体仅保留为历史",
				}).Error; err != nil {
				return err
			}
		} else if storyboard.Status != model.ManjuProductionStageStatusLocked {
			if err := tx.DB().Model(&model.ManjuProductionStage{}).
				Where("id = ?", storyboard.ID).
				Updates(map[string]any{
					"status":       model.ManjuProductionStageStatusNotStarted,
					"task_id":      "",
					"started_at":   nil,
					"completed_at": nil,
					"artifact_id":  "",
					"confirmed_by": "",
					"confirmed_at": nil,
					"input_json":   "",
					"error":        "",
				}).Error; err != nil {
				return err
			}
		}
		for index := range stages {
			if stages[index].Position <= storyboard.Position {
				continue
			}
			if err := tx.DB().Model(&model.ManjuProductionStage{}).
				Where("id = ?", stages[index].ID).
				Updates(map[string]any{
					"status":       model.ManjuProductionStageStatusNotStarted,
					"task_id":      "",
					"artifact_id":  "",
					"error":        "旧 P3 缺少 P0B 决策链，等待重新生成的 P3",
					"confirmed_by": "",
					"confirmed_at": nil,
					"completed_at": nil,
					"input_json":   "",
				}).Error; err != nil {
				return err
			}
		}
		previs, err := tx.ManjuProductionPrevis(run.ProjectID, run.ID)
		if err != nil {
			return err
		}
		if previs != nil {
			previs.Status = model.ManjuProductionPrevisStatusFailed
			previs.Error = "旧 P3 缺少 P0B 决策链，请重新生成 P3 后再生成 P4.5"
			if err := tx.SaveManjuProductionPrevis(previs); err != nil {
				return err
			}
		}
	}
	run.CurrentStage = ProductionStageDecisionAnchor
	run.UpdatedAt = now
	return tx.SaveManjuProductionRun(run)
}

func (s *Service) CreateProductionRun(actorID, projectID string, input ProductionRunInput) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	input.AssetRunID = strings.TrimSpace(input.AssetRunID)
	if input.AssetRunID == "" {
		return nil, ErrInvalidProductionRun
	}

	var runID string
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked {
			return ErrP0Required
		}
		assetRun, err := tx.ManjuAssetRun(projectID, input.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil {
			return ErrProductionAssetNotReady
		}
		if assetRun.Status != model.ManjuAssetRunStatusCompleted || assetRun.LockRevision != receipt.Revision {
			return ErrProductionAssetNotReady
		}
		compileStage, err := tx.ManjuAssetStage(assetRun.ID, AssetStageCompile)
		if err != nil {
			return err
		}
		if compileStage == nil ||
			compileStage.Status != model.ManjuAssetStageStatusLocked ||
			strings.TrimSpace(compileStage.ArtifactID) == "" {
			return ErrProductionAssetNotReady
		}
		activeRuns, err := tx.ManjuProductionRuns(projectID)
		if err != nil {
			return err
		}
		for _, existing := range activeRuns {
			if existing.Status == model.ManjuProductionRunStatusActive {
				return ErrInvalidProductionRun
			}
		}

		now := time.Now()
		run := &model.ManjuProductionRun{
			ID:           uuid.NewString(),
			ProjectID:    projectID,
			AssetRunID:   assetRun.ID,
			Status:       model.ManjuProductionRunStatusActive,
			CurrentStage: ProductionStageDecisionAnchor,
			LockRevision: receipt.Revision,
			CreatedBy:    actorID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.CreateManjuProductionRun(run); err != nil {
			return err
		}
		stages := make([]model.ManjuProductionStage, 0, len(productionStageDefinitions()))
		for index, definition := range productionStageDefinitions() {
			status := model.ManjuProductionStageStatusNotStarted
			var startedAt *time.Time
			if index == 0 {
				status = model.ManjuProductionStageStatusInProgress
				startedAt = &now
			}
			stages = append(stages, model.ManjuProductionStage{
				ID:        uuid.NewString(),
				RunID:     run.ID,
				Key:       definition.Key,
				Title:     definition.Title,
				Position:  index + 1,
				Status:    status,
				StartedAt: startedAt,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		if err := tx.CreateManjuProductionStages(stages); err != nil {
			return err
		}
		runID = run.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

// RegenerateProductionStage creates a new P3/P4 production run version. Every
// successor carries the locked P0B decision artifact; P4 also carries the locked
// P3 artifact and ready storyboard images. The superseded run and all of its
// artifacts, images, and videos remain readable as history.
func (s *Service) RegenerateProductionStage(actorID, projectID, runID, stageKey string) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	if stageKey != ProductionStageStoryboard && stageKey != ProductionStageFeed {
		return nil, ErrProductionStageNotFound
	}

	var newRunID string
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		if run.Status != model.ManjuProductionRunStatusActive &&
			run.Status != model.ManjuProductionRunStatusCompleted {
			return ErrProductionStageNotReady
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil ||
			assetRun.Status != model.ManjuAssetRunStatusCompleted ||
			assetRun.LockRevision != run.LockRevision {
			return ErrProductionAssetNotReady
		}
		stages, err := tx.ManjuProductionStages(runID)
		if err != nil {
			return err
		}
		targetIndex := productionStageIndex(stages, stageKey)
		if targetIndex < 0 {
			return ErrProductionStageNotFound
		}
		target := stages[targetIndex]
		if target.Status != model.ManjuProductionStageStatusLocked &&
			target.Status != model.ManjuProductionStageStatusAwaitingConfirmation &&
			target.Status != model.ManjuProductionStageStatusStale {
			return ErrProductionStageNotReady
		}
		if strings.TrimSpace(target.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		if err := ensureNoLiveProductionWork(tx, actorID, runID, stages); err != nil {
			return err
		}

		artifacts, err := tx.ManjuProductionArtifacts(runID)
		if err != nil {
			return err
		}
		decisionIndex := productionStageIndex(stages, ProductionStageDecisionAnchor)
		if decisionIndex < 0 {
			return ErrProductionStageNotReady
		}
		sourceDecision := &stages[decisionIndex]
		if sourceDecision.Status != model.ManjuProductionStageStatusLocked ||
			strings.TrimSpace(sourceDecision.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		sourceDecisionArtifact := productionArtifactByID(artifacts, sourceDecision.ArtifactID)
		if sourceDecisionArtifact == nil {
			return ErrProductionStageNotReady
		}

		var sourceP3 *model.ManjuProductionStage
		var sourceP3Artifact *model.ManjuProductionArtifact
		var sourceImages []model.ManjuProductionImage
		if stageKey == ProductionStageFeed {
			p3Index := productionStageIndex(stages, ProductionStageStoryboard)
			if p3Index < 0 {
				return ErrProductionStageNotFound
			}
			sourceP3 = &stages[p3Index]
			if sourceP3.Status != model.ManjuProductionStageStatusLocked ||
				strings.TrimSpace(sourceP3.ArtifactID) == "" {
				return ErrProductionStageNotReady
			}
			sourceP3Artifact = productionArtifactByID(artifacts, sourceP3.ArtifactID)
			if sourceP3Artifact == nil {
				return ErrProductionStageNotReady
			}
			sourceImages, err = tx.ManjuProductionImages(runID)
			if err != nil {
				return err
			}
		}

		now := time.Now()
		newRun := &model.ManjuProductionRun{
			ID:           uuid.NewString(),
			ProjectID:    projectID,
			AssetRunID:   run.AssetRunID,
			Status:       model.ManjuProductionRunStatusActive,
			CurrentStage: stageKey,
			LockRevision: run.LockRevision,
			CreatedBy:    actorID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.CreateManjuProductionRun(newRun); err != nil {
			return err
		}

		newStages := make([]model.ManjuProductionStage, 0, len(productionStageDefinitions()))
		for index, definition := range productionStageDefinitions() {
			stage := model.ManjuProductionStage{
				ID:        uuid.NewString(),
				RunID:     newRun.ID,
				Key:       definition.Key,
				Title:     definition.Title,
				Position:  index + 1,
				Status:    model.ManjuProductionStageStatusNotStarted,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if definition.Key == stageKey {
				stage.Status = model.ManjuProductionStageStatusInProgress
				stage.StartedAt = &now
			}
			if definition.Key == ProductionStageDecisionAnchor {
				stage.Status = model.ManjuProductionStageStatusLocked
				stage.Version = sourceDecision.Version
				stage.InputJSON = sourceDecision.InputJSON
				stage.ConfirmedBy = sourceDecision.ConfirmedBy
				stage.ConfirmedAt = sourceDecision.ConfirmedAt
				stage.StartedAt = sourceDecision.StartedAt
				stage.CompletedAt = sourceDecision.CompletedAt
			}
			if stageKey == ProductionStageFeed && definition.Key == ProductionStageStoryboard {
				stage.Status = model.ManjuProductionStageStatusLocked
				stage.Version = sourceP3.Version
				stage.InputJSON = sourceP3.InputJSON
				stage.ConfirmedBy = sourceP3.ConfirmedBy
				stage.ConfirmedAt = sourceP3.ConfirmedAt
				stage.StartedAt = sourceP3.StartedAt
				stage.CompletedAt = sourceP3.CompletedAt
			}
			newStages = append(newStages, stage)
		}
		if err := tx.CreateManjuProductionStages(newStages); err != nil {
			return err
		}

		decisionNewIndex := productionStageIndex(newStages, ProductionStageDecisionAnchor)
		decisionArtifactCreator := sourceDecisionArtifact.CreatedBy
		if strings.TrimSpace(decisionArtifactCreator) == "" {
			decisionArtifactCreator = actorID
		}
		decisionArtifact := &model.ManjuProductionArtifact{
			ID:          uuid.NewString(),
			ProjectID:   projectID,
			RunID:       newRun.ID,
			StageID:     newStages[decisionNewIndex].ID,
			Kind:        sourceDecisionArtifact.Kind,
			Version:     sourceDecisionArtifact.Version,
			Content:     sourceDecisionArtifact.Content,
			ContentHash: sourceDecisionArtifact.ContentHash,
			TaskID:      sourceDecisionArtifact.TaskID,
			CreatedBy:   decisionArtifactCreator,
			CreatedAt:   now,
		}
		if err := tx.CreateManjuProductionArtifact(decisionArtifact); err != nil {
			return err
		}
		if err := recordManjuArtifactAudit(
			tx, projectID, newRun.ID, newStages[decisionNewIndex].ID,
			decisionArtifact.ID, "P0B", decisionArtifact.Kind,
			decisionArtifact.Version,
			manjuArtifactVisibility("P0B", newStages[decisionNewIndex].Key),
			decisionArtifact.Content, decisionArtifact.ContentHash,
			decisionArtifact.TaskID, decisionArtifactCreator,
		); err != nil {
			return err
		}
		newStages[decisionNewIndex].ArtifactID = decisionArtifact.ID
		if err := tx.SaveManjuProductionStage(&newStages[decisionNewIndex]); err != nil {
			return err
		}

		if stageKey == ProductionStageFeed {
			storyboardNewIndex := productionStageIndex(newStages, ProductionStageStoryboard)
			sourceCreator := sourceP3Artifact.CreatedBy
			if strings.TrimSpace(sourceCreator) == "" {
				sourceCreator = actorID
			}
			artifact := &model.ManjuProductionArtifact{
				ID:          uuid.NewString(),
				ProjectID:   projectID,
				RunID:       newRun.ID,
				StageID:     newStages[storyboardNewIndex].ID,
				Kind:        sourceP3Artifact.Kind,
				Version:     sourceP3Artifact.Version,
				Content:     sourceP3Artifact.Content,
				ContentHash: sourceP3Artifact.ContentHash,
				TaskID:      sourceP3Artifact.TaskID,
				CreatedBy:   sourceCreator,
				CreatedAt:   now,
			}
			if err := tx.CreateManjuProductionArtifact(artifact); err != nil {
				return err
			}
			if err := recordManjuArtifactAudit(
				tx, projectID, newRun.ID, newStages[storyboardNewIndex].ID, artifact.ID, "P3", artifact.Kind,
				artifact.Version, manjuArtifactVisibility("P3", newStages[storyboardNewIndex].Key),
				artifact.Content, artifact.ContentHash, artifact.TaskID, sourceCreator,
			); err != nil {
				return err
			}
			newStages[storyboardNewIndex].ArtifactID = artifact.ID
			if err := tx.SaveManjuProductionStage(&newStages[storyboardNewIndex]); err != nil {
				return err
			}

			clonedImages := make([]model.ManjuProductionImage, 0, len(sourceImages))
			for _, image := range sourceImages {
				if image.Status != ProductionImageStatusReady || strings.TrimSpace(image.ResourceID) == "" {
					continue
				}
				createdBy := image.CreatedBy
				if strings.TrimSpace(createdBy) == "" {
					createdBy = actorID
				}
				clonedImages = append(clonedImages, model.ManjuProductionImage{
					ID:          uuid.NewString(),
					ProjectID:   projectID,
					RunID:       newRun.ID,
					StageID:     newStages[storyboardNewIndex].ID,
					ArtifactID:  artifact.ID,
					Position:    image.Position,
					SegmentCode: image.SegmentCode,
					ShotLabel:   image.ShotLabel,
					Prompt:      image.Prompt,
					Attempt:     image.Attempt,
					Status:      ProductionImageStatusReady,
					TaskID:      image.TaskID,
					ResourceID:  image.ResourceID,
					URL:         image.URL,
					CreatedBy:   createdBy,
					CreatedAt:   now,
					UpdatedAt:   now,
				})
			}
			if err := tx.CreateManjuProductionImages(clonedImages); err != nil {
				return err
			}
		}

		if err := tx.ActivateManjuProductionRun(newRun); err != nil {
			return err
		}
		newRunID = newRun.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.ProductionRunDetail(actorID, projectID, newRunID)
}

// RegenerateProductionArtifact resets one production artifact inside the
// existing run. Only downstream stages are invalidated; locked upstream
// artifacts and historical media remain intact.
func (s *Service) RegenerateProductionArtifact(actorID, projectID, runID, stageKey string, input ProductionStageRunInput) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Instruction = strings.TrimSpace(input.Instruction)
	if stageKey != ProductionStageDecisionAnchor &&
		stageKey != ProductionStageStoryboard &&
		stageKey != ProductionStageFeed &&
		stageKey != ProductionStageQC {
		return nil, ErrProductionStageNotFound
	}

	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		if run.Status != model.ManjuProductionRunStatusActive &&
			run.Status != model.ManjuProductionRunStatusCompleted {
			return ErrProductionStageNotReady
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		stages, err := tx.ManjuProductionStages(runID)
		if err != nil {
			return err
		}
		targetIndex := productionStageIndex(stages, stageKey)
		if targetIndex < 0 {
			return ErrProductionStageNotFound
		}
		target := &stages[targetIndex]
		if strings.TrimSpace(target.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		switch target.Status {
		case model.ManjuProductionStageStatusLocked,
			model.ManjuProductionStageStatusAwaitingConfirmation,
			model.ManjuProductionStageStatusBlocked,
			model.ManjuProductionStageStatusStale:
		default:
			return ErrProductionStageNotReady
		}
		if stageKey != ProductionStageDecisionAnchor {
			if err := ensureProductionStageGate(stages, stageKey); err != nil {
				return err
			}
		}
		if err := ensureNoLiveProductionWork(tx, actorID, runID, stages); err != nil {
			return err
		}
		if stageKey == ProductionStageStoryboard {
			if err := staleP3CanvasImportForRun(tx, projectID, runID); err != nil {
				return err
			}
		}

		target.Status = model.ManjuProductionStageStatusInProgress
		target.TaskID = ""
		target.ArtifactID = ""
		target.Error = ""
		target.ConfirmedBy = ""
		target.ConfirmedAt = nil
		target.CompletedAt = nil
		target.InputJSON = ""
		if err := tx.SaveManjuProductionStage(target); err != nil {
			return err
		}
		for index := targetIndex + 1; index < len(stages); index++ {
			downstream := &stages[index]
			downstream.Status = model.ManjuProductionStageStatusNotStarted
			downstream.TaskID = ""
			downstream.ArtifactID = ""
			downstream.Error = "上游工件已重新生成，等待新工件"
			downstream.ConfirmedBy = ""
			downstream.ConfirmedAt = nil
			downstream.CompletedAt = nil
			downstream.InputJSON = ""
			if err := tx.SaveManjuProductionStage(downstream); err != nil {
				return err
			}
		}
		if stageKey == ProductionStageStoryboard || stageKey == ProductionStageFeed {
			previs, err := tx.ManjuProductionPrevis(projectID, runID)
			if err != nil {
				return err
			}
			if previs != nil {
				previs.Status = model.ManjuProductionPrevisStatusFailed
				previs.Error = "上游 P3/P4 工件已重新生成，请重新生成 P4.5"
				if err := tx.SaveManjuProductionPrevis(previs); err != nil {
					return err
				}
			}
		}
		run.CurrentStage = stageKey
		if err := tx.ActivateManjuProductionRun(run); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.RunProductionStage(actorID, projectID, runID, stageKey, input)
}

func ensureNoLiveProductionWork(tx *repository.Repository, actorID, runID string, stages []model.ManjuProductionStage) error {
	for _, stage := range stages {
		if strings.TrimSpace(stage.TaskID) == "" {
			continue
		}
		task, err := tx.TaskForUser(actorID, stage.TaskID)
		if errors.Is(err, gorm.ErrRecordNotFound) || task == nil {
			continue
		}
		if err != nil {
			return err
		}
		if task.Status == model.TaskStatusQueued || task.Status == model.TaskStatusRunning {
			return ErrProductionStageNotReady
		}
	}
	images, err := tx.ManjuProductionImages(runID)
	if err != nil {
		return err
	}
	for _, image := range images {
		if image.Status == ProductionImageStatusQueued || image.Status == ProductionImageStatusRunning {
			return ErrProductionStageNotReady
		}
	}
	videos, err := tx.ManjuProductionVideos(runID)
	if err != nil {
		return err
	}
	for _, video := range videos {
		if video.Status == ProductionVideoStatusQueued || video.Status == ProductionVideoStatusRunning {
			return ErrProductionStageNotReady
		}
	}
	return nil
}

func (s *Service) ProductionRuns(actorID, projectID string) ([]ProductionRunView, error) {
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
	runs, err := s.repo.ManjuProductionRuns(projectID)
	if err != nil {
		return nil, err
	}
	deliverables, err := s.repo.ManjuDeliverablesForProject(projectID)
	if err != nil {
		return nil, err
	}
	hasDeliverables := make(map[string]bool, len(deliverables))
	for _, deliverable := range deliverables {
		hasDeliverables[deliverable.ProductionRunID] = true
	}
	items := make([]ProductionRunView, 0, len(runs))
	for _, run := range runs {
		view := productionRunView(run)
		view.HasDeliverables = hasDeliverables[run.ID]
		items = append(items, view)
	}
	return items, nil
}

func (s *Service) runOfficialProductionFeedCompiler(actorID, projectID, runID string, _ ProductionStageRunInput) (*ProductionRunDetail, error) {
	var stateJSON string
	var upstreamScript string
	var upstreamArtifactID string
	var validationOptions ManjuSkillValidationOptions
	alreadyReady := false
	var preflightErr error
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		if run.Status != model.ManjuProductionRunStatusActive || run.CurrentStage != ProductionStageFeed {
			return ErrProductionStageNotReady
		}
		stage, err := tx.ManjuProductionStage(runID, ProductionStageFeed)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrProductionStageNotFound
		}
		if stage.Status == model.ManjuProductionStageStatusAwaitingConfirmation && strings.TrimSpace(stage.ArtifactID) != "" {
			alreadyReady = true
			return nil
		}
		stages, err := tx.ManjuProductionStages(runID)
		if err != nil {
			return err
		}
		if err := ensureProductionStageGate(stages, ProductionStageFeed); err != nil {
			return err
		}
		p3Stage := stages[productionStageIndex(stages, ProductionStageStoryboard)]
		if p3Stage.Status != model.ManjuProductionStageStatusLocked || strings.TrimSpace(p3Stage.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		upstreamScript, err = productionArtifactContent(tx, runID, p3Stage.ArtifactID)
		if err != nil || strings.TrimSpace(upstreamScript) == "" {
			return ErrProductionStageNotReady
		}
		upstreamArtifactID = p3Stage.ArtifactID
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil || assetRun == nil {
			return ErrProductionAssetNotReady
		}
		if _, err := ensureManjuProjectState(tx, projectID, access.Project.Name, run.LockRevision, actorID); err != nil {
			return err
		}
		state, snapshot, err := manjuProjectStateMapForArtifact(
			tx,
			projectID,
			run.LockRevision,
			"production:director_storyboard",
			upstreamArtifactID,
		)
		if err != nil {
			return err
		}
		if snapshot == nil {
			return fmt.Errorf(
				"%w: 当前锁定 P3 工件 %s 没有对应的 project_state 快照",
				ErrProjectStateUnavailable,
				upstreamArtifactID,
			)
		}
		if p3OfficialSegmentHeading.MatchString(upstreamScript) {
			derived, err := compileP3OfficialStoryboardState(upstreamScript, state)
			if err != nil {
				return err
			}
			oldSegments, _ := json.Marshal(state["segments"])
			newSegments, _ := json.Marshal(derived["segments"])
			if string(oldSegments) != string(newSegments) {
				state["segments"] = derived["segments"]
				if err := persistManjuProjectState(
					tx, projectID, run.LockRevision,
					"production:director_storyboard", upstreamArtifactID,
					state, actorID,
				); err != nil {
					return err
				}
			}
		}
		view, err := receiptView(*receipt)
		if err != nil {
			return err
		}
		if err := validateP3AssetRefsForP4(state, view.Snapshot.VideoModel); err != nil {
			message := "P3_CONTRACT_UPGRADE_REQUIRED: 当前锁定的 P3 工件不符合分段资产引用契约；请只重新生成 P3 当前工件，不要重新生成 P2，也不要让 P4 自动补写或过滤资产: " + err.Error()
			if blockErr := blockProductionStageRecord(tx, stage, message); blockErr != nil {
				return blockErr
			}
			preflightErr = errors.New(message)
			return nil
		}
		normalizedState, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化 V7.4.26 project_state.json: %w", err)
		}
		stateJSON = string(normalizedState)
		validationOptions = ManjuSkillValidationOptions{
			Model: view.Snapshot.VideoModel, Track: manjuSkillTrackCode(view.Snapshot.Track), Combat: detectManjuCombat(string(mustJSON(state))),
			Mode: assetRun.AssetMode, Aspect: view.Snapshot.AspectRatio, StyleSource: view.Snapshot.StyleCardName,
			StylePromptParameters: view.Snapshot.StylePromptParameters, StyleVisualPrompt: view.Snapshot.StyleVisualPrompt,
			StyleMotionParameters: view.Snapshot.StyleMotionParameters, StyleForbiddenItems: view.Snapshot.StyleForbiddenItems,
			Assets: 99, Timeout: 120 * time.Second, UpstreamScript: upstreamScript,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if alreadyReady {
		return s.ProductionRunDetail(actorID, projectID, runID)
	}
	if preflightErr != nil {
		return nil, preflightErr
	}
	feed, validation, compileErr := defaultManjuSkillRuntime.CompileFeed(stateJSON, validationOptions)
	if compileErr != nil {
		message := "V7.4.26 官方 compile_feed.py 编译失败"
		if strings.TrimSpace(validation.Output) != "" {
			message += ": " + truncateScriptText(validation.Output, 6000)
		} else if strings.TrimSpace(compileErr.Error()) != "" {
			message += ": " + truncateScriptText(compileErr.Error(), 6000)
		}
		s.blockProductionStage(projectID, runID, ProductionStageFeed, message)
		return nil, errors.New(message)
	}
	if !validation.Passed {
		message := "V7.4.26 官方 P4 编译、机检或台词完整性检查未通过"
		if len(validation.Issues) > 0 {
			message += ": " + truncateScriptText(strings.Join(validation.Issues, "\n"), 6000)
		} else if strings.TrimSpace(validation.Output) != "" {
			message += ": " + truncateScriptText(validation.Output, 6000)
		}
		if route := p4RepairRouteInstruction(validation.Issues...); route != "" {
			message += "；" + route
		}
		s.blockProductionStage(projectID, runID, ProductionStageFeed, message)
		return nil, errors.New(message)
	}
	if strings.TrimSpace(feed) == "" {
		message := "V7.4.26 官方 compile_feed.py 返回空投喂稿"
		s.blockProductionStage(projectID, runID, ProductionStageFeed, message)
		return nil, errors.New(message)
	}

	err = s.repo.Transaction(func(tx *repository.Repository) error {
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil || run.Status != model.ManjuProductionRunStatusActive || run.CurrentStage != ProductionStageFeed {
			return ErrProductionStageNotReady
		}
		stage, err := tx.ManjuProductionStage(runID, ProductionStageFeed)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrProductionStageNotFound
		}
		if strings.TrimSpace(stage.ArtifactID) != "" {
			return nil
		}
		now := time.Now()
		stage.Version++
		sum := sha256.Sum256([]byte(feed))
		artifact := &model.ManjuProductionArtifact{
			ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
			Kind: model.ManjuProductionArtifactFeed, Version: stage.Version,
			Content: feed, ContentHash: hex.EncodeToString(sum[:]), CreatedBy: actorID, CreatedAt: now,
		}
		if err := tx.CreateManjuProductionArtifact(artifact); err != nil {
			return err
		}
		if err := recordManjuArtifactAudit(
			tx, projectID, runID, stage.ID, artifact.ID, "P4", artifact.Kind,
			artifact.Version, manjuArtifactVisibility("P4", stage.Key),
			artifact.Content, artifact.ContentHash, artifact.TaskID, actorID,
		); err != nil {
			return err
		}
		stage.ArtifactID = artifact.ID
		stage.CompletedAt = &now
		stage.Status = model.ManjuProductionStageStatusAwaitingConfirmation
		stage.TaskID = ""
		stage.Error = ""
		allWarnings := productionMachineCheckWarnings(validation)
		machineCheckNote := "V7.4.26 compile_feed.py、validate_prompt.py、gate_check.py 与 script_feed_diff.py 已通过。"
		if len(allWarnings) > 0 {
			machineCheckNote = fmt.Sprintf(
				"V7.4.26 P4 硬门已通过，但仍有 %d 条官方规则或台词完整性警告；必须人工核对后才可视为严格合规。",
				len(allWarnings),
			)
		}
		proof := &ManjuProductionMachineCheckProof{
			Version:                    ManjuProductionMachineCheckProofVersion,
			Pipeline:                   ManjuProductionMachineCheckPipeline,
			Compiler:                   ManjuProductionMachineCheckCompiler,
			Validator:                  ManjuProductionMachineCheckValidator,
			Gate:                       ManjuProductionMachineCheckGate,
			DialogueDiff:               ManjuProductionMachineCheckDialogueDiff,
			DialogueBaselineArtifactID: upstreamArtifactID,
			Status:                     model.ManjuProductionMachineCheckStatusPass,
			Warnings:                   allWarnings,
		}
		runtimeMeta, err := json.Marshal(productionStageRuntimeMetadata{
			MachineCheckProof: proof,
			MachineCheckNote:  machineCheckNote,
		})
		if err != nil {
			return err
		}
		stage.InputJSON = string(runtimeMeta)
		if err := tx.SaveManjuProductionStage(stage); err != nil {
			return err
		}
		run.CurrentStage = ProductionStageFeed
		return tx.SaveManjuProductionRun(run)
	})
	if err != nil {
		return nil, err
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

func productionMachineCheckWarnings(validation ManjuSkillValidationResult) []string {
	result := make([]string, 0, len(validation.Warnings)+len(validation.DialogueWarnings))
	seen := make(map[string]bool, cap(result))
	for _, warning := range append(append([]string{}, validation.Warnings...), validation.DialogueWarnings...) {
		warning = strings.TrimSpace(warning)
		if warning == "" || seen[warning] {
			continue
		}
		seen[warning] = true
		result = append(result, warning)
	}
	return result
}

func (s *Service) GenerateProductionPrevis(actorID, projectID, runID string) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	previsContext, err := defaultManjuSkillRuntime.PrevisContext()
	if err != nil {
		return nil, err
	}

	var request localtask.CreateRequest
	var existingTaskID string
	var previsAttemptVersion int64
	var operationKey string
	alreadyReady := false
	err = s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil || run.Status != model.ManjuProductionRunStatusActive || run.CurrentStage != ProductionStageFeed {
			return ErrProductionStageNotReady
		}
		stage, err := tx.ManjuProductionStage(runID, ProductionStageFeed)
		if err != nil {
			return err
		}
		if stage == nil || strings.TrimSpace(stage.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		existing, err := tx.ManjuProductionPrevis(projectID, runID)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.Status == model.ManjuProductionPrevisStatusReady && strings.TrimSpace(existing.ArtifactID) != "" {
				alreadyReady = true
				return nil
			}
			if existing.Status == model.ManjuProductionPrevisStatusQueued ||
				existing.Status == model.ManjuProductionPrevisStatusRunning {
				existingTaskID = existing.TaskID
				return nil
			}
			previsAttemptVersion = existing.Version + 1
		}
		if previsAttemptVersion <= 0 {
			previsAttemptVersion = 1
		}
		binding, err := tx.ManjuProjectModelBinding(projectID)
		if err != nil {
			return err
		}
		if binding == nil || strings.TrimSpace(binding.TextChannelID) == "" || strings.TrimSpace(binding.TextModelKey) == "" {
			return ErrInvalidModelBinding
		}
		p3Stage, err := tx.ManjuProductionStage(runID, ProductionStageStoryboard)
		if err != nil {
			return err
		}
		if p3Stage == nil ||
			p3Stage.Status != model.ManjuProductionStageStatusLocked ||
			strings.TrimSpace(p3Stage.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		state, snapshot, err := manjuProjectStateMapForArtifact(
			tx,
			projectID,
			run.LockRevision,
			"production:director_storyboard",
			p3Stage.ArtifactID,
		)
		if err != nil {
			return err
		}
		if snapshot == nil {
			return ErrProjectStateUnavailable
		}
		stateJSON, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		feed, err := productionArtifactContent(tx, runID, stage.ArtifactID)
		if err != nil {
			return err
		}
		segments := len(anySlice(state["segments"]))
		if segments <= 0 {
			return ErrProjectStateUnavailable
		}
		prompt := buildPrevisStoryboardPrompt(access.Project.Name, *run, feed, string(stateJSON), segments, previsContext)
		promptHash := sha256.Sum256([]byte(prompt))
		operationKey = fmt.Sprintf(
			"manju:previs:%s:v%d:%s",
			run.ID,
			previsAttemptVersion,
			hex.EncodeToString(promptHash[:])[:12],
		)
		ruleSnapshot, err := s.ensureManjuRuleSnapshot(tx, projectID, run.LockRevision, actorID)
		if err != nil {
			return err
		}
		exchangeID := uuid.NewString()
		request = localtask.CreateRequest{
			ProjectID: projectID,
			Type:      "canvas_text",
			Operation: "manju_previs_storyboard",
			Prompt:    prompt,
			Model:     binding.TextModelKey,
			Input: map[string]any{
				"mode":   "text",
				"prompt": prompt,
				"config": map[string]any{"channelId": binding.TextChannelID, "model": binding.TextModelKey},
				"metadata": map[string]any{
					"clientOperationId":       operationKey,
					"manjuProjectId":          projectID,
					"manjuProductionRunId":    run.ID,
					"manjuPrevisSegmentCount": segments,
					"manjuRuleSnapshot":       ruleSnapshot.ID,
					"manjuModelExchange":      exchangeID,
				},
			},
		}
		requestJSON, err := json.Marshal(request)
		if err != nil {
			return err
		}
		exchange, err := buildManjuModelExchange(
			projectID, run.ID, stage.ID, "P4.5", operationKey, string(requestJSON), "",
		)
		if err != nil {
			return err
		}
		exchange.ID = exchangeID
		if err := tx.CreateManjuModelExchange(exchange); err != nil {
			return err
		}
		previsRuleHash := sha256.Sum256([]byte(previsContext))
		if err := tx.CreateManjuRuleReadReceipts([]model.ManjuRuleReadReceipt{{
			ProjectID:     projectID,
			RunID:         run.ID,
			StageID:       stage.ID,
			SnapshotID:    ruleSnapshot.ID,
			StageKey:      "P4.5",
			RulePath:      "sd-storyboard-engine/references/p45-previs-storyboard.md",
			StartByte:     0,
			EndByte:       int64(len([]byte(previsContext))),
			ContentSHA256: hex.EncodeToString(previsRuleHash[:]),
			RequestID:     operationKey,
			ToolName:      "p45_official_context",
		}}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if alreadyReady || existingTaskID != "" {
		return s.ProductionRunDetail(actorID, projectID, runID)
	}
	if s.scriptTaskCreate == nil {
		return nil, ErrScriptTaskUnavailable
	}
	task, err := s.scriptTaskCreate(actorID, request)
	if err != nil {
		s.failProductionPrevis(projectID, runID, err.Error())
		return nil, err
	}
	if task == nil || strings.TrimSpace(task.ID) == "" {
		err := errors.New("P4.5 文本任务服务没有返回任务 ID")
		s.failProductionPrevis(projectID, runID, err.Error())
		return nil, err
	}
	now := time.Now()
	err = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuProductionStage(runID, ProductionStageFeed)
		if err != nil || stage == nil {
			return ErrProductionStageNotFound
		}
		item, err := tx.ManjuProductionPrevis(projectID, runID)
		if err != nil {
			return err
		}
		if item == nil {
			item = &model.ManjuProductionPrevis{
				ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
				CreatedBy: actorID, CreatedAt: now,
			}
		}
		item.TaskID = task.ID
		item.Status = model.ManjuProductionPrevisStatusRunning
		item.Error = ""
		item.Version = previsAttemptVersion
		if err := tx.AttachManjuModelExchangeTask(operationKey, task.ID); err != nil {
			return err
		}
		return tx.SaveManjuProductionPrevis(item)
	})
	if err != nil {
		return nil, err
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

func (s *Service) failProductionPrevis(projectID, runID, message string) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		item, err := tx.ManjuProductionPrevis(projectID, runID)
		if err != nil || item == nil {
			return err
		}
		item.Status = model.ManjuProductionPrevisStatusFailed
		item.Error = strings.TrimSpace(message)
		if err := tx.CompleteManjuModelExchangeByTaskID(
			item.TaskID, "", "", model.ManjuModelExchangeStatusFailed, message,
		); err != nil {
			return err
		}
		return tx.SaveManjuProductionPrevis(item)
	})
}

func (s *Service) reconcileProductionPrevis(actorID, projectID, runID string) error {
	return s.repo.Transaction(func(tx *repository.Repository) error {
		item, err := tx.ManjuProductionPrevis(projectID, runID)
		if err != nil || item == nil {
			return err
		}
		if item.Status != model.ManjuProductionPrevisStatusQueued &&
			item.Status != model.ManjuProductionPrevisStatusRunning {
			return nil
		}
		task, err := tx.TaskForUser(actorID, item.TaskID)
		if errors.Is(err, gorm.ErrRecordNotFound) || task == nil {
			return nil
		}
		if err != nil {
			return err
		}
		switch task.Status {
		case model.TaskStatusFailed, model.TaskStatusCancelled:
			item.Status = model.ManjuProductionPrevisStatusFailed
			item.Error = fallbackValue(strings.TrimSpace(task.Error), "P4.5 文本任务未成功完成")
			return tx.SaveManjuProductionPrevis(item)
		case model.TaskStatusSucceeded:
			content, err := scriptTaskResultText(task.ResultJSON)
			if err != nil || strings.TrimSpace(content) == "" {
				item.Status = model.ManjuProductionPrevisStatusFailed
				item.Error = "P4.5 文本任务成功但没有可读取的正文"
				return tx.SaveManjuProductionPrevis(item)
			}
			run, err := tx.ManjuProductionRun(projectID, runID)
			if err != nil || run == nil {
				item.Status = model.ManjuProductionPrevisStatusFailed
				item.Error = "P4.5 无法读取生产链锁定版本"
				return tx.SaveManjuProductionPrevis(item)
			}
			p3Stage, err := tx.ManjuProductionStage(runID, ProductionStageStoryboard)
			if err != nil {
				return err
			}
			if p3Stage == nil ||
				p3Stage.Status != model.ManjuProductionStageStatusLocked ||
				strings.TrimSpace(p3Stage.ArtifactID) == "" {
				item.Status = model.ManjuProductionPrevisStatusFailed
				item.Error = "P4.5 无法读取当前锁定的 P3 工件"
				return tx.SaveManjuProductionPrevis(item)
			}
			state, snapshot, err := manjuProjectStateMapForArtifact(
				tx,
				projectID,
				run.LockRevision,
				"production:director_storyboard",
				p3Stage.ArtifactID,
			)
			if err != nil || snapshot == nil {
				item.Status = model.ManjuProductionPrevisStatusFailed
				item.Error = "P4.5 无法读取 P3 project_state"
				return tx.SaveManjuProductionPrevis(item)
			}
			segments := len(anySlice(state["segments"]))
			validation, validationErr := defaultManjuSkillRuntime.ValidateP45(content, segments)
			if validationErr != nil || !validation.Passed {
				item.Status = model.ManjuProductionPrevisStatusFailed
				item.Error = "V7.4.26 官方 check_p45.py 未通过"
				if validationErr != nil {
					item.Error += ": " + validationErr.Error()
				} else if strings.TrimSpace(validation.Output) != "" {
					item.Error += ": " + truncateScriptText(validation.Output, 6000)
				}
				return tx.SaveManjuProductionPrevis(item)
			}
			now := time.Now()
			item.Version++
			sum := sha256.Sum256([]byte(content))
			artifact := &model.ManjuProductionArtifact{
				ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: item.StageID,
				Kind: model.ManjuProductionArtifactPrevis, Version: item.Version,
				Content: content, ContentHash: hex.EncodeToString(sum[:]), TaskID: task.ID,
				CreatedBy: actorID, CreatedAt: now,
			}
			if err := tx.CreateManjuProductionArtifact(artifact); err != nil {
				return err
			}
			if err := recordManjuArtifactAudit(
				tx, projectID, runID, item.StageID, artifact.ID, "P4.5", artifact.Kind,
				artifact.Version, manjuArtifactVisibility("P4.5", ""),
				artifact.Content, artifact.ContentHash, artifact.TaskID, actorID,
			); err != nil {
				return err
			}
			item.ArtifactID = artifact.ID
			item.Status = model.ManjuProductionPrevisStatusReady
			item.Error = ""
			return tx.SaveManjuProductionPrevis(item)
		}
		return nil
	})
}

func buildPrevisStoryboardPrompt(projectName string, run model.ManjuProductionRun, feed, stateJSON string, segments int, rules string) string {
	var out strings.Builder
	out.WriteString("你是 VisiDeep AI 漫剧工作室的 P4.5 预演故事板执行器，严格遵循漫剧老李 AIGC 全流程 V7.4.26。\n")
	out.WriteString("只输出独立 Markdown 附件，不输出解释或代码围栏。P4.5 只生成给人核对的故事板提示词，不生成图片，不进入视频投喂稿。\n")
	out.WriteString("唯一权威文件：sd-storyboard-engine/references/p45-previs-storyboard.md。输出文件名语义固定为 `{项目名}-预演故事板.md`。\n")
	out.WriteString(manjuStrictRuleBoundaryText())
	out.WriteString("\n")
	fmt.Fprintf(&out, "【项目】%s\n【生产链】%s\n【必须覆盖段数】%d\n\n", projectName, run.ID, segments)
	out.WriteString("【V7.4.26 P4.5 权威规则】\n")
	out.WriteString(rules)
	out.WriteString("\n\n【P453 六问硬模板】\n")
	out.WriteString("每一个“第 k 格：”必须按以下顺序逐项写满，不得只写叙事散文，不得省略标签，不得写“同上”：\n")
	out.WriteString("第 k 格：谁在哪：{具名人物 + 具体位置}；朝哪：{身体朝向}；看哪：{视线落点}；做什么：{可见动作}；镜头在哪：{景别 + 机位 + 角度}；光从哪来：{光源方向 + 光线性质}。\n")
	out.WriteString("输出前逐格执行 P453 六问自检：六项任一缺失就重写该格，确认每格都同时包含“谁在哪、朝哪、看哪、做什么、镜头在哪、光从哪来”。\n")
	out.WriteString("\n\n【P4.5 逐段格数硬骨架】\n")
	out.WriteString("必须逐字保留每段的格数声明和每一行“第 k 格：”。不得把多格合并成一个“第 1 格”，不得只写一张总故事板描述。\n")
	out.WriteString(p45PanelSkeleton(stateJSON, segments))
	out.WriteString("\n\n【已锁定 P4 投喂稿】\n")
	out.WriteString(feed)
	out.WriteString("\n\n【P3 project_state】\n")
	out.WriteString(stateJSON)
	return out.String()
}

func p45PanelSkeleton(stateJSON string, expectedSegments int) string {
	var state map[string]any
	_ = json.Unmarshal([]byte(stateJSON), &state)
	segments := anySlice(state["segments"])
	if len(segments) == 0 {
		segments = make([]any, 0, expectedSegments)
		for index := 1; index <= expectedSegments; index++ {
			segments = append(segments, map[string]any{
				"seg_id":            fmt.Sprintf("段%d", index),
				"storyboard_panels": []any{"起始", "过程", "反应", "出口"},
			})
		}
	}
	var out strings.Builder
	for index, raw := range segments {
		segment := anyMap(raw)
		segmentCode := fallbackValue(strings.TrimSpace(anyString(segment["seg_id"])), fmt.Sprintf("段%d", index+1))
		segmentNumber := strings.TrimSpace(strings.TrimPrefix(segmentCode, "段"))
		if segmentNumber == "" {
			segmentNumber = fmt.Sprintf("%d", index+1)
		}
		panelCount := len(anySlice(segment["storyboard_panels"]))
		if panelCount < 3 || panelCount > 9 {
			panelCount = 4
		}
		fmt.Fprintf(&out, "## 段 %s\n", segmentNumber)
		fmt.Fprintf(&out, "### 故事板提示词（%d 格）\n", panelCount)
		for panel := 1; panel <= panelCount; panel++ {
			fmt.Fprintf(&out, "第 %d 格：\n", panel)
		}
		out.WriteString("\n")
	}
	return strings.TrimSpace(out.String())
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

func (s *Service) repairLegacyProductionPrerequisite(actorID, projectID, runID string) error {
	return s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		return repairLegacyProductionDecisionStage(tx, run)
	})
}

func (s *Service) preflightProductionStage(actorID, projectID, runID, stageKey string) error {
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !canEditProduction(access.Role) {
		return ErrForbidden
	}
	run, err := s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return err
	}
	if run == nil {
		return ErrProductionRunNotFound
	}
	if run.Status != model.ManjuProductionRunStatusActive || run.CurrentStage != stageKey {
		return ErrProductionStageNotReady
	}
	stages, err := s.repo.ManjuProductionStages(runID)
	if err != nil {
		return err
	}
	return ensureProductionStageGate(stages, stageKey)
}

func (s *Service) RunProductionStage(actorID, projectID, runID, stageKey string, input ProductionStageRunInput) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Instruction = strings.TrimSpace(input.Instruction)
	if stageKey != ProductionStageDecisionAnchor &&
		stageKey != ProductionStageStoryboard &&
		stageKey != ProductionStageFeed &&
		stageKey != ProductionStageQC {
		return nil, ErrProductionStageNotFound
	}
	if err := s.repairLegacyProductionPrerequisite(actorID, projectID, runID); err != nil {
		return nil, err
	}
	if s.officialSkillValidationEnabled() && stageKey == ProductionStageFeed {
		return s.runOfficialProductionFeedCompiler(actorID, projectID, runID, input)
	}
	if s.scriptTaskCreate == nil {
		if err := s.preflightProductionStage(actorID, projectID, runID, stageKey); err != nil {
			return nil, err
		}
		return nil, ErrScriptTaskUnavailable
	}

	var request localtask.CreateRequest
	var existingTaskID string
	var operationKey string
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		if err := repairLegacyProductionDecisionStage(tx, run); err != nil {
			return err
		}
		if run.Status != model.ManjuProductionRunStatusActive || run.CurrentStage != stageKey {
			return ErrProductionStageNotReady
		}
		stage, err := tx.ManjuProductionStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrProductionStageNotFound
		}
		if strings.TrimSpace(stage.TaskID) != "" {
			existingTaskID = stage.TaskID
			return nil
		}
		if stage.Status != model.ManjuProductionStageStatusInProgress &&
			stage.Status != model.ManjuProductionStageStatusBlocked {
			return ErrProductionStageNotReady
		}
		stages, err := tx.ManjuProductionStages(runID)
		if err != nil {
			return err
		}
		if err := ensureProductionStageGate(stages, stageKey); err != nil {
			return err
		}
		binding, err := tx.ManjuProjectModelBinding(projectID)
		if err != nil {
			return err
		}
		if binding == nil || strings.TrimSpace(binding.TextChannelID) == "" || strings.TrimSpace(binding.TextModelKey) == "" {
			return ErrInvalidModelBinding
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil ||
			assetRun.Status != model.ManjuAssetRunStatusCompleted ||
			assetRun.LockRevision != run.LockRevision {
			return ErrProductionAssetNotReady
		}
		assetArtifacts, err := lockedAssetArtifacts(tx, *assetRun)
		if err != nil {
			return err
		}
		productionArtifacts, err := lockedProductionArtifacts(tx, runID)
		if err != nil {
			return err
		}
		productionImages, err := tx.ManjuProductionImages(runID)
		if err != nil {
			return err
		}
		productionVideos, err := tx.ManjuProductionVideos(runID)
		if err != nil {
			return err
		}
		scriptRun, err := tx.ManjuScriptRun(projectID, assetRun.ScriptRunID)
		if err != nil {
			return err
		}
		if scriptRun == nil {
			return ErrProductionAssetNotReady
		}
		scriptArtifacts, err := lockedScriptArtifacts(tx, *scriptRun)
		if err != nil {
			return err
		}
		stageCode := "P3"
		if stage.Key == ProductionStageDecisionAnchor {
			stageCode = "P0B"
		} else if stage.Key == ProductionStageFeed {
			stageCode = "P4"
		} else if stage.Key == ProductionStageQC {
			stageCode = "P5"
		}
		combat := productionRuleCombat(scriptArtifacts, assetArtifacts, productionArtifacts, input.Instruction)
		ruleBundle, err := defaultManjuSkillRuntime.StageRuleDispatch(manjuSkillStageOptions(stageCode, receipt, assetRun.AssetMode, combat))
		if err != nil {
			return err
		}
		skillContext := ruleBundle.Content
		prompt := buildProductionStagePrompt(access.Project.Name, *run, *stage, *assetRun, receipt, scriptArtifacts, assetArtifacts, productionArtifacts, productionImages, productionVideos, input.Instruction, skillContext)
		operationKey = fmt.Sprintf("manju:production:%s:%s:v%d", run.ID, stage.Key, stage.Version+1)
		ruleSnapshot, err := s.ensureManjuRuleSnapshot(tx, projectID, run.LockRevision, actorID)
		if err != nil {
			return err
		}
		exchangeID := uuid.NewString()
		request = localtask.CreateRequest{
			ProjectID: projectID,
			Type:      "canvas_text",
			Operation: "manju_production_stage",
			Prompt:    prompt,
			Model:     binding.TextModelKey,
			Input: map[string]any{
				"mode":   "text",
				"prompt": prompt,
				"config": map[string]any{
					"channelId": binding.TextChannelID,
					"model":     binding.TextModelKey,
				},
				"metadata": map[string]any{
					"clientOperationId":       operationKey,
					"manjuProjectId":          projectID,
					"manjuProductionRunId":    run.ID,
					"manjuProductionStageKey": stage.Key,
					"manjuAssetRunId":         assetRun.ID,
					"manjuAssetMode":          assetRun.AssetMode,
					"manjuRuleSnapshot":       ruleSnapshot.ID,
					"manjuModelExchange":      exchangeID,
				},
			},
		}
		requestJSON, err := json.Marshal(request)
		if err != nil {
			return err
		}
		exchange, err := buildManjuModelExchange(
			projectID, run.ID, stage.ID, stage.Key, operationKey, string(requestJSON), "",
		)
		if err != nil {
			return err
		}
		exchange.ID = exchangeID
		if err := tx.CreateManjuModelExchange(exchange); err != nil {
			return err
		}
		if err := s.recordManjuRuleReads(tx, ruleSnapshot, run.ID, stage.ID, stageCode, operationKey, ruleBundle); err != nil {
			return err
		}
		stageInput, err := json.Marshal(map[string]string{
			"instruction":     input.Instruction,
			"prompt":          prompt,
			"ruleSnapshotId":  ruleSnapshot.ID,
			"modelExchangeId": exchange.ID,
		})
		if err != nil {
			return err
		}
		stage.InputJSON = string(stageInput)
		stage.Status = model.ManjuProductionStageStatusInProgress
		stage.Error = ""
		return tx.SaveManjuProductionStage(stage)
	})
	if err != nil {
		return nil, err
	}
	if existingTaskID != "" {
		return s.ProductionRunDetail(actorID, projectID, runID)
	}
	task, err := s.scriptTaskCreate(actorID, request)
	if err != nil {
		s.blockProductionStage(projectID, runID, stageKey, err.Error())
		return nil, err
	}
	if task == nil || strings.TrimSpace(task.ID) == "" {
		err := errors.New("文本任务服务没有返回任务 ID")
		s.blockProductionStage(projectID, runID, stageKey, err.Error())
		return nil, err
	}
	if err := s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuProductionStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrProductionStageNotFound
		}
		if strings.TrimSpace(stage.TaskID) == "" {
			stage.TaskID = task.ID
		}
		stage.Status = model.ManjuProductionStageStatusInProgress
		stage.Error = ""
		if err := tx.AttachManjuModelExchangeTask(operationKey, task.ID); err != nil {
			return err
		}
		return tx.SaveManjuProductionStage(stage)
	}); err != nil {
		return nil, err
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

func (s *Service) blockProductionStage(projectID, runID, stageKey, message string) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuProductionStage(runID, stageKey)
		if err != nil || stage == nil {
			return err
		}
		taskID := stage.TaskID
		stage.Status = model.ManjuProductionStageStatusBlocked
		stage.Error = strings.TrimSpace(message)
		stage.TaskID = ""
		stage.Version++
		if err := tx.CompleteManjuModelExchangeByTaskID(
			taskID, "", "", model.ManjuModelExchangeStatusFailed, message,
		); err != nil {
			return err
		}
		return tx.SaveManjuProductionStage(stage)
	})
}

func (s *Service) ConfirmProductionStage(actorID, projectID, runID, stageKey string, _ ProductionConfirmInput) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	if stageKey != ProductionStageDecisionAnchor &&
		stageKey != ProductionStageStoryboard &&
		stageKey != ProductionStageFeed &&
		stageKey != ProductionStageQC {
		return nil, ErrProductionStageNotFound
	}
	if err := s.repairLegacyProductionPrerequisite(actorID, projectID, runID); err != nil {
		return nil, err
	}
	if err := s.reconcileProductionStageTasks(actorID, projectID, runID); err != nil {
		return nil, err
	}
	canvasVerifiedThrough, err := s.verifyProductionCanvasImport(projectID, runID, stageKey)
	if err != nil {
		return nil, err
	}
	var canvasStoryboardImages []model.ManjuProductionImage
	if stageKey == ProductionStageStoryboard && canvasVerifiedThrough == CanvasPackageStageP3Images {
		canvasStoryboardImages, err = s.p3CanvasStoryboardImages(projectID, runID)
		if err != nil {
			return nil, err
		}
	}
	var deliveryPlan *productionDeliveryPlan
	if stageKey == ProductionStageQC {
		deliveryPlan, err = s.prepareProductionDelivery(actorID, projectID, runID)
		if err != nil {
			return nil, err
		}
	}
	err = s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canConfirmProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		if err := repairLegacyProductionDecisionStage(tx, run); err != nil {
			return err
		}
		stage, err := tx.ManjuProductionStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrProductionStageNotFound
		}
		if run.Status == model.ManjuProductionRunStatusCompleted && stage.Status == model.ManjuProductionStageStatusLocked {
			if stageKey == ProductionStageQC {
				if deliveryPlan == nil || deliveryPlan.ArtifactID != stage.ArtifactID {
					return ErrProductionDeliverablesNotReady
				}
				if !deliveryPlan.AlreadyReady {
					if err := persistProductionDeliveryRows(tx, projectID, runID, deliveryPlan.Mode, deliveryPlan.Rows); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if run.Status != model.ManjuProductionRunStatusActive ||
			run.CurrentStage != stageKey ||
			stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation ||
			strings.TrimSpace(stage.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		if stageKey == ProductionStageFeed {
			p3Stage, err := tx.ManjuProductionStage(runID, ProductionStageStoryboard)
			if err != nil {
				return err
			}
			if p3Stage == nil ||
				p3Stage.Status != model.ManjuProductionStageStatusLocked ||
				strings.TrimSpace(p3Stage.ArtifactID) == "" {
				return ErrProductionStageNotReady
			}
			if err := validateProductionStageMachineCheckProof(*stage, p3Stage.ArtifactID); err != nil {
				return err
			}
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil {
			return ErrProductionAssetNotReady
		}
		content, err := productionArtifactContent(tx, runID, stage.ArtifactID)
		if err != nil {
			return err
		}
		if stageKey == ProductionStageDecisionAnchor {
			if err := validateP0BDecisionArtifact(content); err != nil {
				return fmt.Errorf("%w: %v", ErrProductionStageNotReady, err)
			}
		}
		if stageKey == ProductionStageStoryboard {
			candidateStateJSON, err := previewManjuProjectStatePatch(
				tx,
				projectID,
				access.Project.Name,
				run.LockRevision,
				"production:director_storyboard",
				content,
				actorID,
			)
			if err != nil {
				return fmt.Errorf(
					"P3_REGENERATION_REQUIRED: 当前 P3 工件无法建立 V7.4.26 项目状态台账，请只重新生成 P3；历史工件和媒体已保留: %w",
					err,
				)
			}
			var state map[string]any
			if err := json.Unmarshal([]byte(candidateStateJSON), &state); err != nil {
				return fmt.Errorf(
					"P3_REGENERATION_REQUIRED: 当前 P3 项目状态无法读取，请只重新生成 P3；历史工件和媒体已保留: %w",
					err,
				)
			}
			required, err := p3RequiredSegments(state)
			if err != nil {
				return err
			}
			if err := validateP3StoryboardTable(content, required); err != nil {
				return err
			}
			requirements, err := p3DeliveryRequirements(content, required)
			if err != nil {
				return err
			}
			images, err := reconcileProductionStoryboardImageLineage(
				tx,
				projectID,
				runID,
				*stage,
				required,
				requirements,
				canvasStoryboardImages,
			)
			if err != nil {
				return err
			}
			if err := validateP3Delivery(content, state, images); err != nil {
				return fmt.Errorf(
					"P3_REGENERATION_REQUIRED: 当前 P3 未满足完整表演、导演、故事板交付契约；请只重新生成 P3 并补齐逐段图片或合法单镜过场豁免，历史工件和媒体已保留: %w",
					err,
				)
			}
			if len(images) > 0 {
				if err := storyboardImagesReady(tx, runID); err != nil {
					return fmt.Errorf(
						"P3_REGENERATION_REQUIRED: 当前 P3 故事板图资源已失效或归属不符；请重新生成或重新对账 P3 图片，历史工件和媒体已保留: %w",
						err,
					)
				}
			}
		}
		// P4 is compiled and machine-checked by the official V7.4.26
		// compile_feed.py + validate_prompt.py + gate_check.py chain.
		// Its output intentionally follows the official seven-section format
		// rather than the older platform Markdown wrapper.
		if stageKey == ProductionStageStoryboard && s.officialSkillValidationEnabled() {
			if err := commitManjuProjectStatePatch(tx, projectID, access.Project.Name, run.LockRevision, "production:director_storyboard", stage.ArtifactID, content, actorID, true); err != nil {
				return err
			}
		}
		if stageKey == ProductionStageFeed {
			previs, err := tx.ManjuProductionPrevis(projectID, runID)
			if err != nil {
				return err
			}
			if previs == nil ||
				previs.Status != model.ManjuProductionPrevisStatusReady ||
				strings.TrimSpace(previs.ArtifactID) == "" {
				return ErrProductionPrevisRequired
			}
			previsContent, err := productionArtifactContent(tx, runID, previs.ArtifactID)
			if err != nil || strings.TrimSpace(previsContent) == "" {
				return ErrProductionPrevisRequired
			}
		}
		if stageKey == ProductionStageFeed && canvasVerifiedThrough != CanvasPackageStageP4Videos {
			if err := productionVideosReady(tx, runID); err != nil {
				return err
			}
		}
		if stageKey == ProductionStageQC && canvasVerifiedThrough != CanvasPackageStageP4Videos {
			if err := productionVideosReady(tx, runID); err != nil {
				return err
			}
		}
		if stageKey == ProductionStageQC {
			if deliveryPlan == nil || deliveryPlan.ArtifactID != stage.ArtifactID {
				return ErrProductionDeliverablesNotReady
			}
			if err := persistProductionDeliveryRows(tx, projectID, runID, deliveryPlan.Mode, deliveryPlan.Rows); err != nil {
				return err
			}
		}
		now := time.Now()
		stage.Status = model.ManjuProductionStageStatusLocked
		stage.ConfirmedBy = actorID
		stage.ConfirmedAt = &now
		stage.Error = ""
		if err := tx.SaveManjuProductionStage(stage); err != nil {
			return err
		}

		switch stageKey {
		case ProductionStageDecisionAnchor, ProductionStageStoryboard:
			next, err := activateNextProductionStage(tx, runID, stage.Position)
			if err != nil {
				return err
			}
			if next == nil {
				return ErrProductionStageNotFound
			}
			run.CurrentStage = next.Key
		case ProductionStageFeed:
			next, err := activateNextProductionStage(tx, runID, stage.Position)
			if err != nil {
				return err
			}
			if next == nil {
				return ErrProductionStageNotFound
			}
			run.CurrentStage = next.Key
		case ProductionStageQC:
			run.Status = model.ManjuProductionRunStatusCompleted
			run.CurrentStage = ""
		}
		return tx.SaveManjuProductionRun(run)
	})
	if err != nil {
		return nil, err
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

func (s *Service) ProductionRunDetail(actorID, projectID, runID string) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
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
	run, err := s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrProductionRunNotFound
	}
	if err := s.repo.Transaction(func(tx *repository.Repository) error {
		return repairLegacyProductionDecisionStage(tx, run)
	}); err != nil {
		return nil, err
	}
	run, err = s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrProductionRunNotFound
	}
	if err := s.reconcileProductionStageTasks(actorID, projectID, runID); err != nil {
		return nil, err
	}
	if err := s.reconcileProductionPrevis(actorID, projectID, runID); err != nil {
		return nil, err
	}
	run, err = s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrProductionRunNotFound
	}
	assetRun, err := s.repo.ManjuAssetRun(projectID, run.AssetRunID)
	if err != nil {
		return nil, err
	}
	if assetRun == nil {
		return nil, ErrProductionAssetNotReady
	}
	scriptRun, err := s.repo.ManjuScriptRun(projectID, assetRun.ScriptRunID)
	if err != nil {
		return nil, err
	}
	if scriptRun == nil {
		return nil, ErrAssetScriptNotReady
	}
	stages, err := s.repo.ManjuProductionStages(runID)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.repo.ManjuProductionArtifacts(runID)
	if err != nil {
		return nil, err
	}
	images, err := s.repo.ManjuProductionImages(runID)
	if err != nil {
		return nil, err
	}
	videos, err := s.repo.ManjuProductionVideos(runID)
	if err != nil {
		return nil, err
	}
	deliverables, err := s.repo.ManjuDeliverablesForRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	runView := productionRunView(*run)
	runView.HasDeliverables = len(deliverables) > 0
	detail := &ProductionRunDetail{
		Run:       runView,
		AssetRun:  assetRunView(*assetRun),
		ScriptRun: scriptRunView(*scriptRun),
		Stages:    make([]ProductionStageView, 0, len(stages)),
		Artifacts: make([]ProductionArtifactView, 0, len(artifacts)),
		Images:    make([]ProductionImageView, 0, len(images)),
		Videos:    make([]ProductionVideoView, 0, len(videos)),
	}
	previs, err := s.repo.ManjuProductionPrevis(projectID, runID)
	if err != nil {
		return nil, err
	}
	if previs != nil {
		view := productionPrevisView(*previs)
		detail.Previs = &view
	}
	p3ArtifactID := ""
	for _, stage := range stages {
		if stage.Key == ProductionStageStoryboard &&
			stage.Status == model.ManjuProductionStageStatusLocked &&
			strings.TrimSpace(stage.ArtifactID) != "" {
			p3ArtifactID = stage.ArtifactID
			break
		}
	}
	for _, stage := range stages {
		view := productionStageView(stage, p3ArtifactID)
		if stage.Status == model.ManjuProductionStageStatusBlocked {
			draft, draftErr := s.repo.LatestManjuStageDraft(runID, stage.ID)
			if draftErr != nil {
				return nil, draftErr
			}
			view.Draft = stageDraftView(draft)
		}
		detail.Stages = append(detail.Stages, view)
	}
	for _, artifact := range artifacts {
		detail.Artifacts = append(detail.Artifacts, ProductionArtifactView{
			ID: artifact.ID, StageID: artifact.StageID, Kind: artifact.Kind,
			Version: artifact.Version, Content: artifact.Content, ContentHash: artifact.ContentHash,
			TaskID: artifact.TaskID, CreatedAt: artifact.CreatedAt,
		})
	}
	for _, image := range images {
		detail.Images = append(detail.Images, productionImageView(image))
	}
	for _, video := range videos {
		detail.Videos = append(detail.Videos, productionVideoView(video))
	}
	return detail, nil
}

func (s *Service) reconcileProductionStageTasks(actorID, projectID, runID string) error {
	lockSnapshot := s.latestCanvasLockSnapshot(projectID)
	return s.repo.Transaction(func(tx *repository.Repository) error {
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil {
			return ErrProductionAssetNotReady
		}
		stages, err := tx.ManjuProductionStages(runID)
		if err != nil {
			return err
		}
		for index := range stages {
			stage := &stages[index]
			if strings.TrimSpace(stage.TaskID) == "" || stage.Status != model.ManjuProductionStageStatusInProgress {
				continue
			}
			task, err := tx.TaskForUser(actorID, stage.TaskID)
			if errors.Is(err, gorm.ErrRecordNotFound) || task == nil {
				continue
			}
			if err != nil {
				return err
			}
			switch task.Status {
			case model.TaskStatusFailed, model.TaskStatusCancelled:
				stage.Status = model.ManjuProductionStageStatusBlocked
				stage.Error = strings.TrimSpace(task.Error)
				if stage.Error == "" {
					stage.Error = "文本任务未成功完成"
				}
				stage.TaskID = ""
				stage.Version++
				if err := tx.SaveManjuProductionStage(stage); err != nil {
					return err
				}
			case model.TaskStatusSucceeded:
				if strings.TrimSpace(stage.ArtifactID) != "" {
					continue
				}
				content, err := scriptTaskResultText(task.ResultJSON)
				if err != nil || strings.TrimSpace(content) == "" {
					if err := blockProductionStageRecord(tx, stage, "文本任务成功但没有可读取的正文"); err != nil {
						return err
					}
					continue
				}
				if stage.Key == ProductionStageStoryboard && s.officialSkillValidationEnabled() {
					if err := validateRequiredManjuProjectStatePatch("production:director_storyboard", content); err != nil {
						message := manjuProjectStateFailure("P3 分镜与故事板", err)
						if draftErr := saveFailedManjuStageDraft(
							tx, projectID, "P3", runID, stage.ID, stage.Key, content,
							"MODEL_FAILED", validationJSONForError(message), actorID,
						); draftErr != nil {
							return draftErr
						}
						if blockErr := blockProductionStageRecord(tx, stage, message); blockErr != nil {
							return blockErr
						}
						continue
					}
				}
				if stage.Key == ProductionStageFeed && s.officialSkillValidationEnabled() {
					message := "历史 P4 文本任务不能作为 PASS；V7.4.26 要求重新执行 compile_feed.py -> validate_prompt.py -> gate_check.py -> script_feed_diff.py 完整链路。"
					if blockErr := blockProductionStageRecord(tx, stage, message); blockErr != nil {
						return blockErr
					}
					continue
				}
				if stage.Key == ProductionStageQC && s.officialSkillValidationEnabled() {
					contract, contractErr := auditProductionContractWithTx(tx, projectID, *assetRun, runID, content, lockSnapshot.StyleCardName)
					if contractErr != nil || !contract.Passed {
						message := "V7.4.26 官方 audit_contract.py 全流程交付契约未通过"
						if contractErr != nil {
							message += ": " + contractErr.Error()
						} else {
							message += ": " + truncateScriptText(contract.Output, 6000)
						}
						if blockErr := blockProductionStageRecord(tx, stage, message); blockErr != nil {
							return blockErr
						}
						continue
					}
				}
				if stage.Key == ProductionStageDecisionAnchor {
					if err := validateP0BDecisionArtifact(content); err != nil {
						message := fmt.Sprintf("%v", err)
						if draftErr := saveFailedManjuStageDraft(
							tx, projectID, "P0B", runID, stage.ID, stage.Key, content,
							"MODEL_FAILED", validationJSONForError(message), actorID,
						); draftErr != nil {
							return draftErr
						}
						if blockErr := blockProductionStageRecord(tx, stage, message); blockErr != nil {
							return blockErr
						}
						continue
					}
				}
				now := time.Now()
				stage.Version++
				sum := sha256.Sum256([]byte(content))
				artifact := &model.ManjuProductionArtifact{
					ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
					Kind: productionArtifactKind(stage.Key), Version: stage.Version,
					Content: content, ContentHash: hex.EncodeToString(sum[:]), TaskID: task.ID,
					CreatedBy: actorID, CreatedAt: now,
				}
				if err := tx.CreateManjuProductionArtifact(artifact); err != nil {
					return err
				}
				phase := "P3"
				if stage.Key == ProductionStageDecisionAnchor {
					phase = "P0B"
				} else if stage.Key == ProductionStageFeed {
					phase = "P4"
				} else if stage.Key == ProductionStageQC {
					phase = "P5"
				}
				if err := recordManjuArtifactAudit(
					tx, projectID, runID, stage.ID, artifact.ID, phase, artifact.Kind,
					artifact.Version, manjuArtifactVisibility(phase, stage.Key), artifact.Content, artifact.ContentHash,
					artifact.TaskID, actorID,
				); err != nil {
					return err
				}
				stage.ArtifactID = artifact.ID
				stage.CompletedAt = &now
				stage.Status = model.ManjuProductionStageStatusAwaitingConfirmation
				stage.Error = ""
				if err := tx.SaveManjuProductionStage(stage); err != nil {
					return err
				}
				run.CurrentStage = stage.Key
				if err := tx.SaveManjuProductionRun(run); err != nil {
					return err
				}
			}
		}
		if err := reconcileProductionStoryboardImages(tx, runID); err != nil {
			return err
		}
		if err := reconcileProductionVideos(tx, runID); err != nil {
			return err
		}
		return nil
	})
}

func blockProductionStageRecord(tx *repository.Repository, stage *model.ManjuProductionStage, message string) error {
	taskID := stage.TaskID
	stage.Status = model.ManjuProductionStageStatusBlocked
	stage.Error = strings.TrimSpace(message)
	stage.TaskID = ""
	stage.Version++
	if err := tx.CompleteManjuModelExchangeByTaskID(
		taskID, "", "", model.ManjuModelExchangeStatusFailed, message,
	); err != nil {
		return err
	}
	return tx.SaveManjuProductionStage(stage)
}

func activateNextProductionStage(tx *repository.Repository, runID string, position int) (*model.ManjuProductionStage, error) {
	now := time.Now()
	next, err := tx.ManjuProductionStageByPosition(runID, position+1)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return nil, nil
	}
	if next.Status == model.ManjuProductionStageStatusNotStarted || next.Status == model.ManjuProductionStageStatusBlocked {
		next.Status = model.ManjuProductionStageStatusInProgress
		next.StartedAt = &now
		next.Error = ""
		if err := tx.SaveManjuProductionStage(next); err != nil {
			return nil, err
		}
	}
	return next, nil
}

func ensureProductionStageGate(stages []model.ManjuProductionStage, stageKey string) error {
	index := productionStageIndex(stages, stageKey)
	if index < 0 {
		return ErrProductionStageNotFound
	}
	if index == 0 {
		return nil
	}
	previous := stages[index-1]
	if previous.Status != model.ManjuProductionStageStatusLocked || strings.TrimSpace(previous.ArtifactID) == "" {
		return ErrProductionStageNotReady
	}
	return nil
}

func lockedAssetArtifacts(tx *repository.Repository, run model.ManjuAssetRun) ([]model.ManjuAssetArtifact, error) {
	stages, err := tx.ManjuAssetStages(run.ID)
	if err != nil {
		return nil, err
	}
	artifacts, err := tx.ManjuAssetArtifacts(run.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.ManjuAssetArtifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	result := make([]model.ManjuAssetArtifact, 0, len(stages))
	for _, stage := range stages {
		if stage.Status != model.ManjuAssetStageStatusLocked || strings.TrimSpace(stage.ArtifactID) == "" {
			continue
		}
		if artifact, ok := byID[stage.ArtifactID]; ok {
			result = append(result, artifact)
		}
	}
	return result, nil
}

func lockedProductionArtifacts(tx *repository.Repository, runID string) ([]model.ManjuProductionArtifact, error) {
	stages, err := tx.ManjuProductionStages(runID)
	if err != nil {
		return nil, err
	}
	artifacts, err := tx.ManjuProductionArtifacts(runID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.ManjuProductionArtifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	result := make([]model.ManjuProductionArtifact, 0, len(stages))
	for _, stage := range stages {
		if stage.Status != model.ManjuProductionStageStatusLocked || strings.TrimSpace(stage.ArtifactID) == "" {
			continue
		}
		if artifact, ok := byID[stage.ArtifactID]; ok {
			result = append(result, artifact)
		}
	}
	return result, nil
}

func productionArtifactContent(tx *repository.Repository, runID, artifactID string) (string, error) {
	artifacts, err := tx.ManjuProductionArtifacts(runID)
	if err != nil {
		return "", err
	}
	for _, artifact := range artifacts {
		if artifact.ID == artifactID {
			return artifact.Content, nil
		}
	}
	return "", ErrProductionStageNotReady
}

func buildProductionStagePrompt(projectName string, run model.ManjuProductionRun, stage model.ManjuProductionStage, assetRun model.ManjuAssetRun, receipt *model.ManjuLockReceipt, scriptArtifacts []model.ManjuScriptArtifact, assetArtifacts []model.ManjuAssetArtifact, productionArtifacts []model.ManjuProductionArtifact, productionImages []model.ManjuProductionImage, productionVideos []model.ManjuProductionVideo, instruction, skillContext string) string {
	var out strings.Builder
	out.WriteString("你是 VisiDeep AI 漫剧工作室的 P0B-P5 生产执行器，严格遵循漫剧老李 AIGC 全流程 V7.4.26。\n")
	out.WriteString("只输出本阶段 Markdown 成果，不输出思考过程。不得伪造人工确认、PASS、成片、图片或视频生成结果。\n")
	out.WriteString("当前阶段只使用项目文本模型，不调用新的图片模型或视频模型，也不得把文本任务说成图片或视频生成任务。\n\n")
	out.WriteString("P0 画风只能使用可投喂视觉参数；交付稿不得出现导演名、工作室名、影片名或选择层卡号。\n\n")
	fmt.Fprintf(&out, "【项目】%s\n【素材形态】%s\n【P0B-P5 阶段】%s（%s）\n【P2 资产任务】%s\n\n", projectName, assetRun.AssetMode, stage.Title, stage.Key, assetRun.ID)
	if receipt != nil {
		fmt.Fprintf(&out, "%s\n", p0DeliveryPrompt(receipt))
	}
	if stage.Key == ProductionStageDecisionAnchor && len(scriptArtifacts) > 0 {
		out.WriteString("【P1 已锁定且可交接的剧本工件】\n")
		for _, artifact := range scriptArtifacts {
			fmt.Fprintf(&out, "--- %s v%d ---\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
		}
		out.WriteString("\n")
	}
	if len(assetArtifacts) > 0 {
		out.WriteString("【P2 已锁定且可交接的工件】\n")
		for _, artifact := range assetArtifacts {
			fmt.Fprintf(&out, "--- %s v%d ---\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
		}
		out.WriteString("\n")
	}
	if len(productionArtifacts) > 0 {
		out.WriteString("【P3-P5 已确认并锁定的上游工件】\n")
		for _, artifact := range productionArtifacts {
			fmt.Fprintf(&out, "--- %s v%d ---\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
		}
		out.WriteString("\n")
	}
	if assetRun.AssetMode != AssetModeM0Value && (stage.Key == ProductionStageFeed || stage.Key == ProductionStageQC) {
		readyImages := make([]model.ManjuProductionImage, 0, len(productionImages))
		for _, image := range productionImages {
			if image.Status == ProductionImageStatusReady && strings.TrimSpace(image.ResourceID) != "" {
				readyImages = append(readyImages, image)
			}
		}
		if len(readyImages) > 0 {
			out.WriteString("【P3 已锁定故事板图资源映射】\n")
			for _, image := range readyImages {
				shot := strings.TrimSpace(image.ShotLabel)
				if shot == "" {
					shot = "未标注格号"
				}
				fmt.Fprintf(&out, "- %s · %s -> resourceId=%s · url=%s\n", image.SegmentCode, shot, image.ResourceID, fallbackValue(image.URL, "未返回公开 URL"))
			}
			out.WriteString("\n")
		}
	}
	if stage.Key == ProductionStageQC {
		readyVideos := make([]model.ManjuProductionVideo, 0, len(productionVideos))
		for _, video := range productionVideos {
			if video.Status == ProductionVideoStatusReady && strings.TrimSpace(video.ResourceID) != "" {
				readyVideos = append(readyVideos, video)
			}
		}
		if len(readyVideos) > 0 {
			out.WriteString("【P4 已锁定真实生成视频资源映射】\n")
			for _, video := range readyVideos {
				fmt.Fprintf(&out, "- %s -> resourceId=%s · url=%s · mimeType=%s · %d秒\n", video.SegmentCode, video.ResourceID, fallbackValue(video.URL, "未返回公开 URL"), fallbackValue(video.MimeType, "未知"), video.DurationSeconds)
			}
			out.WriteString("\n")
		}
	}
	if instruction != "" {
		fmt.Fprintf(&out, "【本次用户补充】\n%s\n\n", instruction)
	}
	appendManjuSkillContext(&out, skillContext)
	out.WriteString("\n\n【VisiDeep AI 运行接口】\n")
	out.WriteString(productionStageInstruction(stage.Key, assetRun.AssetMode))
	return out.String()
}

func productionStageInstruction(stageKey, assetMode string) string {
	return productionStageInstructionWithLensRange(stageKey, assetMode, nil)
}

func productionStageInstructionWithLensRange(stageKey, assetMode string, lensRange *styleLensRange) string {
	_, _, _ = stageKey, assetMode, lensRange
	return "运行接口：严格按上方官方总控和强制读取原文执行当前阶段。只输出当前阶段官方规定的 Markdown 工件；不要输出规则复述、思考过程、流程回执或机检说明。"
}

func productionArtifactByID(artifacts []model.ManjuProductionArtifact, artifactID string) *model.ManjuProductionArtifact {
	for index := range artifacts {
		if artifacts[index].ID == artifactID {
			return &artifacts[index]
		}
	}
	return nil
}

func auditProductionContractWithTx(tx *repository.Repository, projectID string, assetRun model.ManjuAssetRun, productionRunID, currentContent, director string) (ManjuSkillValidationResult, error) {
	scriptRun, err := tx.ManjuScriptRun(projectID, assetRun.ScriptRunID)
	if err != nil || scriptRun == nil {
		return ManjuSkillValidationResult{}, ErrProductionAssetNotReady
	}
	scriptArtifacts, err := lockedScriptArtifacts(tx, *scriptRun)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	assetArtifacts, err := lockedAssetArtifacts(tx, assetRun)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	productionArtifacts, err := lockedProductionArtifacts(tx, productionRunID)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	previs, err := tx.ManjuProductionPrevis(projectID, productionRunID)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	var previsArtifact *model.ManjuProductionArtifact
	if previs != nil && strings.TrimSpace(previs.ArtifactID) != "" {
		artifacts, err := tx.ManjuProductionArtifacts(productionRunID)
		if err != nil {
			return ManjuSkillValidationResult{}, err
		}
		for index := range artifacts {
			if artifacts[index].ID == previs.ArtifactID {
				previsArtifact = &artifacts[index]
				break
			}
		}
	}
	document := buildProductionContractArchive(scriptArtifacts, assetArtifacts, productionArtifacts, previsArtifact, currentContent)
	return defaultManjuSkillRuntime.AuditContract(document, director)
}

func buildProductionContractArchive(
	scriptArtifacts []model.ManjuScriptArtifact,
	assetArtifacts []model.ManjuAssetArtifact,
	productionArtifacts []model.ManjuProductionArtifact,
	previsArtifact *model.ManjuProductionArtifact,
	currentContent string,
) string {
	var document strings.Builder
	document.WriteString("# VisiDeep AI 漫剧全流程交付归档\n")
	for _, artifact := range scriptArtifacts {
		fmt.Fprintf(&document, "\n# P1 · %s v%d\n\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
	}
	for _, artifact := range assetArtifacts {
		fmt.Fprintf(&document, "\n# P2 · %s v%d\n\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
	}
	for _, artifact := range productionArtifacts {
		fmt.Fprintf(&document, "\n# 生产阶段 · %s v%d\n\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
	}
	if previsArtifact != nil {
		fmt.Fprintf(&document, "\n# P4.5 · %s v%d\n\n%s\n", previsArtifact.Kind, previsArtifact.Version, previsArtifact.Content)
	} else {
		document.WriteString("\n# P4.5 · PREVIS_STORYBOARD.md\n\n预演故事板：⛔未附（缺少已通过机检的 P4.5 工件）\n")
	}
	document.WriteString("\n# P5 当前质检交付\n\n")
	document.WriteString(currentContent)
	return document.String()
}

func productionPrevisView(item model.ManjuProductionPrevis) ProductionPrevisView {
	return ProductionPrevisView{
		ID: item.ID, RunID: item.RunID, TaskID: item.TaskID, ArtifactID: item.ArtifactID,
		Status: item.Status, Version: item.Version, Error: item.Error,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func productionArtifactKind(stageKey string) string {
	switch stageKey {
	case ProductionStageDecisionAnchor:
		return model.ManjuProductionArtifactEmotionAnchor
	case ProductionStageStoryboard:
		return model.ManjuProductionArtifactStoryboard
	case ProductionStageFeed:
		return model.ManjuProductionArtifactFeed
	case ProductionStageQC:
		return model.ManjuProductionArtifactQC
	default:
		return "PRODUCTION_STAGE_OUTPUT.md"
	}
}

func productionStageIndex(stages []model.ManjuProductionStage, key string) int {
	for index := range stages {
		if stages[index].Key == key {
			return index
		}
	}
	return -1
}

func canEditProduction(role string) bool {
	return role == model.ManjuMemberRoleOwner ||
		role == model.ManjuMemberRoleProducer ||
		role == model.ManjuMemberRoleDirector
}

func canConfirmProduction(role string) bool {
	return canEditProduction(role)
}

func productionRunView(run model.ManjuProductionRun) ProductionRunView {
	return ProductionRunView{
		ID: run.ID, ProjectID: run.ProjectID, AssetRunID: run.AssetRunID,
		Status: run.Status, CurrentStage: run.CurrentStage, LockRevision: run.LockRevision,
		CreatedBy: run.CreatedBy, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt,
	}
}

func productionStageView(stage model.ManjuProductionStage, expectedP3ArtifactID string) ProductionStageView {
	view := ProductionStageView{
		ID: stage.ID, Key: stage.Key, Title: stage.Title, Position: stage.Position,
		Status: stage.Status, Version: stage.Version, TaskID: stage.TaskID,
		ArtifactID: stage.ArtifactID, Error: stage.Error, ConfirmedBy: stage.ConfirmedBy,
		ConfirmedAt: stage.ConfirmedAt, StartedAt: stage.StartedAt,
		CompletedAt: stage.CompletedAt, UpdatedAt: stage.UpdatedAt,
	}
	if stage.Key == ProductionStageFeed {
		meta, parseErr := parseProductionStageRuntimeMetadata(stage.InputJSON)
		if parseErr == nil &&
			strings.TrimSpace(expectedP3ArtifactID) != "" &&
			validateManjuProductionMachineCheckProof(meta.MachineCheckProof, expectedP3ArtifactID) == nil {
			view.MachineCheckStatus = model.ManjuProductionMachineCheckStatusPass
			view.MachineCheckProofVerified = true
			view.MachineCheckNote = strings.TrimSpace(meta.MachineCheckNote)
			view.MachineCheckWarnings = append([]string(nil), meta.MachineCheckProof.Warnings...)
		} else if strings.TrimSpace(stage.ArtifactID) != "" {
			view.MachineCheckStatus = model.ManjuProductionMachineCheckStatusLegacyNonCompliant
			view.MachineCheckNote = manjuProductionLegacyMachineCheckNote
		}
	}
	return view
}

func parseProductionStageRuntimeMetadata(inputJSON string) (productionStageRuntimeMetadata, error) {
	if strings.TrimSpace(inputJSON) == "" {
		return productionStageRuntimeMetadata{}, ErrProductionMachineCheckProofInvalid
	}
	var meta productionStageRuntimeMetadata
	if err := json.Unmarshal([]byte(inputJSON), &meta); err != nil {
		return productionStageRuntimeMetadata{}, fmt.Errorf("%w: %v", ErrProductionMachineCheckProofInvalid, err)
	}
	return meta, nil
}

func validateProductionStageMachineCheckProof(stage model.ManjuProductionStage, expectedBaselineArtifactID string) error {
	meta, err := parseProductionStageRuntimeMetadata(stage.InputJSON)
	if err != nil {
		return fmt.Errorf("%w: 机检凭证缺失或格式错误", ErrProductionMachineCheckProofInvalid)
	}
	return validateManjuProductionMachineCheckProof(meta.MachineCheckProof, expectedBaselineArtifactID)
}

func validateManjuProductionMachineCheckProof(proof *ManjuProductionMachineCheckProof, expectedBaselineArtifactID string) error {
	if proof == nil {
		return fmt.Errorf("%w: 机检凭证缺失", ErrProductionMachineCheckProofInvalid)
	}
	checks := []struct {
		name  string
		value string
		want  string
	}{
		{"version", proof.Version, ManjuProductionMachineCheckProofVersion},
		{"pipeline", proof.Pipeline, ManjuProductionMachineCheckPipeline},
		{"compiler", proof.Compiler, ManjuProductionMachineCheckCompiler},
		{"validator", proof.Validator, ManjuProductionMachineCheckValidator},
		{"gate", proof.Gate, ManjuProductionMachineCheckGate},
		{"dialogueDiff", proof.DialogueDiff, ManjuProductionMachineCheckDialogueDiff},
		{"status", proof.Status, model.ManjuProductionMachineCheckStatusPass},
	}
	for _, check := range checks {
		if check.value != check.want {
			return fmt.Errorf("%w: 机检凭证 %s 不匹配", ErrProductionMachineCheckProofInvalid, check.name)
		}
	}
	if proof.Warnings == nil {
		return fmt.Errorf("%w: 机检凭证 warnings 缺失", ErrProductionMachineCheckProofInvalid)
	}
	for _, warning := range proof.Warnings {
		if strings.TrimSpace(warning) == "" {
			return fmt.Errorf("%w: 机检凭证 warnings 含空项", ErrProductionMachineCheckProofInvalid)
		}
	}
	baselineArtifactID := strings.TrimSpace(proof.DialogueBaselineArtifactID)
	if baselineArtifactID == "" {
		return fmt.Errorf("%w: 机检凭证 dialogueBaselineArtifactId 缺失", ErrProductionMachineCheckProofInvalid)
	}
	if expectedBaselineArtifactID != "" && baselineArtifactID != expectedBaselineArtifactID {
		return fmt.Errorf("%w: 机检凭证 dialogueBaselineArtifactId 与当前锁定 P3 不一致", ErrProductionMachineCheckProofInvalid)
	}
	return nil
}
