package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AssetModeM0Value = model.ManjuAssetModeM0
	AssetModeM1Value = model.ManjuAssetModeM1
	AssetModeM2Value = model.ManjuAssetModeM2

	AssetStageExtraction = model.ManjuAssetStageExtraction
	AssetStageManifest   = model.ManjuAssetStageManifest
	AssetStageCompile    = model.ManjuAssetStageCompile

	AssetStageStatusNotStarted           = model.ManjuAssetStageStatusNotStarted
	AssetStageStatusInProgress           = model.ManjuAssetStageStatusInProgress
	AssetStageStatusAwaitingConfirmation = model.ManjuAssetStageStatusAwaitingConfirmation
	AssetStageStatusLocked               = model.ManjuAssetStageStatusLocked
	AssetStageStatusStale                = model.ManjuAssetStageStatusStale
	AssetStageStatusBlocked              = model.ManjuAssetStageStatusBlocked
)

var (
	ErrAssetRunNotFound       = errors.New("manju asset run not found")
	ErrAssetStageNotFound     = errors.New("manju asset stage not found")
	ErrAssetScriptNotReady    = errors.New("manju P1 script is not completed and locked")
	ErrAssetStageNotReady     = errors.New("manju asset stage is not ready")
	ErrInvalidAssetRun        = errors.New("invalid manju asset run")
	ErrInvalidAssetReference  = errors.New("invalid manju asset reference")
	ErrAssetReferencesMissing = errors.New("manju M2 asset references are incomplete")
)

type AssetRunInput struct {
	ScriptRunID    string `json:"scriptRunId"`
	AssetMode      string `json:"assetMode,omitempty"`
	BaseAssetRunID string `json:"baseAssetRunId,omitempty"`
	ReuseAssets    bool   `json:"reuseAssets"`
}

type AssetRegenerationInput struct {
	Instruction string `json:"instruction"`
	ReuseAssets *bool  `json:"reuseAssets,omitempty"`
}

type AssetStageRunInput struct {
	Instruction string `json:"instruction"`
}

type AssetConfirmInput struct {
	Note string `json:"note,omitempty"`
}

