package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ManualArtifactCorrectionInput struct {
	Content string `json:"content"`
	Note    string `json:"note,omitempty"`
}

type ManjuCompiledSegmentPreview struct {
	SegmentCode   string `json:"segmentCode"`
	TotalChars    int    `json:"totalChars"`
	Characters    int    `json:"characters"`
	SceneProps    int    `json:"sceneProps"`
	Shots         int    `json:"shots"`
	Style         int    `json:"style"`
	BansAndOther  int    `json:"bansAndOther"`
	Body          string `json:"body"`
	OverHardLimit bool   `json:"overHardLimit"`
}

type ManjuCompilePreview struct {
	Model     string                        `json:"model"`
	HardLimit int                           `json:"hardLimit"`
	Passed    bool                          `json:"passed"`
	Issues    []string                      `json:"issues,omitempty"`
	Warnings  []string                      `json:"warnings,omitempty"`
	Feed      string                        `json:"feed"`
	Segments  []ManjuCompiledSegmentPreview `json:"segments"`
}

func (s *Service) ManualCorrectScriptArtifact(
	actorID, projectID, runID, stageKey string,
	input ManualArtifactCorrectionInput,
) (*ScriptRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Content = strings.TrimSpace(input.Content)
	input.Note = strings.TrimSpace(input.Note)
	if input.Content == "" {
		return nil, fmt.Errorf("%w: 修正正文不能为空", ErrInvalidProjectState)
	}
	var manualErr error
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditScript(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuScriptRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrScriptRunNotFound
		}
		stages, err := tx.ManjuScriptStages(runID)
		if err != nil {
			return err
		}
		targetIndex := scriptStageIndex(stages, stageKey)
		if targetIndex < 0 {
			return ErrScriptStageNotFound
		}
		target := &stages[targetIndex]
		if stageKey == ScriptStageSourceRegistration {
			return ErrScriptStageNotReady
		}
		if stageKey == ScriptStageRoundtableReview &&
			(target.Status == model.ManjuScriptStageStatusAwaitingDecision ||
				target.Status == model.ManjuScriptStageStatusLocked) {
			return ErrRoundtableDecisionEndpointRequired
		}
		switch target.Status {
		case model.ManjuScriptStageStatusBlocked,
			model.ManjuScriptStageStatusAwaitingConfirm,
			model.ManjuScriptStageStatusAwaitingDecision,
			model.ManjuScriptStageStatusLocked:
		default:
			return ErrScriptStageNotReady
		}
		if patchStage := scriptProjectStateStageKey(stageKey); patchStage != "" {
			if _, found, err := extractManjuProjectStatePatch(input.Content); err != nil {
				if draftErr := saveFailedManjuStageDraft(
					tx, projectID, "P1", runID, target.ID, stageKey, input.Content,
					"MANUAL_FAILED", validationJSONForError(err.Error()), actorID,
				); draftErr != nil {
					return draftErr
				}
				if blockErr := blockScriptStageRecord(tx, target, err.Error()); blockErr != nil {
					return blockErr
				}
				manualErr = err
				return nil
			} else if found {
				if _, err := previewManjuProjectStatePatch(
					tx, projectID, access.Project.Name, run.LockRevision, patchStage, input.Content, actorID,
				); err != nil {
					message := manjuProjectStateFailure("P1 "+target.Title, err)
					if draftErr := saveFailedManjuStageDraft(
						tx, projectID, "P1", runID, target.ID, stageKey, input.Content,
						"MANUAL_FAILED", validationJSONForError(message), actorID,
					); draftErr != nil {
						return draftErr
					}
					if blockErr := blockScriptStageRecord(tx, target, message); blockErr != nil {
						return blockErr
					}
					manualErr = err
					return nil
				}
			}
		}
		now := time.Now()
		target.Version++
		sum := sha256.Sum256([]byte(input.Content))
		artifact := &model.ManjuScriptArtifact{
			ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: target.ID,
			Kind: scriptArtifactKind(run.Branch, stageKey), Version: target.Version,
			Content: input.Content, ContentHash: hex.EncodeToString(sum[:]),
			CreatedBy: actorID, CreatedAt: now,
		}
		if err := tx.CreateManjuScriptArtifact(artifact); err != nil {
			return err
		}
		if err := recordManjuArtifactAudit(
			tx, projectID, runID, target.ID, artifact.ID, "P1", artifact.Kind,
			artifact.Version, model.ManjuArtifactVisibilityInternal,
			artifact.Content, artifact.ContentHash, "", actorID,
		); err != nil {
			return err
		}
		sourceArtifactID := target.ArtifactID
		target.ArtifactID = artifact.ID
		target.TaskID = ""
		target.Status = model.ManjuScriptStageStatusAwaitingConfirm
		target.Error = ""
		target.ConfirmedBy = ""
		target.ConfirmedAt = nil
		target.CompletedAt = &now
		if err := tx.SaveManjuScriptStage(target); err != nil {
			return err
		}
		if err := markDownstreamScriptStagesStale(tx, runID, target.Position); err != nil {
			return err
		}
		if err := tx.SupersedeManjuConsumersForScriptRun(projectID, runID); err != nil {
			return err
		}
		run.Status = model.ManjuScriptRunStatusActive
		run.CurrentStage = stageKey
		if err := tx.SaveManjuScriptRun(run); err != nil {
			return err
		}
		draft, err := createPassedManualDraft(
			tx, actorID, projectID, "P1", runID, target.ID, input.Content,
			ManjuSkillValidationResult{Passed: true},
		)
		if err != nil {
			return err
		}
		return createManualCorrectionAudit(
			tx, actorID, projectID, "P1", runID, target.ID,
			sourceArtifactID, artifact.ID, draft.ID, artifact.ContentHash,
			ManjuSkillValidationResult{Passed: true}, input.Note, now,
		)
	})
	if err != nil {
		return nil, err
	}
	if manualErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProjectState, manualErr)
	}
	return s.ScriptRunDetail(actorID, projectID, runID)
}