type AssetReferenceInput struct {
	Code       string `json:"code"`
	AssetType  string `json:"assetType"`
	ResourceID string `json:"resourceId"`
	URL        string `json:"url,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type AssetRunView struct {
	ID                  string    `json:"id"`
	ProjectID           string    `json:"projectId"`
	ScriptRunID         string    `json:"scriptRunId"`
	BaseAssetRunID      string    `json:"baseAssetRunId,omitempty"`
	ReuseAssets         bool      `json:"reuseAssets"`
	InheritedAssetCount int       `json:"inheritedAssetCount"`
	AssetMode           string    `json:"assetMode"`
	Status              string    `json:"status"`
	CurrentStage        string    `json:"currentStage"`
	LockRevision        int64     `json:"lockRevision"`
	CreatedBy           string    `json:"createdBy"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type AssetStageView struct {
	ID          string               `json:"id"`
	Key         string               `json:"key"`
	Title       string               `json:"title"`
	Position    int                  `json:"position"`
	Status      string               `json:"status"`
	Version     int64                `json:"version"`
	TaskID      string               `json:"taskId,omitempty"`
	ArtifactID  string               `json:"artifactId,omitempty"`
	Error       string               `json:"error,omitempty"`
	ConfirmedBy string               `json:"confirmedBy,omitempty"`
	ConfirmedAt *time.Time           `json:"confirmedAt,omitempty"`
	StartedAt   *time.Time           `json:"startedAt,omitempty"`
	CompletedAt *time.Time           `json:"completedAt,omitempty"`
	UpdatedAt   time.Time            `json:"updatedAt"`
	Draft       *ManjuStageDraftView `json:"draft,omitempty"`
}

type AssetArtifactView struct {
	ID          string    `json:"id"`
	StageID     string    `json:"stageId"`
	Kind        string    `json:"kind"`
	Version     int64     `json:"version"`
	Content     string    `json:"content"`
	ContentHash string    `json:"contentHash"`
	TaskID      string    `json:"taskId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AssetReferenceView struct {
	ID         string    `json:"id"`
	Code       string    `json:"code"`
	AssetType  string    `json:"assetType"`
	ResourceID string    `json:"resourceId"`
	URL        string    `json:"url,omitempty"`
	Status     string    `json:"status"`
	Notes      string    `json:"notes,omitempty"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type AssetRunDetail struct {
	Run        AssetRunView         `json:"run"`
	ScriptRun  ScriptRunView        `json:"scriptRun"`
	Stages     []AssetStageView     `json:"stages"`
	Artifacts  []AssetArtifactView  `json:"artifacts,omitempty"`
	References []AssetReferenceView `json:"references,omitempty"`
}

type assetStageDefinition struct {
	Key   string
	Title string
}

var assetCodePattern = regexp.MustCompile(`\b(?:CHR|SCN|PRP|AUD)-[^\s|,，、:：)\]}]+`)

func assetStageDefinitions() []assetStageDefinition {
	return []assetStageDefinition{
		{Key: AssetStageExtraction, Title: "实体提取"},
		{Key: AssetStageManifest, Title: "资产清单"},
		{Key: AssetStageCompile, Title: "资产交付编译"},
	}
}

func (s *Service) CreateAssetRun(actorID, projectID string, input AssetRunInput) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	input.ScriptRunID = strings.TrimSpace(input.ScriptRunID)
	input.AssetMode = strings.ToUpper(strings.TrimSpace(input.AssetMode))
	input.BaseAssetRunID = strings.TrimSpace(input.BaseAssetRunID)
	if input.ScriptRunID == "" {
		return nil, ErrInvalidAssetRun
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
		if !canEditAsset(access.Role) {
			return ErrForbidden
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked {
			return ErrP0Required
		}
		lock, err := receiptView(*receipt)
		if err != nil {
			return err
		}
		scriptRun, err := tx.ManjuScriptRun(projectID, input.ScriptRunID)
		if err != nil {
			return err
		}
		if scriptRun == nil ||
			scriptRun.Status != model.ManjuScriptRunStatusCompleted ||
			scriptRun.LockRevision != receipt.Revision {
			return ErrAssetScriptNotReady
		}
		assetMode := strings.ToUpper(strings.TrimSpace(lock.Snapshot.AssetMode))
		if assetMode == "" {
			assetMode = AssetModeM1Value
		}
		if input.AssetMode != "" {
			assetMode = input.AssetMode
		}
		if !assetModeValues[assetMode] {
			return ErrInvalidAssetRun
		}
		baseAssetRunID := input.BaseAssetRunID
		inheritedAssetCount := 0
		if input.ReuseAssets {
			var baseRun *model.ManjuAssetRun
			runs, err := tx.ManjuAssetRuns(projectID)
			if err != nil {
				return err
			}
			for _, candidate := range runs {
				if baseAssetRunID != "" && candidate.ID != baseAssetRunID {
					continue
				}
				if candidate.Status == "COMPLETED" &&
					candidate.LockRevision == receipt.Revision &&
					candidate.AssetMode == assetMode {
					baseAssetRunID = candidate.ID
					baseRun = &candidate
					break
				}
			}
			if baseRun == nil ||
				baseRun.Status != "COMPLETED" ||
				baseRun.LockRevision != receipt.Revision ||
				baseRun.AssetMode != assetMode {
				return ErrInvalidAssetRun
			}
			var referenceCount int64
			if err := tx.DB().Table("manju_asset_references").Where("run_id = ?", baseAssetRunID).Count(&referenceCount).Error; err != nil {
				return err
			}
			inheritedAssetCount = int(referenceCount)
		}
		if err := tx.SupersedeActiveManjuAssetRuns(projectID); err != nil {
			return err
		}

		now := time.Now()
		run := &model.ManjuAssetRun{
			ID:                  uuid.NewString(),
			ProjectID:           projectID,
			ScriptRunID:         scriptRun.ID,
			BaseAssetRunID:      baseAssetRunID,
			ReuseAssets:         input.ReuseAssets,
			InheritedAssetCount: inheritedAssetCount,
			AssetMode:           assetMode,
			Status:              model.ManjuAssetRunStatusActive,
			CurrentStage:        AssetStageExtraction,
			LockRevision:        receipt.Revision,
			CreatedBy:           actorID,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := tx.CreateManjuAssetRun(run); err != nil {
			return err
		}
		stages := make([]model.ManjuAssetStage, 0, len(assetStageDefinitions()))
		for index, definition := range assetStageDefinitions() {
			status := model.ManjuAssetStageStatusNotStarted
			var startedAt *time.Time
			if index == 0 {
				status = model.ManjuAssetStageStatusInProgress
				startedAt = &now
			}
			stages = append(stages, model.ManjuAssetStage{
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
		if err := tx.CreateManjuAssetStages(stages); err != nil {
			return err
		}
		if input.ReuseAssets && baseAssetRunID != "" {
			type inheritedReference struct {
				Code       string
				AssetType  string
				ResourceID string
				URL        string
				Status     string
				Notes      string
			}
			var references []inheritedReference
			if err := tx.DB().Table("manju_asset_references").Where("run_id = ?", baseAssetRunID).Find(&references).Error; err != nil {
				return err
			}
			for _, reference := range references {
				if err := tx.DB().Exec(
					"INSERT INTO manju_asset_references (id, project_id, run_id, code, asset_type, resource_id, url, status, notes, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					uuid.NewString(), projectID, run.ID, reference.Code, reference.AssetType,
					reference.ResourceID, reference.URL, reference.Status, reference.Notes, actorID, now, now,
				).Error; err != nil {
					return err
				}
			}
		}
		runID = run.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.AssetRunDetail(actorID, projectID, runID)
}

// RegenerateAssetRun keeps the previous P2 run as history and starts a fresh
// extraction task. Reusing the completed asset library is the default because
// it preserves already uploaded references across runs.
func (s *Service) RegenerateAssetRun(actorID, projectID, runID string, input AssetRegenerationInput) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	input.Instruction = strings.TrimSpace(input.Instruction)

	var baseRun model.ManjuAssetRun
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
		run, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrAssetRunNotFound
		}
		if run.Status != model.ManjuAssetRunStatusCompleted {
			return ErrInvalidAssetRun
		}
		baseRun = *run
		return nil
	})
	if err != nil {
		return nil, err
	}

	reuseAssets := true
	if input.ReuseAssets != nil {
		reuseAssets = *input.ReuseAssets
	}
	baseAssetRunID := ""
	if reuseAssets {
		baseAssetRunID = baseRun.ID
	}
	detail, err := s.CreateAssetRun(actorID, projectID, AssetRunInput{
		ScriptRunID:    baseRun.ScriptRunID,
		AssetMode:      baseRun.AssetMode,
		BaseAssetRunID: baseAssetRunID,
		ReuseAssets:    reuseAssets,
	})
	if err != nil {
		return nil, err
	}
	return s.RunAssetStage(actorID, projectID, detail.Run.ID, AssetStageExtraction, AssetStageRunInput{
		Instruction: input.Instruction,
	})
}

// RegenerateAssetArtifact reruns one P2 stage artifact in the existing run.
// Locked upstream artifacts and historical records are preserved.
func (s *Service) RegenerateAssetArtifact(actorID, projectID, runID, stageKey string, input AssetStageRunInput) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Instruction = strings.TrimSpace(input.Instruction)
	if stageKey != AssetStageExtraction && stageKey != AssetStageManifest && stageKey != AssetStageCompile {
		return nil, ErrAssetStageNotFound
	}

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
		run, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrAssetRunNotFound
		}
		if run.Status != model.ManjuAssetRunStatusActive &&
			run.Status != model.ManjuAssetRunStatusCompleted {
			return ErrInvalidAssetRun
		}
		stages, err := tx.ManjuAssetStages(runID)
		if err != nil {
			return err
		}
		targetIndex := assetStageIndex(stages, stageKey)
		if targetIndex < 0 {
			return ErrAssetStageNotFound
		}
		target := &stages[targetIndex]
		if strings.TrimSpace(target.ArtifactID) == "" {
			return ErrAssetStageNotReady
		}
		switch target.Status {
		case model.ManjuAssetStageStatusLocked,
			model.ManjuAssetStageStatusAwaitingConfirmation,
			model.ManjuAssetStageStatusBlocked:
		default:
			return ErrAssetStageNotReady
		}
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
				return ErrAssetStageNotReady
			}
		}

		target.Status = model.ManjuAssetStageStatusInProgress
		target.TaskID = ""
		target.ArtifactID = ""
		target.Error = ""
		target.ConfirmedBy = ""
		target.ConfirmedAt = nil
		target.CompletedAt = nil
		target.InputJSON = ""
		if err := tx.SaveManjuAssetStage(target); err != nil {
			return err
		}
		for index := targetIndex + 1; index < len(stages); index++ {
			downstream := &stages[index]
			downstream.Status = model.ManjuAssetStageStatusStale
			downstream.TaskID = ""
			downstream.ArtifactID = ""
			downstream.Error = "上游工件已重新生成，等待新工件"
			downstream.ConfirmedBy = ""
			downstream.ConfirmedAt = nil
			downstream.CompletedAt = nil
			downstream.InputJSON = ""
			if err := tx.SaveManjuAssetStage(downstream); err != nil {
				return err
			}
		}
		if err := tx.SupersedeManjuProductionRunsForAssetRun(projectID, runID); err != nil {
			return err
		}
		run.Status = model.ManjuAssetRunStatusActive
		run.CurrentStage = stageKey
		if err := tx.SaveManjuAssetRun(run); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.RunAssetStage(actorID, projectID, runID, stageKey, input)
}

func (s *Service) AssetRuns(actorID, projectID string) ([]AssetRunView, error) {
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
	runs, err := s.repo.ManjuAssetRuns(projectID)
	if err != nil {
		return nil, err
	}
	items := make([]AssetRunView, 0, len(runs))
	for _, run := range runs {
		items = append(items, assetRunView(run))
	}
	return items, nil
}

func (s *Service) RunAssetStage(actorID, projectID, runID, stageKey string, input AssetStageRunInput) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Instruction = strings.TrimSpace(input.Instruction)
	if s.scriptTaskCreate == nil {
		return nil, ErrScriptTaskUnavailable
	}
	if stageKey != AssetStageExtraction && stageKey != AssetStageManifest && stageKey != AssetStageCompile {
		return nil, ErrAssetStageNotFound
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
		if !canEditAsset(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrAssetRunNotFound
		}
		if run.Status != model.ManjuAssetRunStatusActive {
			return ErrInvalidAssetRun
		}
		stage, err := tx.ManjuAssetStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrAssetStageNotFound
		}
		if strings.TrimSpace(stage.TaskID) != "" {
			existingTaskID = stage.TaskID
			return nil
		}
		if stage.Status != model.ManjuAssetStageStatusInProgress &&
			stage.Status != model.ManjuAssetStageStatusBlocked &&
			stage.Status != model.ManjuAssetStageStatusStale {
			return ErrAssetStageNotReady
		}
		stages, err := tx.ManjuAssetStages(runID)
		if err != nil {
			return err
		}
		if err := ensureAssetStageGate(stages, stageKey); err != nil {
			return err
		}
		if stage.Key == AssetStageCompile && run.AssetMode == AssetModeM2Value {
			required, err := requiredAssetReferencesForCompile(tx, *run, *stage)
			if err != nil {
				return err
			}
			missing := make([]string, 0, len(required))
			for _, reference := range required {
				if strings.TrimSpace(reference.ResourceID) == "" {
					missing = append(missing, reference.Code)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("%w: %s", ErrAssetReferencesMissing, strings.Join(missing, "、"))
			}
		}
		binding, err := tx.ManjuProjectModelBinding(projectID)
		if err != nil {
			return err
		}
		if binding == nil || strings.TrimSpace(binding.TextChannelID) == "" || strings.TrimSpace(binding.TextModelKey) == "" {
			return ErrInvalidModelBinding
		}
		scriptRun, err := tx.ManjuScriptRun(projectID, run.ScriptRunID)
		if err != nil {
			return err
		}
		if scriptRun == nil || scriptRun.Status != model.ManjuScriptRunStatusCompleted || scriptRun.LockRevision != run.LockRevision {
			return ErrAssetScriptNotReady
		}
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Revision != run.LockRevision || receipt.Status != model.ManjuLockStatusLocked {
			return ErrAssetScriptNotReady
		}
		scriptArtifacts, err := lockedScriptArtifacts(tx, *scriptRun)
		if err != nil {
			return err
		}
		assetArtifacts, err := lockedAssetArtifacts(tx, *run)
		if err != nil {
			return err
		}
		assetReferences, err := tx.ManjuAssetReferences(runID)
		if err != nil {
			return err
		}
		if stage.Key == AssetStageExtraction && run.ReuseAssets && strings.TrimSpace(run.BaseAssetRunID) != "" {
			baseRun, err := tx.ManjuAssetRun(projectID, run.BaseAssetRunID)
			if err != nil {
				return err
			}
			if baseRun == nil {
				return ErrAssetRunNotFound
			}
			baseArtifacts, err := lockedAssetArtifacts(tx, *baseRun)
			if err != nil {
				return err
			}
			if inheritedPrompt := inheritedAssetBaselinePrompt(baseArtifacts); inheritedPrompt != "" {
				if input.Instruction != "" {
					input.Instruction += "\n\n"
				}
				input.Instruction += inheritedPrompt
			}
		}
		ruleBundle, err := defaultManjuSkillRuntime.StageRuleDispatch(manjuSkillStageOptions("P2", receipt, run.AssetMode, "none"))
		if err != nil {
			return err
		}
		skillContext := ruleBundle.Content
		prompt := buildAssetStagePrompt(access.Project.Name, *run, *stage, *scriptRun, receipt, scriptArtifacts, assetArtifacts, assetReferences, input.Instruction, skillContext)
		operationKey = fmt.Sprintf("manju:%s:%s:v%d", run.ID, stage.Key, stage.Version+1)
		ruleSnapshot, err := s.ensureManjuRuleSnapshot(tx, projectID, run.LockRevision, actorID)
		if err != nil {
			return err
		}
		exchangeID := uuid.NewString()
		request = localtask.CreateRequest{
			ProjectID: projectID,
			Type:      "canvas_text",
			Operation: "manju_asset_stage",
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
					"clientOperationId":    operationKey,
					"manjuProjectId":       projectID,
					"manjuAssetRunId":      run.ID,
					"manjuAssetStageKey":   stage.Key,
					"manjuAssetStageTitle": stage.Title,
					"manjuAssetMode":       run.AssetMode,
					"manjuRuleSnapshot":    ruleSnapshot.ID,
					"manjuModelExchange":   exchangeID,
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
		if err := s.recordManjuRuleReads(tx, ruleSnapshot, run.ID, stage.ID, "P2", operationKey, ruleBundle); err != nil {
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
		stage.Status = model.ManjuAssetStageStatusInProgress
		stage.Error = ""
		return tx.SaveManjuAssetStage(stage)
	})
	if err != nil {
		return nil, err
	}
	if existingTaskID != "" {
		return s.AssetRunDetail(actorID, projectID, runID)
	}
	task, err := s.scriptTaskCreate(actorID, request)
	if err != nil {
		s.blockAssetStage(projectID, runID, stageKey, err.Error())
		return nil, err
	}
	if task == nil || strings.TrimSpace(task.ID) == "" {
		err := errors.New("文本任务服务没有返回任务 ID")
		s.blockAssetStage(projectID, runID, stageKey, err.Error())
		return nil, err
	}
	if err := s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuAssetStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrAssetStageNotFound
		}
		if strings.TrimSpace(stage.TaskID) == "" {
			stage.TaskID = task.ID
		}
		stage.Status = model.ManjuAssetStageStatusInProgress
		stage.Error = ""
		if err := tx.AttachManjuModelExchangeTask(operationKey, task.ID); err != nil {
			return err
		}
		return tx.SaveManjuAssetStage(stage)
	}); err != nil {
		return nil, err
	}
	return s.AssetRunDetail(actorID, projectID, runID)
}