func createPassedManualDraft(
	tx *repository.Repository,
	actorID, projectID, phase, runID, stageID, content string,
	validation ManjuSkillValidationResult,
) (*model.ManjuStageDraft, error) {
	version, err := tx.NextManjuStageDraftVersion(runID, stageID)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(content))
	draft := &model.ManjuStageDraft{
		ID: uuid.NewString(), ProjectID: projectID, Phase: phase,
		RunID: runID, StageID: stageID, Version: version,
		Content: content, ContentHash: hex.EncodeToString(sum[:]),
		ValidationJSON: validationJSONForResult(validation, ""),
		Source:         "MANUAL_PASSED", CreatedBy: actorID, CreatedAt: time.Now(),
	}
	if err := tx.CreateManjuStageDraft(draft); err != nil {
		return nil, err
	}
	return draft, nil
}

func createManualCorrectionAudit(
	tx *repository.Repository,
	actorID, projectID, phase, runID, stageID, sourceArtifactID, resultArtifactID, draftID, contentHash string,
	validation ManjuSkillValidationResult,
	note string,
	now time.Time,
) error {
	validationJSON := validationJSONForResult(validation, "")
	validationSum := sha256.Sum256([]byte(validationJSON))
	return tx.CreateManjuArtifactCorrection(&model.ManjuArtifactCorrection{
		ID: uuid.NewString(), ProjectID: projectID, Phase: phase,
		RunID: runID, StageID: stageID,
		SourceArtifactID: sourceArtifactID, ResultArtifactID: resultArtifactID,
		DraftID: draftID, ContentHash: contentHash,
		ValidationHash: hex.EncodeToString(validationSum[:]),
		Note:           note, CorrectedBy: actorID, CreatedAt: now,
	})
}

func (s *Service) ManualCorrectAssetArtifact(
	actorID, projectID, runID, stageKey string,
	input ManualArtifactCorrectionInput,
) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Content = strings.TrimSpace(input.Content)
	input.Note = strings.TrimSpace(input.Note)
	if input.Content == "" {
		return nil, fmt.Errorf("%w: 修正正文不能为空", ErrInvalidAssetRun)
	}
	var target model.ManjuAssetStage
	var candidateStateJSON string
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditAsset(access.Role) {
			return ErrForbidden
		}
		loadedRun, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if loadedRun == nil {
			return ErrAssetRunNotFound
		}
		loadedStage, err := tx.ManjuAssetStage(runID, stageKey)
		if err != nil {
			return err
		}
		if loadedStage == nil {
			return ErrAssetStageNotFound
		}
		switch loadedStage.Status {
		case model.ManjuAssetStageStatusBlocked,
			model.ManjuAssetStageStatusAwaitingConfirmation,
			model.ManjuAssetStageStatusLocked:
		default:
			return ErrAssetStageNotReady
		}
		loadedReferences, err := requiredAssetReferencesForCompile(tx, *loadedRun, *loadedStage)
		if err != nil {
			return err
		}
		if loadedStage.Key == AssetStageCompile {
			if err := validateAssetDelivery(loadedRun.AssetMode, input.Content, loadedReferences); err != nil {
				return err
			}
			if err := validateRequiredManjuProjectStatePatch("asset:asset_compile", input.Content); err != nil {
				return err
			}
			candidateStateJSON, err = previewManjuProjectStatePatch(
				tx, projectID, access.Project.Name, loadedRun.LockRevision,
				"asset:asset_compile", input.Content, actorID,
			)
			if err != nil {
				return err
			}
		}
		target = *loadedStage
		return nil
	})
	if err != nil {
		s.saveManualAssetFailure(actorID, projectID, runID, stageKey, input.Content, err.Error())
		return nil, fmt.Errorf("%w: %v", ErrInvalidAssetRun, err)
	}
	validation := ManjuSkillValidationResult{Passed: true}
	if target.Key == AssetStageCompile && s.officialSkillValidationEnabled() {
		validation, err = defaultManjuSkillRuntime.ValidateAssetBlocks(input.Content)
		if err != nil || !validation.Passed {
			message := "P2 人工修正未通过官方 check_asset_blocks.py"
			if err != nil {
				message += ": " + err.Error()
			} else if strings.TrimSpace(validation.Output) != "" {
				message += ": " + validation.Output
			}
			s.saveManualAssetFailureWithValidation(actorID, projectID, runID, stageKey, input.Content, validation, message)
			return nil, fmt.Errorf("%w: %s", ErrInvalidAssetRun, message)
		}
	}
	if target.Key == AssetStageCompile && candidateStateJSON == "" {
		return nil, ErrProjectStateUnavailable
	}
	err = s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditAsset(access.Role) {
			return ErrForbidden
		}
		loadedRun, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if loadedRun == nil {
			return ErrAssetRunNotFound
		}
		stages, err := tx.ManjuAssetStages(runID)
		if err != nil {
			return err
		}
		targetIndex := assetStageIndex(stages, stageKey)
		if targetIndex < 0 {
			return ErrAssetStageNotFound
		}
		updated := &stages[targetIndex]
		switch updated.Status {
		case model.ManjuAssetStageStatusBlocked,
			model.ManjuAssetStageStatusAwaitingConfirmation,
			model.ManjuAssetStageStatusLocked:
		default:
			return ErrAssetStageNotReady
		}
		now := time.Now()
		updated.Version++
		sum := sha256.Sum256([]byte(input.Content))
		artifact := &model.ManjuAssetArtifact{
			ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: updated.ID,
			Kind: assetArtifactKind(loadedRun.AssetMode, stageKey), Version: updated.Version,
			Content: input.Content, ContentHash: hex.EncodeToString(sum[:]),
			CreatedBy: actorID, CreatedAt: now,
		}
		if err := tx.CreateManjuAssetArtifact(artifact); err != nil {
			return err
		}
		if err := recordManjuArtifactAudit(
			tx, projectID, runID, updated.ID, artifact.ID, "P2", artifact.Kind,
			artifact.Version, manjuArtifactVisibility("P2", stageKey), artifact.Content, artifact.ContentHash, "", actorID,
		); err != nil {
			return err
		}
		sourceArtifactID := updated.ArtifactID
		updated.ArtifactID = artifact.ID
		updated.TaskID = ""
		updated.Error = ""
		updated.ConfirmedBy = ""
		updated.ConfirmedAt = nil
		updated.CompletedAt = &now
		if stageKey == AssetStageExtraction {
			updated.Status = model.ManjuAssetStageStatusLocked
			updated.ConfirmedBy = actorID
			updated.ConfirmedAt = &now
		} else {
			updated.Status = model.ManjuAssetStageStatusAwaitingConfirmation
		}
		if err := tx.SaveManjuAssetStage(updated); err != nil {
			return err
		}
		for index := targetIndex + 1; index < len(stages); index++ {
			downstream := &stages[index]
			downstream.Status = model.ManjuAssetStageStatusStale
			downstream.TaskID = ""
			downstream.ArtifactID = ""
			downstream.Error = "上游 P2 工件经人工修正，等待重新生成"
			downstream.ConfirmedBy = ""
			downstream.ConfirmedAt = nil
			downstream.CompletedAt = nil
			if err := tx.SaveManjuAssetStage(downstream); err != nil {
				return err
			}
		}
		nextStageKey := stageKey
		if stageKey == AssetStageExtraction {
			next, err := activateNextAssetStage(tx, runID, updated.Position)
			if err != nil {
				return err
			}
			if next != nil {
				nextStageKey = next.Key
			}
		}
		loadedRun.Status = model.ManjuAssetRunStatusActive
		loadedRun.CurrentStage = nextStageKey
		if err := tx.SaveManjuAssetRun(loadedRun); err != nil {
			return err
		}
		draft, err := createPassedManualDraft(
			tx, actorID, projectID, "P2", runID, updated.ID, input.Content, validation,
		)
		if err != nil {
			return err
		}
		return createManualCorrectionAudit(
			tx, actorID, projectID, "P2", runID, updated.ID,
			sourceArtifactID, artifact.ID, draft.ID, artifact.ContentHash,
			validation, input.Note, now,
		)
	})
	if err != nil {
		return nil, err
	}
	return s.AssetRunDetail(actorID, projectID, runID)
}