func (s *Service) blockAssetStage(projectID, runID, stageKey, message string) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuAssetStage(runID, stageKey)
		if err != nil || stage == nil {
			return err
		}
		taskID := stage.TaskID
		stage.Status = model.ManjuAssetStageStatusBlocked
		stage.Error = strings.TrimSpace(message)
		stage.TaskID = ""
		stage.Version++
		if err := tx.CompleteManjuModelExchangeByTaskID(
			taskID, "", "", model.ManjuModelExchangeStatusFailed, message,
		); err != nil {
			return err
		}
		return tx.SaveManjuAssetStage(stage)
	})
}

func ensureAssetStageGate(stages []model.ManjuAssetStage, stageKey string) error {
	index := assetStageIndex(stages, stageKey)
	if index < 0 {
		return ErrAssetStageNotFound
	}
	if index == 0 {
		return nil
	}
	previous := stages[index-1]
	if previous.Status != model.ManjuAssetStageStatusLocked || strings.TrimSpace(previous.ArtifactID) == "" {
		return ErrAssetStageNotReady
	}
	return nil
}

func (s *Service) ConfirmAssetManifest(actorID, projectID, runID string) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canConfirmAsset(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrAssetRunNotFound
		}
		if run.Status != model.ManjuAssetRunStatusActive {
			return ErrAssetStageNotReady
		}
		stage, err := tx.ManjuAssetStage(runID, AssetStageManifest)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrAssetStageNotFound
		}
		if stage.Status == model.ManjuAssetStageStatusLocked {
			return nil
		}
		if stage.Status != model.ManjuAssetStageStatusAwaitingConfirmation || strings.TrimSpace(stage.ArtifactID) == "" {
			return ErrAssetStageNotReady
		}
		now := time.Now()
		stage.Status = model.ManjuAssetStageStatusLocked
		stage.ConfirmedBy = actorID
		stage.ConfirmedAt = &now
		stage.Error = ""
		if err := tx.SaveManjuAssetStage(stage); err != nil {
			return err
		}
		compile, err := tx.ManjuAssetStage(runID, AssetStageCompile)
		if err != nil {
			return err
		}
		if compile == nil {
			return ErrAssetStageNotFound
		}
		if compile.Status == model.ManjuAssetStageStatusNotStarted ||
			compile.Status == model.ManjuAssetStageStatusBlocked ||
			compile.Status == model.ManjuAssetStageStatusStale {
			compile.Status = model.ManjuAssetStageStatusInProgress
			compile.StartedAt = &now
			compile.Error = ""
			if err := tx.SaveManjuAssetStage(compile); err != nil {
				return err
			}
		}
		run.CurrentStage = AssetStageCompile
		return tx.SaveManjuAssetRun(run)
	})
	if err != nil {
		return nil, err
	}
	return s.AssetRunDetail(actorID, projectID, runID)
}

func (s *Service) ConfirmAssetRun(actorID, projectID, runID string, _ AssetConfirmInput) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canConfirmAsset(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrAssetRunNotFound
		}
		if run.Status == model.ManjuAssetRunStatusCompleted {
			return nil
		}
		if run.Status != model.ManjuAssetRunStatusActive {
			return ErrAssetStageNotReady
		}
		stage, err := tx.ManjuAssetStage(runID, AssetStageCompile)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrAssetStageNotFound
		}
		if stage.Status != model.ManjuAssetStageStatusAwaitingConfirmation || strings.TrimSpace(stage.ArtifactID) == "" {
			return ErrAssetStageNotReady
		}
		content, err := assetArtifactContentByID(tx, runID, stage.ArtifactID)
		if err != nil {
			return err
		}
		if err := validateCompiledAssetArtifact(tx, *run, *stage); err != nil {
			return err
		}
		if s.officialSkillValidationEnabled() {
			if err := commitManjuProjectStatePatch(tx, projectID, access.Project.Name, run.LockRevision, "asset:asset_compile", stage.ArtifactID, content, actorID, true); err != nil {
				return err
			}
		}
		now := time.Now()
		stage.Status = model.ManjuAssetStageStatusLocked
		stage.ConfirmedBy = actorID
		stage.ConfirmedAt = &now
		stage.Error = ""
		if err := tx.SaveManjuAssetStage(stage); err != nil {
			return err
		}
		run.Status = model.ManjuAssetRunStatusCompleted
		run.CurrentStage = ""
		return tx.SaveManjuAssetRun(run)
	})
	if err != nil {
		return nil, err
	}
	return s.AssetRunDetail(actorID, projectID, runID)
}

func (s *Service) RegisterAssetReference(actorID, projectID, runID string, input AssetReferenceInput) (*AssetRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	input.Code = strings.TrimSpace(input.Code)
	input.AssetType = strings.ToUpper(strings.TrimSpace(input.AssetType))
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.URL = strings.TrimSpace(input.URL)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.Code == "" || input.ResourceID == "" {
		return nil, ErrInvalidAssetReference
	}
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
		run, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrAssetRunNotFound
		}
		if run.AssetMode != AssetModeM2Value || run.Status != model.ManjuAssetRunStatusActive {
			return ErrInvalidAssetReference
		}
		reference := &model.ManjuAssetReference{
			ID:         uuid.NewString(),
			ProjectID:  projectID,
			RunID:      runID,
			Code:       input.Code,
			AssetType:  input.AssetType,
			ResourceID: input.ResourceID,
			URL:        input.URL,
			Status:     model.ManjuAssetReferenceStatusReady,
			Notes:      input.Notes,
			CreatedBy:  actorID,
		}
		return tx.UpsertManjuAssetReference(reference)
	})
	if err != nil {
		return nil, err
	}
	return s.AssetRunDetail(actorID, projectID, runID)
}

func (s *Service) AssetRunDetail(actorID, projectID, runID string) (*AssetRunDetail, error) {
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
	run, err := s.repo.ManjuAssetRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrAssetRunNotFound
	}
	if err := s.reconcileAssetStageTasks(actorID, projectID, runID); err != nil {
		return nil, err
	}
	run, err = s.repo.ManjuAssetRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrAssetRunNotFound
	}
	scriptRun, err := s.repo.ManjuScriptRun(projectID, run.ScriptRunID)
	if err != nil {
		return nil, err
	}
	if scriptRun == nil {
		return nil, ErrAssetScriptNotReady
	}
	stages, err := s.repo.ManjuAssetStages(runID)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.repo.ManjuAssetArtifacts(runID)
	if err != nil {
		return nil, err
	}
	references, err := s.repo.ManjuAssetReferences(runID)
	if err != nil {
		return nil, err
	}
	detail := &AssetRunDetail{
		Run:        assetRunView(*run),
		ScriptRun:  scriptRunView(*scriptRun),
		Stages:     make([]AssetStageView, 0, len(stages)),
		Artifacts:  make([]AssetArtifactView, 0, len(artifacts)),
		References: make([]AssetReferenceView, 0, len(references)),
	}
	for _, stage := range stages {
		view := assetStageView(stage)
		if stage.Status == model.ManjuAssetStageStatusBlocked {
			draft, draftErr := s.repo.LatestManjuStageDraft(runID, stage.ID)
			if draftErr != nil {
				return nil, draftErr
			}
			view.Draft = stageDraftView(draft)
		}
		detail.Stages = append(detail.Stages, view)
	}
	for _, artifact := range artifacts {
		detail.Artifacts = append(detail.Artifacts, AssetArtifactView{
			ID: artifact.ID, StageID: artifact.StageID, Kind: artifact.Kind,
			Version: artifact.Version, Content: artifact.Content, ContentHash: artifact.ContentHash,
			TaskID: artifact.TaskID, CreatedAt: artifact.CreatedAt,
		})
	}
	for _, reference := range references {
		detail.References = append(detail.References, AssetReferenceView{
			ID: reference.ID, Code: reference.Code, AssetType: reference.AssetType,
			ResourceID: reference.ResourceID, URL: reference.URL, Status: reference.Status,
			Notes: reference.Notes, CreatedBy: reference.CreatedBy, CreatedAt: reference.CreatedAt,
			UpdatedAt: reference.UpdatedAt,
		})
	}
	return detail, nil
}