func (s *Service) saveManualAssetFailure(
	actorID, projectID, runID, stageKey, content, message string,
) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuAssetStage(runID, stageKey)
		if err != nil || stage == nil {
			return err
		}
		if err := saveFailedManjuStageDraft(
			tx, projectID, "P2", runID, stage.ID, stageKey, content,
			"MANUAL_FAILED", validationJSONForError(message), actorID,
		); err != nil {
			return err
		}
		return blockAssetStageRecord(tx, stage, message)
	})
}

func (s *Service) saveManualAssetFailureWithValidation(
	actorID, projectID, runID, stageKey, content string,
	validation ManjuSkillValidationResult,
	message string,
) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuAssetStage(runID, stageKey)
		if err != nil || stage == nil {
			return err
		}
		if err := saveFailedManjuStageDraft(
			tx, projectID, "P2", runID, stage.ID, stageKey, content,
			"MANUAL_FAILED", validationJSONForResult(validation, message), actorID,
		); err != nil {
			return err
		}
		return blockAssetStageRecord(tx, stage, message)
	})
}

func (s *Service) ManualCorrectProductionArtifact(
	actorID, projectID, runID, stageKey string,
	input ManualArtifactCorrectionInput,
) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Content = strings.TrimSpace(input.Content)
	input.Note = strings.TrimSpace(input.Note)
	if input.Content == "" {
		return nil, fmt.Errorf("%w: 修正正文不能为空", ErrProductionArtifactInvalid)
	}
	if stageKey != ProductionStageStoryboard {
		return nil, fmt.Errorf("%w: 当前只允许人工修正 P3；P4 编译错误请回 P3 修正", ErrProductionStageNotReady)
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
		stage, err := tx.ManjuProductionStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrProductionStageNotFound
		}
		switch stage.Status {
		case model.ManjuProductionStageStatusBlocked,
			model.ManjuProductionStageStatusAwaitingConfirmation,
			model.ManjuProductionStageStatusLocked:
		default:
			return ErrProductionStageNotReady
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil || assetRun.Status != model.ManjuAssetRunStatusCompleted {
			return ErrProductionAssetNotReady
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		if err := validateRequiredManjuProjectStatePatch("production:director_storyboard", input.Content); err != nil {
			return err
		}
		candidateStateJSON, err := previewManjuProjectStatePatch(
			tx, projectID, access.Project.Name, run.LockRevision,
			"production:director_storyboard", input.Content, actorID,
		)
		if err != nil {
			return err
		}
		_ = candidateStateJSON
		return nil
	})
	if err != nil {
		s.saveManualProductionFailure(actorID, projectID, runID, stageKey, input.Content, err.Error())
		return nil, fmt.Errorf("%w: %v", ErrProductionArtifactInvalid, err)
	}

	validation := ManjuSkillValidationResult{Passed: true}
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
		if run == nil {
			return ErrProductionRunNotFound
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
		switch target.Status {
		case model.ManjuProductionStageStatusBlocked,
			model.ManjuProductionStageStatusAwaitingConfirmation,
			model.ManjuProductionStageStatusLocked:
		default:
			return ErrProductionStageNotReady
		}
		now := time.Now()
		target.Version++
		sum := sha256.Sum256([]byte(input.Content))
		artifact := &model.ManjuProductionArtifact{
			ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: target.ID,
			Kind: productionArtifactKind(stageKey), Version: target.Version,
			Content: input.Content, ContentHash: hex.EncodeToString(sum[:]),
			CreatedBy: actorID, CreatedAt: now,
		}
		if err := tx.CreateManjuProductionArtifact(artifact); err != nil {
			return err
		}
		phase := "P3"
		if stageKey == ProductionStageFeed {
			phase = "P4"
		} else if stageKey == ProductionStageQC {
			phase = "P5"
		}
		if err := recordManjuArtifactAudit(
			tx, projectID, runID, target.ID, artifact.ID, phase, artifact.Kind,
			artifact.Version, manjuArtifactVisibility(phase, stageKey), artifact.Content, artifact.ContentHash, "", actorID,
		); err != nil {
			return err
		}
		sourceArtifactID := target.ArtifactID
		draft, err := createPassedManualDraft(
			tx, actorID, projectID, "P3", runID, target.ID, input.Content, validation,
		)
		if err != nil {
			return err
		}
		target.ArtifactID = artifact.ID
		target.TaskID = ""
		target.Status = model.ManjuProductionStageStatusAwaitingConfirmation
		target.Error = ""
		target.ConfirmedBy = ""
		target.ConfirmedAt = nil
		target.CompletedAt = &now
		if err := tx.SaveManjuProductionStage(target); err != nil {
			return err
		}
		for index := targetIndex + 1; index < len(stages); index++ {
			downstream := &stages[index]
			downstream.Status = model.ManjuProductionStageStatusNotStarted
			downstream.TaskID = ""
			downstream.ArtifactID = ""
			downstream.Error = "上游 P3 工件经人工修正，等待重新生成"
			downstream.ConfirmedBy = ""
			downstream.ConfirmedAt = nil
			downstream.CompletedAt = nil
			if err := tx.SaveManjuProductionStage(downstream); err != nil {
				return err
			}
		}
		previs, err := tx.ManjuProductionPrevis(projectID, runID)
		if err != nil {
			return err
		}
		if previs != nil {
			previs.Status = model.ManjuProductionPrevisStatusFailed
			previs.Error = "P3 工件经人工修正，请重新生成 P4.5"
			if err := tx.SaveManjuProductionPrevis(previs); err != nil {
				return err
			}
		}
		run.Status = model.ManjuProductionRunStatusActive
		run.CurrentStage = stageKey
		if err := tx.SaveManjuProductionRun(run); err != nil {
			return err
		}
		return createManualCorrectionAudit(
			tx, actorID, projectID, "P3", runID, target.ID,
			sourceArtifactID, artifact.ID, draft.ID, artifact.ContentHash,
			validation, input.Note, now,
		)
	})
	if err != nil {
		return nil, err
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

func (s *Service) PreviewProductionStoryboardCompile(
	actorID, projectID, runID, content string,
) (*ManjuCompilePreview, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrProductionArtifactInvalid
	}
	var candidateStateJSON string
	var validationOptions ManjuSkillValidationOptions
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
		stage, err := tx.ManjuProductionStage(runID, ProductionStageStoryboard)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrProductionStageNotFound
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		candidateStateJSON, err = previewManjuProjectStatePatch(
			tx, projectID, access.Project.Name, run.LockRevision,
			"production:director_storyboard", content, actorID,
		)
		if err != nil {
			return err
		}
		view, err := receiptView(*receipt)
		if err != nil {
			return err
		}
		validationOptions = ManjuSkillValidationOptions{
			Model: view.Snapshot.VideoModel, Track: manjuSkillTrackCode(view.Snapshot.Track),
			Combat: detectManjuCombat(candidateStateJSON), Mode: AssetModeM1Value,
			Aspect: view.Snapshot.AspectRatio, StyleSource: view.Snapshot.StyleCardName,
			StylePromptParameters: view.Snapshot.StylePromptParameters,
			StyleVisualPrompt:     view.Snapshot.StyleVisualPrompt,
			StyleMotionParameters: view.Snapshot.StyleMotionParameters,
			StyleForbiddenItems:   view.Snapshot.StyleForbiddenItems,
			StyleBans:             strings.Join(view.Snapshot.StyleForbiddenItems, "；"),
			Assets:                99, Timeout: 120 * time.Second,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	feed, validation, compileErr := defaultManjuSkillRuntime.CompileFeed(candidateStateJSON, validationOptions)
	if compileErr != nil && strings.TrimSpace(feed) == "" {
		return nil, compileErr
	}
	contract := mustLoadP3StoryboardContract()
	normalizedModel := normalizeManjuSkillModel(validationOptions.Model)
	hardLimit := contract.Budget.HardByModel[normalizedModel]
	preview := &ManjuCompilePreview{
		Model: normalizedModel, HardLimit: hardLimit, Passed: validation.Passed,
		Issues: validation.Issues, Warnings: validation.Warnings, Feed: feed,
		Segments: previewCompiledFeedSegments(feed, hardLimit),
	}
	return preview, nil
}

var compiledUnitHeadingPattern = regexp.MustCompile(`(?m)^##\s*【投喂单元[^\n]*】\s*$`)
var compiledPreviewShotLinePattern = regexp.MustCompile(`^\d{2}:\d{2}-\d{2}:\d{2}`)

func previewCompiledFeedSegments(feed string, hardLimit int) []ManjuCompiledSegmentPreview {
	matches := compiledUnitHeadingPattern.FindAllStringIndex(feed, -1)
	items := make([]ManjuCompiledSegmentPreview, 0, len(matches))
	for index, match := range matches {
		end := len(feed)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		heading := strings.TrimSpace(feed[match[0]:match[1]])
		chunk := feed[match[1]:end]
		body := extractCompiledFeedBody(chunk)
		item := ManjuCompiledSegmentPreview{
			SegmentCode: heading,
			Body:        body,
			TotalChars:  len([]rune(body)),
		}
		item.OverHardLimit = hardLimit > 0 && item.TotalChars > hardLimit
		for _, line := range strings.Split(body, "\n") {
			n := len([]rune(line)) + 1
			switch {
			case strings.HasPrefix(line, "【画幅风格】"),
				strings.HasPrefix(line, "【STYLE"),
				strings.HasPrefix(line, "【PERFORMANCE"):
				item.Style += n
			case strings.HasPrefix(line, "【场景资产】"):
				item.SceneProps += n
			case strings.HasPrefix(line, "【核心人物】"):
				item.Characters += n
			case strings.HasPrefix(line, "【时间轴分镜】"),
				compiledPreviewShotLinePattern.MatchString(line):
				item.Shots += n
			case strings.HasPrefix(line, "【强制禁止项】"),
				strings.HasPrefix(line, "【音效】"),
				strings.HasPrefix(line, "【站位声明】"):
				item.BansAndOther += n
			}
		}
		items = append(items, item)
	}
	return items
}

func extractCompiledFeedBody(chunk string) string {
	start := strings.Index(chunk, "```text")
	if start < 0 {
		return ""
	}
	start += len("```text")
	if newline := strings.IndexByte(chunk[start:], '\n'); newline >= 0 {
		start += newline + 1
	}
	end := strings.Index(chunk[start:], "```")
	if end < 0 {
		return strings.TrimSpace(chunk[start:])
	}
	return strings.TrimSpace(chunk[start : start+end])
}

func (s *Service) saveManualProductionFailure(
	actorID, projectID, runID, stageKey, content, message string,
) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuProductionStage(runID, stageKey)
		if err != nil || stage == nil {
			return err
		}
		if err := saveFailedManjuStageDraft(
			tx, projectID, "P3", runID, stage.ID, stageKey, content,
			"MANUAL_FAILED", validationJSONForError(message), actorID,
		); err != nil {
			return err
		}
		return blockProductionStageRecord(tx, stage, message)
	})
}

func (s *Service) saveManualProductionFailureWithValidation(
	actorID, projectID, runID, stageKey, content string,
	validation ManjuSkillValidationResult,
	message string,
) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuProductionStage(runID, stageKey)
		if err != nil || stage == nil {
			return err
		}
		if err := saveFailedManjuStageDraft(
			tx, projectID, "P3", runID, stage.ID, stageKey, content,
			"MANUAL_FAILED", validationJSONForResult(validation, message), actorID,
		); err != nil {
			return err
		}
		return blockProductionStageRecord(tx, stage, message)
	})
}