func (s *Service) reconcileAssetStageTasks(actorID, projectID, runID string) error {
	return s.repo.Transaction(func(tx *repository.Repository) error {
		run, err := tx.ManjuAssetRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrAssetRunNotFound
		}
		stages, err := tx.ManjuAssetStages(runID)
		if err != nil {
			return err
		}
		for index := range stages {
			stage := &stages[index]
			if strings.TrimSpace(stage.TaskID) == "" || stage.Status != model.ManjuAssetStageStatusInProgress {
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
				stage.Status = model.ManjuAssetStageStatusBlocked
				stage.Error = strings.TrimSpace(task.Error)
				if stage.Error == "" {
					stage.Error = "文本任务未成功完成"
				}
				stage.TaskID = ""
				stage.Version++
				if err := tx.SaveManjuAssetStage(stage); err != nil {
					return err
				}
			case model.TaskStatusSucceeded:
				if strings.TrimSpace(stage.ArtifactID) != "" {
					continue
				}
				content, err := scriptTaskResultText(task.ResultJSON)
				if err != nil || strings.TrimSpace(content) == "" {
					if err := blockAssetStageRecord(tx, stage, "文本任务成功但没有可读取的正文"); err != nil {
						return err
					}
					continue
				}
				references, err := requiredAssetReferencesForCompile(tx, *run, *stage)
				if err != nil {
					return err
				}
				if stage.Key == AssetStageCompile {
					if err := validateAssetDelivery(run.AssetMode, content, references); err != nil {
						if draftErr := saveFailedManjuStageDraft(
							tx, projectID, "P2", runID, stage.ID, stage.Key, content,
							"MODEL_FAILED", validationJSONForError(err.Error()), actorID,
						); draftErr != nil {
							return draftErr
						}
						if blockErr := blockAssetStageRecord(tx, stage, err.Error()); blockErr != nil {
							return blockErr
						}
						continue
					}
					if s.officialSkillValidationEnabled() {
						if err := validateRequiredManjuProjectStatePatch("asset:asset_compile", content); err != nil {
							message := manjuProjectStateFailure("P2 资产交付", err)
							if draftErr := saveFailedManjuStageDraft(
								tx, projectID, "P2", runID, stage.ID, stage.Key, content,
								"MODEL_FAILED", validationJSONForError(message), actorID,
							); draftErr != nil {
								return draftErr
							}
							if blockErr := blockAssetStageRecord(tx, stage, message); blockErr != nil {
								return blockErr
							}
							continue
						}
						validation, validationErr := defaultManjuSkillRuntime.ValidateAssetBlocks(content)
						if validationErr != nil || !validation.Passed {
							message := "V7.4.26 官方 check_asset_blocks.py 未通过"
							if validationErr != nil {
								message += ": " + validationErr.Error()
							} else if strings.TrimSpace(validation.Output) != "" {
								message += ": " + truncateScriptText(validation.Output, 6000)
							}
							if draftErr := saveFailedManjuStageDraft(
								tx, projectID, "P2", runID, stage.ID, stage.Key, content,
								"MODEL_FAILED", validationJSONForResult(validation, message), actorID,
							); draftErr != nil {
								return draftErr
							}
							if blockErr := blockAssetStageRecord(tx, stage, message); blockErr != nil {
								return blockErr
							}
							continue
						}
					}
				}
				now := time.Now()
				stage.Version++
				sum := sha256.Sum256([]byte(content))
				artifact := &model.ManjuAssetArtifact{
					ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
					Kind: assetArtifactKind(run.AssetMode, stage.Key), Version: stage.Version,
					Content: content, ContentHash: hex.EncodeToString(sum[:]), TaskID: task.ID,
					CreatedBy: actorID, CreatedAt: now,
				}
				if err := tx.CreateManjuAssetArtifact(artifact); err != nil {
					return err
				}
				if err := recordManjuArtifactAudit(
					tx, projectID, runID, stage.ID, artifact.ID, "P2", artifact.Kind,
					artifact.Version, manjuArtifactVisibility("P2", stage.Key), artifact.Content, artifact.ContentHash,
					artifact.TaskID, actorID,
				); err != nil {
					return err
				}
				stage.ArtifactID = artifact.ID
				stage.CompletedAt = &now
				stage.Error = ""
				switch stage.Key {
				case AssetStageExtraction:
					stage.Status = model.ManjuAssetStageStatusLocked
					stage.ConfirmedBy = actorID
					stage.ConfirmedAt = &now
					if err := tx.SaveManjuAssetStage(stage); err != nil {
						return err
					}
					next, err := activateNextAssetStage(tx, runID, stage.Position)
					if err != nil {
						return err
					}
					if next != nil {
						run.CurrentStage = next.Key
					}
				case AssetStageManifest:
					stage.Status = model.ManjuAssetStageStatusAwaitingConfirmation
					run.CurrentStage = AssetStageManifest
					if err := tx.SaveManjuAssetStage(stage); err != nil {
						return err
					}
				case AssetStageCompile:
					stage.Status = model.ManjuAssetStageStatusAwaitingConfirmation
					run.CurrentStage = AssetStageCompile
					if err := tx.SaveManjuAssetStage(stage); err != nil {
						return err
					}
				}
				if err := tx.SaveManjuAssetRun(run); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func blockAssetStageRecord(tx *repository.Repository, stage *model.ManjuAssetStage, message string) error {
	taskID := stage.TaskID
	stage.Status = model.ManjuAssetStageStatusBlocked
	stage.Error = strings.TrimSpace(message)
	stage.TaskID = ""
	stage.Version++
	if err := tx.CompleteManjuModelExchangeByTaskID(
		taskID, "", "", model.ManjuModelExchangeStatusFailed, message,
	); err != nil {
		return err
	}
	return tx.SaveManjuAssetStage(stage)
}

func activateNextAssetStage(tx *repository.Repository, runID string, position int) (*model.ManjuAssetStage, error) {
	now := time.Now()
	stages, err := tx.ManjuAssetStages(runID)
	if err != nil {
		return nil, err
	}
	for index := range stages {
		if stages[index].Position != position+1 {
			continue
		}
		if stages[index].Status == model.ManjuAssetStageStatusNotStarted ||
			stages[index].Status == model.ManjuAssetStageStatusBlocked ||
			stages[index].Status == model.ManjuAssetStageStatusStale {
			stages[index].Status = model.ManjuAssetStageStatusInProgress
			stages[index].StartedAt = &now
			stages[index].Error = ""
			if err := tx.SaveManjuAssetStage(&stages[index]); err != nil {
				return nil, err
			}
		}
		return &stages[index], nil
	}
	return nil, nil
}

func lockedScriptArtifacts(tx *repository.Repository, scriptRun model.ManjuScriptRun) ([]model.ManjuScriptArtifact, error) {
	stages, err := tx.ManjuScriptStages(scriptRun.ID)
	if err != nil {
		return nil, err
	}
	artifacts, err := tx.ManjuScriptArtifacts(scriptRun.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.ManjuScriptArtifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	result := make([]model.ManjuScriptArtifact, 0, len(stages))
	for _, stage := range stages {
		if stage.Status != model.ManjuScriptStageStatusLocked || strings.TrimSpace(stage.ArtifactID) == "" {
			continue
		}
		if artifact, ok := byID[stage.ArtifactID]; ok {
			result = append(result, artifact)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func buildAssetStagePrompt(projectName string, run model.ManjuAssetRun, stage model.ManjuAssetStage, scriptRun model.ManjuScriptRun, receipt *model.ManjuLockReceipt, scriptArtifacts []model.ManjuScriptArtifact, assetArtifacts []model.ManjuAssetArtifact, assetReferences []model.ManjuAssetReference, instruction, skillContext string) string {
	var out strings.Builder
	out.WriteString("你是 VisiDeep AI 漫剧工作室的 P2 数字资产执行器，严格遵循漫剧老李 AIGC 全流程 V7.4.26。\n")
	out.WriteString("只输出本阶段 Markdown 成果，不输出思考过程。不得伪造用户确认、图像核验、PASS、图号或不存在的事实。\n")
	out.WriteString("P2 默认只产出文字档案、提示词与接线单，不调用图片模型。M0/M1 缺图不构成阻断，也不得向用户索要图片作为继续条件。\n\n")
	out.WriteString("P0 画风只能使用可投喂视觉参数；交付稿不得出现导演名、工作室名、影片名或选择层卡号。\n\n")
	fmt.Fprintf(&out, "【项目】%s\n【素材形态】%s\n【P2 阶段】%s（%s）\n【P1 剧本任务】%s / 支线 %s / 模式 %s\n\n", projectName, run.AssetMode, stage.Title, stage.Key, scriptRun.ID, scriptRun.Branch, scriptRun.Mode)
	if receipt != nil {
		fmt.Fprintf(&out, "%s\n", p0DeliveryPrompt(receipt))
	}
	if len(scriptArtifacts) > 0 || len(assetArtifacts) > 0 {
		appendLockedArtifactAuthority(&out, "P1/P2 上游")
	}
	if len(scriptArtifacts) > 0 {
		out.WriteString("【P1 已锁定且可交接的工件】\n")
		for _, artifact := range scriptArtifacts {
			fmt.Fprintf(&out, "--- %s v%d ---\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
		}
		out.WriteString("\n")
	}
	if len(assetArtifacts) > 0 {
		out.WriteString("【P2 已生成的上游工件】\n")
		for _, artifact := range assetArtifacts {
			fmt.Fprintf(&out, "--- %s v%d ---\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
		}
		out.WriteString("\n")
	}
	if run.AssetMode == AssetModeM2Value {
		if references := assetReferenceViews(assetReferences); len(references) > 0 {
			out.WriteString("【已登记的 M2 参考资产】\n")
			for _, reference := range references {
				fmt.Fprintf(&out, "- %s | %s | %s\n", reference.Code, reference.ResourceID, reference.URL)
			}
			out.WriteString("\n")
		}
	}
	if instruction != "" {
		fmt.Fprintf(&out, "【本次用户补充】\n%s\n\n", instruction)
	}
	appendManjuSkillContext(&out, skillContext)
	out.WriteString("\n\n【BeefTV 运行接口】\n")
	out.WriteString(assetStageInstruction(run.AssetMode, stage.Key))
	if stage.Key == AssetStageCompile {
		out.WriteString(projectStatePatchInstruction("P2"))
	}
	return out.String()
}

func inheritedAssetBaselinePrompt(artifacts []model.ManjuAssetArtifact) string {
	structured := make([]model.ManjuAssetArtifact, 0, 2)
	for _, artifact := range artifacts {
		switch artifact.Kind {
		case model.ManjuAssetArtifactExtraction, model.ManjuAssetArtifactManifest:
			structured = append(structured, artifact)
		}
	}
	if len(structured) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("【继承资产基线】保留以下已完成单元中的资产编码和已确认设定；只补充本单元新增或确有变化的实体，不得重新编号已有资产。不得继承旧版编译交付稿中的资产块、图号、接线单或机器块。\n")
	for _, artifact := range structured {
		fmt.Fprintf(&out, "\n--- %s v%d ---\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
	}
	return out.String()
}

func assetReferenceViews(references []model.ManjuAssetReference) []AssetReferenceView {
	result := make([]AssetReferenceView, 0, len(references))
	for _, reference := range references {
		result = append(result, AssetReferenceView{
			ID: reference.ID, Code: reference.Code, AssetType: reference.AssetType,
			ResourceID: reference.ResourceID, URL: reference.URL, Status: reference.Status,
			Notes: reference.Notes, CreatedBy: reference.CreatedBy,
			CreatedAt: reference.CreatedAt, UpdatedAt: reference.UpdatedAt,
		})
	}
	return result
}

func assetStageInstruction(mode, stageKey string) string {
	_, _ = mode, stageKey
	return "运行接口：严格按上方官方总控和强制读取原文执行当前阶段。只输出当前阶段官方规定的 Markdown 工件；不要输出规则复述、思考过程、流程回执或机检说明。"
}

func validateCompiledAssetArtifact(tx *repository.Repository, run model.ManjuAssetRun, stage model.ManjuAssetStage) error {

	artifacts, err := tx.ManjuAssetArtifacts(run.ID)
	if err != nil {
		return err
	}
	var content string
	for _, artifact := range artifacts {
		if artifact.ID == stage.ArtifactID {
			content = artifact.Content
			break
		}
	}
	if strings.TrimSpace(content) == "" {
		return ErrAssetStageNotReady
	}
	references, err := requiredAssetReferencesForCompile(tx, run, stage)
	if err != nil {
		return err
	}
	return validateAssetDelivery(run.AssetMode, content, references)
}

func requiredAssetReferencesForCompile(tx *repository.Repository, run model.ManjuAssetRun, stage model.ManjuAssetStage) ([]requiredAssetReference, error) {
	if run.AssetMode != AssetModeM2Value || stage.Key != AssetStageCompile {
		return nil, nil
	}
	stages, err := tx.ManjuAssetStages(run.ID)
	if err != nil {
		return nil, err
	}
	artifacts, err := tx.ManjuAssetArtifacts(run.ID)
	if err != nil {
		return nil, err
	}
	var manifestContent string
	for _, candidate := range stages {
		if candidate.Key != AssetStageManifest || strings.TrimSpace(candidate.ArtifactID) == "" {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.ID == candidate.ArtifactID {
				manifestContent = artifact.Content
				break
			}
		}
	}
	codes := assetCodesFromManifest(manifestContent)
	references, err := tx.ManjuAssetReferences(run.ID)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]string, len(references))
	for _, reference := range references {
		if reference.Status == model.ManjuAssetReferenceStatusReady {
			byCode[reference.Code] = reference.ResourceID
		}
	}
	result := make([]requiredAssetReference, 0, len(codes))
	for _, code := range codes {
		result = append(result, requiredAssetReference{Code: code, ResourceID: byCode[code]})
	}
	return result, nil
}

func assetCodesFromManifest(content string) []string {
	matches := assetCodePattern.FindAllString(content, -1)
	seen := make(map[string]bool, len(matches))
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		match = strings.TrimSpace(match)
		if match == "" || seen[match] {
			continue
		}
		seen[match] = true
		result = append(result, match)
	}
	sort.Strings(result)
	return result
}

func assetArtifactKind(mode, stageKey string) string {
	switch stageKey {
	case AssetStageExtraction:
		return model.ManjuAssetArtifactExtraction
	case AssetStageManifest:
		return model.ManjuAssetArtifactManifest
	case AssetStageCompile:
		switch mode {
		case AssetModeM0Value:
			return model.ManjuAssetArtifactM0
		case AssetModeM2Value:
			return model.ManjuAssetArtifactM2
		default:
			return model.ManjuAssetArtifactM1
		}
	default:
		return "ASSET_STAGE_OUTPUT.md"
	}
}

func assetStageIndex(stages []model.ManjuAssetStage, key string) int {
	for index := range stages {
		if stages[index].Key == key {
			return index
		}
	}
	return -1
}

func canEditAsset(role string) bool {
	return role == model.ManjuMemberRoleOwner ||
		role == model.ManjuMemberRoleProducer ||
		role == model.ManjuMemberRoleDirector
}

func canConfirmAsset(role string) bool {
	return role == model.ManjuMemberRoleOwner ||
		role == model.ManjuMemberRoleProducer ||
		role == model.ManjuMemberRoleDirector
}

func assetRunView(run model.ManjuAssetRun) AssetRunView {
	return AssetRunView{
		ID: run.ID, ProjectID: run.ProjectID, ScriptRunID: run.ScriptRunID,
		BaseAssetRunID: run.BaseAssetRunID, ReuseAssets: run.ReuseAssets,
		InheritedAssetCount: run.InheritedAssetCount,
		AssetMode:           run.AssetMode, Status: run.Status, CurrentStage: run.CurrentStage,
		LockRevision: run.LockRevision, CreatedBy: run.CreatedBy,
		CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt,
	}
}

func assetStageView(stage model.ManjuAssetStage) AssetStageView {
	return AssetStageView{
		ID: stage.ID, Key: stage.Key, Title: stage.Title, Position: stage.Position,
		Status: stage.Status, Version: stage.Version, TaskID: stage.TaskID,
		ArtifactID: stage.ArtifactID, Error: stage.Error, ConfirmedBy: stage.ConfirmedBy,
		ConfirmedAt: stage.ConfirmedAt, StartedAt: stage.StartedAt,
		CompletedAt: stage.CompletedAt, UpdatedAt: stage.UpdatedAt,
	}
}
