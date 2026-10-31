package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ScriptModeStepwise     = model.ManjuScriptModeStepwise
	ScriptModeFullPipeline = model.ManjuScriptModeFullPipeline

	ScriptInputNovel    = "novel"
	ScriptInputOutline  = "outline"
	ScriptInputFinished = "finished_script"

	TrendResolutionOriginal  = "original"
	TrendResolutionBenchmark = "benchmark"

	ScriptBranchOriginal   = model.ManjuScriptBranchOriginal
	ScriptBranchAdaptation = model.ManjuScriptBranchAdaptation
	ScriptBranchBenchmark  = model.ManjuScriptBranchBenchmark
	ScriptBranchPolish     = model.ManjuScriptBranchPolish

	ScriptStageSourceRegistration = model.ManjuScriptStageSourceRegistration
	ScriptStageReconIntelligence  = model.ManjuScriptStageReconIntelligence
	ScriptStageIPDevelopment      = model.ManjuScriptStageIPDevelopment
	ScriptStageSeriesBible        = model.ManjuScriptStageSeriesBible
	ScriptStageEpisodePlanning    = model.ManjuScriptStageEpisodePlanning
	ScriptStageScriptDraft        = model.ManjuScriptStageScriptDraft
	ScriptStageRoundtableReview   = model.ManjuScriptStageRoundtableReview
	ScriptStagePolishDraft        = model.ManjuScriptStagePolishDraft
	ScriptStagePolishFindings     = model.ManjuScriptStagePolishFindings
	ScriptStagePolishConfirmation = model.ManjuScriptStagePolishConfirmation

	ScriptStageStatusNotStarted           = model.ManjuScriptStageStatusNotStarted
	ScriptStageStatusInProgress           = model.ManjuScriptStageStatusInProgress
	ScriptStageStatusAwaitingConfirm      = model.ManjuScriptStageStatusAwaitingConfirm
	ScriptStageStatusAwaitingConfirmation = ScriptStageStatusAwaitingConfirm
	ScriptStageStatusAwaitingDecision     = model.ManjuScriptStageStatusAwaitingDecision
	ScriptStageStatusLocked               = model.ManjuScriptStageStatusLocked
	ScriptStageStatusStale                = model.ManjuScriptStageStatusStale
	ScriptStageStatusBlocked              = model.ManjuScriptStageStatusBlocked
)

var (
	ErrP0Required                         = errors.New("manju P0 lock is required")
	ErrScriptRunNotFound                  = errors.New("manju script run not found")
	ErrScriptStageNotFound                = errors.New("manju script stage not found")
	ErrInvalidScriptRun                   = errors.New("invalid manju script run")
	ErrTrendResolutionRequired            = errors.New("manju trend resolution is required")
	ErrInvalidScriptSource                = errors.New("invalid manju script source")
	ErrScriptSourceNotFound               = errors.New("manju script source not found")
	ErrScriptUnitInProgress               = errors.New("manju script adaptation unit is still in progress")
	ErrSourceLibraryChanged               = errors.New("manju source library changed since the base unit")
	ErrScriptStageNotReady                = errors.New("manju script stage is not ready")
	ErrRoundtableDecisionRequired         = errors.New("manju roundtable decision is required")
	ErrRoundtableDecisionEndpointRequired = errors.New("manju roundtable decision must use the dedicated decision endpoint")
	ErrRoundtableGradeInvalid             = errors.New("manju roundtable grade is missing, unsupported, or conflicting")
	ErrRoundtableQualityRiskRequired      = errors.New("manju roundtable quality risk acceptance is required")
	ErrScriptTaskUnavailable              = errors.New("manju script task service is unavailable")

	scriptInputKindValues = valueSet(ScriptInputNovel, ScriptInputOutline, ScriptInputFinished)
	trendResolutionValues = valueSet(TrendResolutionOriginal, TrendResolutionBenchmark)

	roundtableGradeJSONPattern  = regexp.MustCompile(`(?m)(?:^|[\s,{])"grade"\s*:\s*"([^"\r\n]*)"`)
	roundtableGradeFieldPattern = regexp.MustCompile(`(?mi)^\s*(?:[-*]\s*)?(?:\*\*)?(?:grade|等级)(?:\*\*)?\s*[:：]\s*(.*?)\s*$`)
	roundtableGradeTablePattern = regexp.MustCompile(`(?mi)^\s*\|\s*(?:\*\*)?(?:grade|等级)(?:\*\*)?\s*\|\s*([^|\r\n]+)\s*\|`)

	roundtableGradePatterns = []*regexp.Regexp{
		roundtableGradeJSONPattern,
		roundtableGradeFieldPattern,
		roundtableGradeTablePattern,
	}
)

type ScriptRunInput struct {
	Mode              string   `json:"mode"`
	InputKind         string   `json:"inputKind"`
	TrendResolution   string   `json:"trendResolution"`
	Title             string   `json:"title"`
	SourceDocumentID  string   `json:"sourceDocumentId"`
	ChapterIDs        []string `json:"chapterIds"`
	BaseRunID         string   `json:"baseRunId"`
	ReuseGlobalStages bool     `json:"reuseGlobalStages"`
}

type ScriptSourceInput struct {
	Title              string            `json:"title"`
	AuthorizationScope string            `json:"authorizationScope"`
	RangeLabel         string            `json:"rangeLabel"`
	Content            string            `json:"content"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type ScriptStageRunInput struct {
	Instruction string `json:"instruction"`
}

type RoundtableDecision string

const (
	RoundtableProceed RoundtableDecision = "PROCEED_CURRENT"
	RoundtableRefine  RoundtableDecision = "REFINE_OR_REWRITE"

	RoundtableGradeS        = "S"
	RoundtableGradeA        = "A"
	RoundtableGradeB        = "B"
	RoundtableGradeNotReady = "NOT_READY"
	RoundtableGradeUnknown  = "UNKNOWN"
)

type RoundtableDecisionInput struct {
	Decision          RoundtableDecision `json:"decision"`
	AcceptQualityRisk bool               `json:"acceptQualityRisk"`
	Note              string             `json:"note,omitempty"`
}

type ScriptTaskCreateFunc func(userID string, request localtask.CreateRequest) (*model.Task, error)

func (s *Service) SetScriptTaskCreator(create ScriptTaskCreateFunc) {
	if s != nil {
		s.scriptTaskCreate = create
	}
}

type ScriptRunView struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"projectId"`
	Branch            string    `json:"branch"`
	Mode              string    `json:"mode"`
	InputKind         string    `json:"inputKind"`
	TrendResolution   string    `json:"trendResolution"`
	Status            string    `json:"status"`
	CurrentStage      string    `json:"currentStage"`
	LockRevision      int64     `json:"lockRevision"`
	SourceID          string    `json:"sourceId,omitempty"`
	UnitTitle         string    `json:"unitTitle"`
	SourceDocumentID  string    `json:"sourceDocumentId,omitempty"`
	SourceChapterIDs  []string  `json:"sourceChapterIds"`
	SourceRangeLabel  string    `json:"sourceRangeLabel"`
	SourceSetHash     string    `json:"sourceSetHash"`
	BaseRunID         string    `json:"baseRunId,omitempty"`
	ReuseGlobalStages bool      `json:"reuseGlobalStages"`
	CreatedBy         string    `json:"createdBy"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type ScriptSourceView struct {
	ID                 string            `json:"id"`
	ProjectID          string            `json:"projectId"`
	RunID              string            `json:"runId"`
	Branch             string            `json:"branch"`
	Title              string            `json:"title"`
	AuthorizationScope string            `json:"authorizationScope"`
	RangeLabel         string            `json:"rangeLabel"`
	ContentHash        string            `json:"contentHash"`
	CharacterCount     int64             `json:"characterCount"`
	ExtractionStatus   string            `json:"extractionStatus"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	CreatedBy          string            `json:"createdBy"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

type ScriptStageView struct {
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
	ReviewGrade string               `json:"reviewGrade,omitempty"`
}

type ScriptArtifactView struct {
	ID          string    `json:"id"`
	StageID     string    `json:"stageId"`
	Kind        string    `json:"kind"`
	Version     int64     `json:"version"`
	Content     string    `json:"content"`
	ContentHash string    `json:"contentHash"`
	TaskID      string    `json:"taskId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type RoundtableDecisionView struct {
	ID                 string    `json:"id"`
	ProjectID          string    `json:"projectId"`
	RunID              string    `json:"runId"`
	StageID            string    `json:"stageId"`
	Round              int64     `json:"round"`
	Version            int64     `json:"version"`
	Decision           string    `json:"decision"`
	ActorID            string    `json:"actorId"`
	Note               string    `json:"note,omitempty"`
	ReviewArtifactID   string    `json:"reviewArtifactId"`
	ReviewArtifactHash string    `json:"reviewArtifactHash"`
	Grade              string    `json:"grade"`
	RiskAccepted       bool      `json:"riskAccepted"`
	CreatedAt          time.Time `json:"createdAt"`
}

type ScriptRunDetail struct {
	Run                 ScriptRunView            `json:"run"`
	Source              *ScriptSourceView        `json:"source,omitempty"`
	Stages              []ScriptStageView        `json:"stages"`
	Artifacts           []ScriptArtifactView     `json:"artifacts,omitempty"`
	RoundtableDecisions []RoundtableDecisionView `json:"roundtableDecisions,omitempty"`
}

type scriptStageDefinition struct {
	Key   string
	Title string
}

func (s *Service) ScriptRuns(actorID, projectID string) ([]ScriptRunView, error) {
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
	runs, err := s.repo.ManjuScriptRuns(projectID)
	if err != nil {
		return nil, err
	}
	latestLock, err := s.repo.LatestManjuLockReceipt(projectID)
	if err != nil {
		return nil, err
	}
	if latestLock != nil {
		if err := s.repo.AbandonManjuRunsBeforeLockRevision(projectID, latestLock.Revision); err != nil {
			return nil, err
		}
		runs, err = s.repo.ManjuScriptRuns(projectID)
		if err != nil {
			return nil, err
		}
	}
	for index := range runs {
		if runs[index].Status != "ACTIVE" || runs[index].Branch != ScriptBranchAdaptation {
			continue
		}
		var sourceCount int64
		if err := s.repo.DB().Table("manju_script_sources").
			Where("run_id = ? AND TRIM(content_text) <> ''", runs[index].ID).
			Count(&sourceCount).Error; err != nil {
			return nil, err
		}
		if sourceCount == 0 {
			if err := s.repo.DB().Table("manju_script_runs").Where("id = ?", runs[index].ID).Updates(map[string]any{
				"status": "ABANDONED", "current_stage": "", "updated_at": time.Now(),
			}).Error; err != nil {
				return nil, err
			}
			runs[index].Status = "ABANDONED"
			runs[index].CurrentStage = ""
		}
	}
	items := make([]ScriptRunView, 0, len(runs))
	for _, run := range runs {
		items = append(items, scriptRunView(run))
	}
	return items, nil
}

func (s *Service) CreateScriptRun(actorID, projectID string, input ScriptRunInput) (*ScriptRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	input.Mode = strings.TrimSpace(input.Mode)
	input.InputKind = strings.TrimSpace(input.InputKind)
	input.TrendResolution = strings.TrimSpace(input.TrendResolution)
	input.Title = strings.TrimSpace(input.Title)
	input.SourceDocumentID = strings.TrimSpace(input.SourceDocumentID)
	input.BaseRunID = strings.TrimSpace(input.BaseRunID)
	if input.Mode == "" {
		input.Mode = ScriptModeStepwise
	}
	if input.Mode != ScriptModeStepwise && input.Mode != ScriptModeFullPipeline {
		return nil, ErrInvalidScriptRun
	}
	libraryState, err := s.SourceLibraryState(actorID, projectID)
	if err != nil {
		return nil, err
	}

	var runID string
	err = s.repo.Transaction(func(tx *repository.Repository) error {
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
		if input.InputKind == "" && lock.Snapshot.Source == SourceAdaptation {
			input.InputKind = ScriptInputNovel
		}
		if (input.InputKind != "" && !scriptInputKindValues[input.InputKind]) ||
			(input.TrendResolution != "" && !trendResolutionValues[input.TrendResolution]) {
			return ErrInvalidScriptRun
		}
		if lock.Snapshot.Source != SourceTrend {
			input.TrendResolution = ""
		}
		branch, err := scriptBranchForInput(lock.Snapshot.Source, input.InputKind, input.TrendResolution)
		if err != nil {
			return err
		}
		if err := tx.AbandonManjuRunsBeforeLockRevision(projectID, receipt.Revision); err != nil {
			return err
		}
		var activeRuns []struct {
			ID     string
			Branch string
		}
		if err := tx.DB().Table("manju_script_runs").
			Where("project_id = ? AND status = ?", projectID, "ACTIVE").
			Find(&activeRuns).Error; err != nil {
			return err
		}
		for _, active := range activeRuns {
			if active.Branch == ScriptBranchAdaptation {
				var sourceCount int64
				if err := tx.DB().Table("manju_script_sources").
					Where("run_id = ? AND TRIM(content_text) <> ''", active.ID).
					Count(&sourceCount).Error; err != nil {
					return err
				}
				if sourceCount == 0 {
					if err := tx.DB().Table("manju_script_runs").Where("id = ?", active.ID).Updates(map[string]any{
						"status": "ABANDONED", "current_stage": "", "updated_at": time.Now(),
					}).Error; err != nil {
						return err
					}
					continue
				}
			}
			return ErrScriptUnitInProgress
		}
		var seededSource *ScriptSourceInput
		var selectedChapters []model.ManjuSourceChapter
		if input.SourceDocumentID != "" {
			if (branch != ScriptBranchAdaptation && branch != ScriptBranchPolish) || len(input.ChapterIDs) == 0 {
				return ErrInvalidScriptRun
			}
			document, err := tx.ManjuSourceDocument(projectID, input.SourceDocumentID)
			if err != nil {
				return err
			}
			if document == nil {
				return ErrScriptSourceNotFound
			}
			chapterIDs := uniqueNonEmptyStrings(input.ChapterIDs)
			selectedChapters, err = tx.ManjuSourceChaptersByIDs(projectID, document.ID, chapterIDs)
			if err != nil {
				return err
			}
			if len(selectedChapters) != len(chapterIDs) {
				return ErrInvalidScriptSource
			}
			seededSource = scriptSourceInputFromChapters(*document, selectedChapters, input.InputKind)
			if input.Title == "" {
				input.Title = selectedChapters[0].Title
			}
		}
		chapterIDsJSON, err := json.Marshal(uniqueNonEmptyStrings(input.ChapterIDs))
		if err != nil {
			return err
		}
		now := time.Now()
		run := &model.ManjuScriptRun{
			ID:                   uuid.NewString(),
			ProjectID:            projectID,
			Branch:               branch,
			Mode:                 input.Mode,
			InputKind:            input.InputKind,
			TrendResolution:      input.TrendResolution,
			Status:               model.ManjuScriptRunStatusActive,
			CurrentStage:         scriptStageDefinitions(branch)[0].Key,
			LockRevision:         receipt.Revision,
			UnitTitle:            input.Title,
			SourceDocumentID:     input.SourceDocumentID,
			SourceChapterIDsJSON: string(chapterIDsJSON),
			SourceRangeLabel:     scriptSourceRangeLabel(input.SourceDocumentID, selectedChapters),
			SourceSetHash:        libraryState.ContentHash,
			BaseRunID:            input.BaseRunID,
			ReuseGlobalStages:    input.ReuseGlobalStages,
			CreatedBy:            actorID,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := tx.CreateManjuScriptRun(run); err != nil {
			return err
		}
		stages := make([]model.ManjuScriptStage, 0, len(scriptStageDefinitions(branch)))
		for index, definition := range scriptStageDefinitions(branch) {
			status := model.ManjuScriptStageStatusNotStarted
			var startedAt *time.Time
			if index == 0 {
				status = model.ManjuScriptStageStatusInProgress
				startedAt = &now
			}
			stages = append(stages, model.ManjuScriptStage{
				ID:        uuid.NewString(),
				RunID:     run.ID,
				Key:       definition.Key,
				Title:     definition.Title,
				Position:  index + 1,
				Status:    status,
				Version:   0,
				StartedAt: startedAt,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		if err := tx.CreateManjuScriptStages(stages); err != nil {
			return err
		}
		if seededSource != nil {
			sourceStage := &stages[0]
			source, err := persistScriptSource(tx, projectID, actorID, run, sourceStage, *seededSource, now)
			if err != nil {
				return err
			}
			run.SourceID = source.ID
			sourceStage.Status = model.ManjuScriptStageStatusLocked
			sourceStage.ConfirmedBy = actorID
			sourceStage.ConfirmedAt = &now
			if err := tx.SaveManjuScriptStage(sourceStage); err != nil {
				return err
			}
			next, err := activateNextScriptStage(tx, run, sourceStage.Position)
			if err != nil {
				return err
			}
			run.CurrentStage = next
		}
		if run.ReuseGlobalStages {
			var previousRuns []struct {
				ID            string
				LockRevision  int64
				Status        string
				SourceSetHash string
			}
			if err := tx.DB().Table("manju_script_runs").Where("project_id = ?", projectID).Find(&previousRuns).Error; err != nil {
				return err
			}
			baseRunID := run.BaseRunID
			if baseRunID == "" {
				for _, candidate := range previousRuns {
					if candidate.ID != run.ID && candidate.LockRevision == run.LockRevision && candidate.Status == "COMPLETED" {
						baseRunID = candidate.ID
						break
					}
				}
				run.BaseRunID = baseRunID
			}
			if baseRunID != "" {
				baseRunIndex := -1
				for index := range previousRuns {
					if previousRuns[index].ID == baseRunID {
						baseRunIndex = index
						break
					}
				}
				if baseRunIndex < 0 || previousRuns[baseRunIndex].LockRevision != run.LockRevision || previousRuns[baseRunIndex].Status != "COMPLETED" {
					return ErrInvalidScriptRun
				}
				if previousRuns[baseRunIndex].SourceSetHash == "" || previousRuns[baseRunIndex].SourceSetHash != run.SourceSetHash {
					return ErrSourceLibraryChanged
				}
				if err := reuseGlobalScriptStages(tx, baseRunID, run.ID, run.LockRevision, actorID, now); err != nil {
					return err
				}
				next, err := activateNextScriptStage(tx, run, 1)
				if err != nil {
					return err
				}
				run.CurrentStage = next
			}
		}
		runID = run.ID
		return tx.SaveManjuScriptRun(run)
	})
	if err != nil {
		return nil, err
	}
	return s.ScriptRunDetail(actorID, projectID, runID)
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func scriptSourceInputFromChapters(document model.ManjuSourceDocument, chapters []model.ManjuSourceChapter, inputKind string) *ScriptSourceInput {
	if len(chapters) == 0 {
		return nil
	}
	parts := make([]string, 0, len(chapters))
	ranges := make([]string, 0, len(chapters))
	chapterIDs := make([]string, 0, len(chapters))
	for _, chapter := range chapters {
		parts = append(parts, "# "+chapter.Title+"\n\n"+chapter.ContentText)
		ranges = append(ranges, chapter.RangeLabel)
		chapterIDs = append(chapterIDs, chapter.ID)
	}
	sourceType := inputKind
	if sourceType == "" {
		sourceType = ScriptInputNovel
	}
	return &ScriptSourceInput{
		Title: document.Title, AuthorizationScope: document.AuthorizationScope,
		RangeLabel: strings.Join(ranges, "、"), Content: strings.Join(parts, "\n\n"),
		Metadata: map[string]string{
			"sourceType": sourceType, "sourceDocumentId": document.ID,
			"sourceChapterIds": strings.Join(chapterIDs, ","),
		},
	}
}

func scriptSourceRangeLabel(documentID string, chapters []model.ManjuSourceChapter) string {
	if documentID == "" || len(chapters) == 0 {
		return ""
	}
	ranges := make([]string, 0, len(chapters))
	for _, chapter := range chapters {
		ranges = append(ranges, chapter.RangeLabel)
	}
	return strings.Join(ranges, "、")
}

func (s *Service) RegisterScriptSource(actorID, projectID, runID string, input ScriptSourceInput) (*ScriptRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	input.Title = strings.TrimSpace(input.Title)
	input.AuthorizationScope = strings.TrimSpace(input.AuthorizationScope)
	input.RangeLabel = strings.TrimSpace(input.RangeLabel)
	input.Content = strings.TrimSpace(input.Content)
	autoStageKey := ""
	if input.Title == "" || input.Content == "" {
		return nil, ErrInvalidScriptSource
	}

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
		if run.Status != model.ManjuScriptRunStatusActive {
			return ErrInvalidScriptRun
		}
		stage, err := tx.ManjuScriptStage(runID, ScriptStageSourceRegistration)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrScriptStageNotFound
		}
		if stage.Status != model.ManjuScriptStageStatusInProgress && stage.Status != model.ManjuScriptStageStatusAwaitingConfirm && stage.Status != model.ManjuScriptStageStatusLocked {
			return ErrScriptStageNotReady
		}
		if err := validateScriptSourceForBranch(run.Branch, input); err != nil {
			return err
		}
		metadataJSON, err := json.Marshal(input.Metadata)
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(input.Content))
		contentHash := hex.EncodeToString(sum[:])
		now := time.Now()
		source, err := tx.ManjuScriptSource(runID)
		if err != nil {
			return err
		}
		if source == nil {
			source = &model.ManjuScriptSource{
				ID:        uuid.NewString(),
				ProjectID: projectID,
				RunID:     runID,
				Branch:    run.Branch,
				CreatedBy: actorID,
				CreatedAt: now,
			}
		}
		source.Title = input.Title
		source.AuthorizationScope = input.AuthorizationScope
		source.RangeLabel = input.RangeLabel
		source.ContentText = input.Content
		source.ContentHash = contentHash
		source.CharacterCount = int64(len([]rune(input.Content)))
		source.ExtractionStatus = "ready"
		source.MetadataJSON = string(metadataJSON)
		source.UpdatedAt = now
		if err := tx.SaveManjuScriptSource(source); err != nil {
			return err
		}
		manifest := map[string]any{
			"schemaVersion":       1,
			"branch":              run.Branch,
			"inputKind":           run.InputKind,
			"trendResolution":     run.TrendResolution,
			"title":               source.Title,
			"authorizationScope":  source.AuthorizationScope,
			"rangeLabel":          source.RangeLabel,
			"contentHash":         source.ContentHash,
			"characterCount":      source.CharacterCount,
			"extractionStatus":    source.ExtractionStatus,
			"registeredBy":        actorID,
			"registeredAt":        now.UTC().Format(time.RFC3339),
			"rawTextStored":       true,
			"downstreamConsumers": []string{"ip_development", "series_bible", "episode_planning", "script_draft"},
		}
		manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		artifactSum := sha256.Sum256(manifestJSON)
		stage.Version++
		artifact := &model.ManjuScriptArtifact{
			ID:          uuid.NewString(),
			ProjectID:   projectID,
			RunID:       runID,
			StageID:     stage.ID,
			Kind:        model.ManjuScriptArtifactSourceManifest,
			Version:     stage.Version,
			Content:     string(manifestJSON),
			ContentHash: hex.EncodeToString(artifactSum[:]),
			CreatedBy:   actorID,
			CreatedAt:   now,
		}
		if err := tx.CreateManjuScriptArtifact(artifact); err != nil {
			return err
		}
		if err := recordManjuArtifactAudit(
			tx, projectID, run.ID, stage.ID, artifact.ID, "P1", artifact.Kind,
			artifact.Version, model.ManjuArtifactVisibilityInternal,
			artifact.Content, artifact.ContentHash, artifact.TaskID, actorID,
		); err != nil {
			return err
		}
		stage.ArtifactID = artifact.ID
		stage.CompletedAt = &now
		stage.Error = ""
		stage.ConfirmedBy = ""
		stage.ConfirmedAt = nil
		if run.Mode == ScriptModeStepwise {
			stage.Status = model.ManjuScriptStageStatusAwaitingConfirm
		} else {
			stage.Status = model.ManjuScriptStageStatusLocked
		}
		if stage.Version > 1 {
			if err := markDownstreamScriptStagesStale(tx, runID, stage.Position); err != nil {
				return err
			}
		}
		if err := tx.SaveManjuScriptStage(stage); err != nil {
			return err
		}
		run.SourceID = source.ID
		run.CurrentStage = stage.Key
		if run.Mode == ScriptModeFullPipeline {
			next, err := activateNextScriptStage(tx, run, stage.Position)
			if err != nil {
				return err
			}
			run.CurrentStage = next
			autoStageKey = next
		}
		return tx.SaveManjuScriptRun(run)
	})
	if err != nil {
		return nil, err
	}
	if autoStageKey != "" {
		_, _ = s.RunScriptStage(actorID, projectID, runID, autoStageKey, ScriptStageRunInput{})
	}
	return s.ScriptRunDetail(actorID, projectID, runID)
}

func persistScriptSource(tx *repository.Repository, projectID, actorID string, run *model.ManjuScriptRun, stage *model.ManjuScriptStage, input ScriptSourceInput, now time.Time) (*model.ManjuScriptSource, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.AuthorizationScope = strings.TrimSpace(input.AuthorizationScope)
	input.RangeLabel = strings.TrimSpace(input.RangeLabel)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" {
		return nil, ErrInvalidScriptSource
	}
	if err := validateScriptSourceForBranch(run.Branch, input); err != nil {
		return nil, err
	}
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, err
	}
	source := &model.ManjuScriptSource{
		ID: uuid.NewString(), ProjectID: projectID, RunID: run.ID, Branch: run.Branch,
		Title: input.Title, AuthorizationScope: input.AuthorizationScope, RangeLabel: input.RangeLabel,
		ContentText: input.Content, ContentHash: contentHash(input.Content),
		CharacterCount: int64(len([]rune(input.Content))), ExtractionStatus: "ready",
		MetadataJSON: string(metadataJSON), CreatedBy: actorID, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.SaveManjuScriptSource(source); err != nil {
		return nil, err
	}
	manifest := map[string]any{
		"schemaVersion": 1, "branch": run.Branch,
		"inputKind": run.InputKind, "trendResolution": run.TrendResolution,
		"title":              source.Title,
		"authorizationScope": source.AuthorizationScope, "rangeLabel": source.RangeLabel,
		"contentHash": source.ContentHash, "characterCount": source.CharacterCount,
		"extractionStatus": source.ExtractionStatus, "registeredBy": actorID,
		"registeredAt": now.UTC().Format(time.RFC3339), "rawTextStored": true,
		"downstreamConsumers": []string{"ip_development", "series_bible", "episode_planning", "script_draft"},
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	stage.Version++
	artifact := &model.ManjuScriptArtifact{
		ID: uuid.NewString(), ProjectID: projectID, RunID: run.ID, StageID: stage.ID,
		Kind: model.ManjuScriptArtifactSourceManifest, Version: stage.Version,
		Content: string(manifestJSON), ContentHash: contentHash(string(manifestJSON)),
		CreatedBy: actorID, CreatedAt: now,
	}
	if err := tx.CreateManjuScriptArtifact(artifact); err != nil {
		return nil, err
	}
	if err := recordManjuArtifactAudit(
		tx, projectID, run.ID, stage.ID, artifact.ID, "P1", artifact.Kind,
		artifact.Version, model.ManjuArtifactVisibilityInternal,
		artifact.Content, artifact.ContentHash, artifact.TaskID, actorID,
	); err != nil {
		return nil, err
	}
	stage.ArtifactID = artifact.ID
	stage.CompletedAt = &now
	stage.Error = ""
	if run.Mode == ScriptModeStepwise {
		stage.Status = model.ManjuScriptStageStatusAwaitingConfirm
	} else {
		stage.Status = model.ManjuScriptStageStatusLocked
		stage.ConfirmedBy = actorID
		stage.ConfirmedAt = &now
	}
	if err := tx.SaveManjuScriptStage(stage); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *Service) RunScriptStage(actorID, projectID, runID, stageKey string, input ScriptStageRunInput) (*ScriptRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Instruction = strings.TrimSpace(input.Instruction)
	if s.scriptTaskCreate == nil {
		return nil, ErrScriptTaskUnavailable
	}
	if stageKey == ScriptStageSourceRegistration {
		return nil, ErrScriptStageNotReady
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
		if run.Status != model.ManjuScriptRunStatusActive {
			return ErrInvalidScriptRun
		}
		stage, err := tx.ManjuScriptStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrScriptStageNotFound
		}
		if strings.TrimSpace(stage.TaskID) != "" {
			existingTaskID = stage.TaskID
			return nil
		}
		if stage.Status != model.ManjuScriptStageStatusInProgress && stage.Status != model.ManjuScriptStageStatusBlocked && stage.Status != model.ManjuScriptStageStatusStale {
			return ErrScriptStageNotReady
		}
		binding, err := tx.ManjuProjectModelBinding(projectID)
		if err != nil {
			return err
		}
		if binding == nil || strings.TrimSpace(binding.TextChannelID) == "" || strings.TrimSpace(binding.TextModelKey) == "" {
			return ErrInvalidModelBinding
		}
		source, err := tx.ManjuScriptSource(runID)
		if err != nil {
			return err
		}
		artifacts, err := tx.ManjuScriptArtifacts(runID)
		if err != nil {
			return err
		}
		artifacts = scriptStagePromptArtifacts(stage.Key, artifacts)
		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		skillOptions := manjuSkillStageOptions("P1", receipt, assetModeFromReceipt(receipt), "none")
		skillOptions.ExternalScript = run.Branch == ScriptBranchPolish
		if run.TrendResolution == TrendResolutionOriginal {
			skillOptions.Source = "1"
		} else if run.TrendResolution == TrendResolutionBenchmark {
			skillOptions.Source = "3"
		}
		ruleBundle, err := defaultManjuSkillRuntime.StageRuleDispatch(skillOptions)
		if err != nil {
			return err
		}
		skillContext := ruleBundle.Content
		projectSourceText := ""
		if stage.Key == ScriptStageIPDevelopment || stage.Key == ScriptStageSeriesBible {
			projectSourceText, err = projectSourceLibraryPrompt(tx, projectID)
			if err != nil {
				return err
			}
		}
		promptInstruction := input.Instruction
		if projectSourceText != "" {
			promptInstruction = "【项目完整原著库】\n" + projectSourceText + "\n\n" + promptInstruction
		}
		prompt := buildScriptStagePrompt(access.Project.Name, *run, *stage, source, artifacts, receipt, promptInstruction, skillContext)
		operationKey = fmt.Sprintf("manju:%s:%s:v%d", run.ID, stage.Key, scriptStageDispatchVersion(*stage))
		ruleSnapshot, err := s.ensureManjuRuleSnapshot(tx, projectID, run.LockRevision, actorID)
		if err != nil {
			return err
		}
		exchangeID := uuid.NewString()
		request = localtask.CreateRequest{
			ProjectID: projectID,
			Type:      "canvas_text",
			Operation: "manju_script_stage",
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
					"clientOperationId":  operationKey,
					"manjuProjectId":     projectID,
					"manjuRunId":         run.ID,
					"manjuStageKey":      stage.Key,
					"manjuStageTitle":    stage.Title,
					"manjuRuleSnapshot":  ruleSnapshot.ID,
					"manjuModelExchange": exchangeID,
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
		if err := s.recordManjuRuleReads(tx, ruleSnapshot, run.ID, stage.ID, "P1", operationKey, ruleBundle); err != nil {
			return err
		}
		stageInputValues := map[string]any{
			"instruction":     input.Instruction,
			"prompt":          prompt,
			"ruleSnapshotId":  ruleSnapshot.ID,
			"modelExchangeId": exchange.ID,
		}
		if preparedVersion := scriptStagePreparedVersion(stage.InputJSON); preparedVersion > 0 {
			stageInputValues["preparedVersion"] = preparedVersion
		}
		stageInput, err := json.Marshal(stageInputValues)
		if err != nil {
			return err
		}
		stage.InputJSON = string(stageInput)
		stage.Status = model.ManjuScriptStageStatusInProgress
		stage.Error = ""
		return tx.SaveManjuScriptStage(stage)
	})
	if err != nil {
		return nil, err
	}
	if existingTaskID != "" {
		return s.ScriptRunDetail(actorID, projectID, runID)
	}

	task, err := s.scriptTaskCreate(actorID, request)
	if err != nil {
		s.blockScriptStage(projectID, runID, stageKey, err.Error())
		return nil, err
	}
	if task == nil || strings.TrimSpace(task.ID) == "" {
		err := errors.New("文本任务服务没有返回任务 ID")
		s.blockScriptStage(projectID, runID, stageKey, err.Error())
		return nil, err
	}
	err = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuScriptStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrScriptStageNotFound
		}
		if strings.TrimSpace(stage.TaskID) == "" {
			stage.TaskID = task.ID
		}
		stage.Status = model.ManjuScriptStageStatusInProgress
		stage.Error = ""
		if err := tx.AttachManjuModelExchangeTask(operationKey, task.ID); err != nil {
			return err
		}
		return tx.SaveManjuScriptStage(stage)
	})
	if err != nil {
		return nil, err
	}
	return s.ScriptRunDetail(actorID, projectID, runID)
}

// RegenerateScriptArtifact reruns one P1 artifact in the existing run while
// preserving locked upstream artifacts and historical outputs.
func (s *Service) RegenerateScriptArtifact(actorID, projectID, runID, stageKey string, input ScriptStageRunInput) (*ScriptRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	input.Instruction = strings.TrimSpace(input.Instruction)

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
		if run.Status != model.ManjuScriptRunStatusActive &&
			run.Status != model.ManjuScriptRunStatusCompleted {
			return ErrInvalidScriptRun
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
		if strings.TrimSpace(target.ArtifactID) == "" {
			return ErrScriptStageNotReady
		}
		if target.Key == ScriptStageRoundtableReview &&
			target.Status == model.ManjuScriptStageStatusAwaitingDecision {
			return ErrScriptStageNotReady
		}
		switch target.Status {
		case model.ManjuScriptStageStatusLocked,
			model.ManjuScriptStageStatusAwaitingConfirm,
			model.ManjuScriptStageStatusAwaitingDecision,
			model.ManjuScriptStageStatusBlocked:
		default:
			return ErrScriptStageNotReady
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
				return ErrScriptStageNotReady
			}
		}
		target.Status = model.ManjuScriptStageStatusInProgress
		target.TaskID = ""
		target.ArtifactID = ""
		target.Error = ""
		target.ConfirmedBy = ""
		target.ConfirmedAt = nil
		target.CompletedAt = nil
		target.InputJSON = ""
		if err := tx.SaveManjuScriptStage(target); err != nil {
			return err
		}
		for index := targetIndex + 1; index < len(stages); index++ {
			downstream := &stages[index]
			downstream.Status = model.ManjuScriptStageStatusStale
			downstream.TaskID = ""
			downstream.ArtifactID = ""
			downstream.Error = "上游工件已重新生成，等待新工件"
			downstream.ConfirmedBy = ""
			downstream.ConfirmedAt = nil
			downstream.CompletedAt = nil
			downstream.InputJSON = ""
			if err := tx.SaveManjuScriptStage(downstream); err != nil {
				return err
			}
		}
		if err := tx.SupersedeManjuConsumersForScriptRun(projectID, runID); err != nil {
			return err
		}
		run.Status = model.ManjuScriptRunStatusActive
		run.CurrentStage = stageKey
		if err := tx.SaveManjuScriptRun(run); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.RunScriptStage(actorID, projectID, runID, stageKey, input)
}

func (s *Service) blockScriptStage(projectID, runID, stageKey, message string) {
	_ = s.repo.Transaction(func(tx *repository.Repository) error {
		stage, err := tx.ManjuScriptStage(runID, stageKey)
		if err != nil || stage == nil {
			return err
		}
		taskID := stage.TaskID
		stage.Status = model.ManjuScriptStageStatusBlocked
		stage.Error = strings.TrimSpace(message)
		stage.TaskID = ""
		stage.Version++
		if err := tx.CompleteManjuModelExchangeByTaskID(
			taskID, "", "", model.ManjuModelExchangeStatusFailed, message,
		); err != nil {
			return err
		}
		return tx.SaveManjuScriptStage(stage)
	})
}

func buildScriptStagePrompt(projectName string, run model.ManjuScriptRun, stage model.ManjuScriptStage, source *model.ManjuScriptSource, artifacts []model.ManjuScriptArtifact, receipt *model.ManjuLockReceipt, instruction, skillContext string) string {
	var out strings.Builder
	out.WriteString("你是 VisiDeep AI 漫剧工作室的编剧执行器，严格遵循漫剧老李 AIGC 全流程 V7.4.26。\n")
	out.WriteString("不得伪造已经审核通过、PASS、圆桌分数或未提供的上游事实。只输出本阶段 Markdown 成果，不输出思考过程。\n\n")
	out.WriteString("P0 画风只能使用可投喂视觉参数；交付稿不得出现导演名、工作室名、影片名或选择层卡号。\n\n")
	fmt.Fprintf(&out, "【项目】%s\n【支线】%s\n【模式】%s\n【阶段】%s（%s）\n\n", projectName, run.Branch, run.Mode, stage.Title, stage.Key)
	if receipt != nil {
		fmt.Fprintf(&out, "%s\n", p0DeliveryPrompt(receipt))
	}
	if source != nil {
		fmt.Fprintf(&out, "【来源账本】\n标题：%s\n授权范围：%s\n本次范围：%s\n内容哈希：%s\n正文：\n%s\n\n", source.Title, source.AuthorizationScope, source.RangeLabel, source.ContentHash, strings.TrimSpace(source.ContentText))
	}
	if len(artifacts) > 0 {
		appendLockedArtifactAuthority(&out, "P1 上游")
		out.WriteString("【已生成并锁定的上游工件】\n")
		for _, artifact := range artifacts {
			fmt.Fprintf(&out, "--- %s v%d ---\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
		}
		out.WriteString("\n")
	}
	if instruction != "" {
		fmt.Fprintf(&out, "【本次用户补充】\n%s\n\n", instruction)
	}
	appendManjuSkillContext(&out, skillContext)
	out.WriteString("\n\n【VisiDeep AI 运行接口】\n")
	out.WriteString(scriptStageInstruction(stage.Key))
	return out.String()
}

func scriptStageInstruction(stageKey string) string {
	_ = stageKey
	return "运行接口：严格按上方官方总控和强制读取原文执行当前阶段。只输出当前阶段官方规定的 Markdown 工件；不要输出规则复述、思考过程、流程回执或机检说明。"
}

func truncateScriptText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + fmt.Sprintf("\n[已截断，原长度 %d 字符]", len(runes))
}

func scriptArtifactKind(branch, stageKey string) string {
	switch stageKey {
	case ScriptStageReconIntelligence:
		return model.ManjuScriptArtifactReconCard
	case ScriptStagePolishDraft:
		return model.ManjuScriptArtifactPolishedScript
	case ScriptStagePolishFindings:
		return model.ManjuScriptArtifactPolishFindings
	case ScriptStagePolishConfirmation:
		return model.ManjuScriptArtifactPolishConfirm
	case ScriptStageIPDevelopment:
		return "IP_DEVELOPMENT.md"
	case ScriptStageSeriesBible:
		return "SERIES_BIBLE.md"
	case ScriptStageEpisodePlanning:
		return "EPISODE_PLAN.md"
	case ScriptStageScriptDraft:
		return "SCRIPT_DRAFT.md"
	case ScriptStageRoundtableReview:
		return model.ManjuScriptArtifactRoundtable
	default:
		if branch == ScriptBranchPolish {
			return model.ManjuScriptArtifactStageOutput
		}
		return model.ManjuScriptArtifactStageOutput
	}
}

func scriptStagePromptArtifacts(stageKey string, artifacts []model.ManjuScriptArtifact) []model.ManjuScriptArtifact {
	if stageKey != ScriptStageRoundtableReview {
		return artifacts
	}
	filtered := make([]model.ManjuScriptArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.Kind == model.ManjuScriptArtifactRoundtable {
			continue
		}
		filtered = append(filtered, artifact)
	}
	return filtered
}

func scriptStagePausesForConfirmation(run model.ManjuScriptRun, stage model.ManjuScriptStage) bool {
	if run.Mode != ScriptModeFullPipeline {
		return true
	}
	return stage.Key == ScriptStageReconIntelligence || stage.Key == ScriptStageRoundtableReview
}

func (s *Service) reconcileScriptStageTasks(actorID, projectID, runID string) (autoStageKey string, err error) {
	err = s.repo.Transaction(func(tx *repository.Repository) error {
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
		for index := range stages {
			stage := &stages[index]
			if strings.TrimSpace(stage.TaskID) == "" || stage.Status != model.ManjuScriptStageStatusInProgress {
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
				stage.Status = model.ManjuScriptStageStatusBlocked
				stage.Error = strings.TrimSpace(task.Error)
				if stage.Error == "" {
					stage.Error = "文本任务未成功完成"
				}
				stage.TaskID = ""
				stage.Version++
				if scriptStagePreparedVersion(stage.InputJSON) > 0 {
					stage.InputJSON = scriptStagePreparedVersionInput(stage.Version)
				}
				if err := tx.SaveManjuScriptStage(stage); err != nil {
					return err
				}
			case model.TaskStatusSucceeded:
				if strings.TrimSpace(stage.ArtifactID) != "" {
					artifact, err := scriptArtifactByID(tx, runID, stage.ArtifactID)
					if err != nil {
						return err
					}
					if artifact != nil && artifact.TaskID == task.ID {
						if stage.Key == ScriptStageRoundtableReview {
							if stage.StartedAt != nil && artifact.CreatedAt.Before(*stage.StartedAt) {
								stage.Status = model.ManjuScriptStageStatusInProgress
								stage.TaskID = ""
								stage.ArtifactID = ""
								stage.Error = "旧审核工件属于上一版剧本，请重新执行五席圆桌审核"
								stage.ConfirmedBy = ""
								stage.ConfirmedAt = nil
								stage.CompletedAt = nil
								if err := tx.SaveManjuScriptStage(stage); err != nil {
									return err
								}
								continue
							}
							completedAt := artifact.CreatedAt
							if task.CompletedAt != nil && task.CompletedAt.After(completedAt) {
								completedAt = *task.CompletedAt
							}
							stage.Status = model.ManjuScriptStageStatusAwaitingDecision
							stage.CompletedAt = &completedAt
							stage.Error = ""
							if err := tx.SaveManjuScriptStage(stage); err != nil {
								return err
							}
						}
						continue
					}
				}
				content, err := scriptTaskResultText(task.ResultJSON)
				if err != nil || strings.TrimSpace(content) == "" {
					stage.Status = model.ManjuScriptStageStatusBlocked
					stage.Error = "文本任务成功但没有可读取的正文"
					if saveErr := tx.SaveManjuScriptStage(stage); saveErr != nil {
						return saveErr
					}
					continue
				}
				now := time.Now()
				artifactVersion := stage.Version + 1
				if preparedVersion := scriptStagePreparedVersion(stage.InputJSON); preparedVersion > 0 {
					artifactVersion = preparedVersion
					stage.Version = preparedVersion
					stage.InputJSON = ""
				} else {
					stage.Version = artifactVersion
				}
				sum := sha256.Sum256([]byte(content))
				artifact := &model.ManjuScriptArtifact{
					ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
					Kind: scriptArtifactKind(run.Branch, stage.Key), Version: artifactVersion,
					Content: content, ContentHash: hex.EncodeToString(sum[:]), TaskID: task.ID,
					CreatedBy: actorID, CreatedAt: now,
				}
				if err := tx.CreateManjuScriptArtifact(artifact); err != nil {
					return err
				}
				if err := recordManjuArtifactAudit(
					tx, projectID, runID, stage.ID, artifact.ID, "P1", artifact.Kind,
					artifact.Version, model.ManjuArtifactVisibilityInternal,
					artifact.Content, artifact.ContentHash, artifact.TaskID, actorID,
				); err != nil {
					return err
				}
				stage.ArtifactID = artifact.ID
				stage.CompletedAt = &now
				stage.Error = ""
				if scriptStagePausesForConfirmation(*run, *stage) {
					if stage.Key == ScriptStageRoundtableReview {
						stage.Status = model.ManjuScriptStageStatusAwaitingDecision
					} else {
						stage.Status = model.ManjuScriptStageStatusAwaitingConfirm
					}
					if err := tx.SaveManjuScriptStage(stage); err != nil {
						return err
					}
					continue
				}
				if s.officialSkillValidationEnabled() && scriptProjectStateStageKey(stage.Key) != "" {
					project, projectErr := tx.ProjectByID(projectID)
					if projectErr != nil {
						return projectErr
					}
					if stateErr := commitManjuProjectStatePatch(tx, projectID, project.Name, run.LockRevision, scriptProjectStateStageKey(stage.Key), artifact.ID, content, actorID, false); stateErr != nil {
						if blockErr := blockScriptStageRecord(tx, stage, stateErr.Error()); blockErr != nil {
							return blockErr
						}
						continue
					}
				}
				stage.Status = model.ManjuScriptStageStatusLocked
				stage.ConfirmedAt = &now
				if err := tx.SaveManjuScriptStage(stage); err != nil {
					return err
				}
				next, err := activateNextScriptStage(tx, run, stage.Position)
				if err != nil {
					return err
				}
				run.CurrentStage = next
				autoStageKey = next
				if err := tx.SaveManjuScriptRun(run); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return autoStageKey, err
}

func scriptTaskResultText(raw string) (string, error) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Text), nil
}

func scriptArtifactTaskID(tx *repository.Repository, runID, artifactID string) (string, error) {
	artifact, err := scriptArtifactByID(tx, runID, artifactID)
	if err != nil || artifact == nil {
		return "", err
	}
	return artifact.TaskID, nil
}

func scriptArtifactByID(tx *repository.Repository, runID, artifactID string) (*model.ManjuScriptArtifact, error) {
	artifacts, err := tx.ManjuScriptArtifacts(runID)
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.ID == artifactID {
			return &artifact, nil
		}
	}
	return nil, nil
}

func roundtableReviewGrade(content string) (string, error) {
	rawGrades := make([]string, 0)
	for _, pattern := range roundtableGradePatterns {
		for _, matches := range pattern.FindAllStringSubmatch(content, -1) {
			if len(matches) >= 2 {
				rawGrades = append(rawGrades, matches[1])
			}
		}
	}
	if len(rawGrades) == 0 {
		return RoundtableGradeUnknown, ErrRoundtableGradeInvalid
	}
	grades := make(map[string]struct{}, len(rawGrades))
	for _, raw := range rawGrades {
		grade, ok := exactRoundtableGrade(raw)
		if !ok {
			return RoundtableGradeUnknown, ErrRoundtableGradeInvalid
		}
		grades[grade] = struct{}{}
	}
	if len(grades) != 1 {
		return RoundtableGradeUnknown, ErrRoundtableGradeInvalid
	}
	for grade := range grades {
		return grade, nil
	}
	return RoundtableGradeUnknown, ErrRoundtableGradeInvalid
}

func exactRoundtableGrade(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "\"'`")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "*")
	value = strings.ToUpper(strings.Join(strings.Fields(value), " "))
	switch value {
	case RoundtableGradeS, RoundtableGradeA, RoundtableGradeB, RoundtableGradeNotReady:
		return value, true
	case "NOT READY":
		return RoundtableGradeNotReady, true
	default:
		return "", false
	}
}

func blockScriptStageRecord(tx *repository.Repository, stage *model.ManjuScriptStage, message string) error {
	taskID := stage.TaskID
	stage.Status = model.ManjuScriptStageStatusBlocked
	stage.Error = strings.TrimSpace(message)
	stage.TaskID = ""
	if err := tx.CompleteManjuModelExchangeByTaskID(
		taskID, "", "", model.ManjuModelExchangeStatusFailed, message,
	); err != nil {
		return err
	}
	return tx.SaveManjuScriptStage(stage)
}

func scriptRoundtableLockedByDecision(tx *repository.Repository, projectID, runID string, stage model.ManjuScriptStage) (bool, error) {
	decisions, err := tx.ManjuRoundtableDecisionsForRun(projectID, runID)
	if err != nil {
		return false, err
	}
	for _, decision := range decisions {
		if decision.StageID == stage.ID &&
			decision.Round == stage.Version &&
			decision.ReviewArtifactID == stage.ArtifactID &&
			decision.Decision == string(RoundtableProceed) {
			return true, nil
		}
	}
	return false, nil
}

func scriptStageIndex(stages []model.ManjuScriptStage, key string) int {
	for index := range stages {
		if stages[index].Key == key {
			return index
		}
	}
	return -1
}

func (s *Service) ConfirmScriptStage(actorID, projectID, runID, stageKey string) (*ScriptRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	stageKey = strings.TrimSpace(stageKey)
	autoStageKey := ""
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canConfirmScript(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuScriptRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrScriptRunNotFound
		}
		if run.Status != model.ManjuScriptRunStatusActive &&
			run.Status != model.ManjuScriptRunStatusCompleted {
			return ErrInvalidScriptRun
		}
		stage, err := tx.ManjuScriptStage(runID, stageKey)
		if err != nil {
			return err
		}
		if stage == nil {
			return ErrScriptStageNotFound
		}
		if stage.Key == ScriptStageRoundtableReview {
			if stage.Status == model.ManjuScriptStageStatusLocked {
				lockedByDecision, err := scriptRoundtableLockedByDecision(tx, projectID, runID, *stage)
				if err != nil {
					return err
				}
				if lockedByDecision {
					return nil
				}
				return ErrRoundtableDecisionEndpointRequired
			}
			if stage.Status == model.ManjuScriptStageStatusAwaitingDecision {
				return ErrRoundtableDecisionRequired
			}
			return ErrRoundtableDecisionEndpointRequired
		}
		if stage.Status == model.ManjuScriptStageStatusLocked {
			return nil
		}
		if stage.Status != model.ManjuScriptStageStatusAwaitingConfirm {
			return ErrScriptStageNotReady
		}
		if strings.TrimSpace(stage.ArtifactID) == "" {
			return ErrScriptStageNotReady
		}
		content, err := scriptArtifactContentByID(tx, runID, stage.ArtifactID)
		if err != nil {
			return err
		}
		if s.officialSkillValidationEnabled() && scriptProjectStateStageKey(stage.Key) != "" {
			if err := commitManjuProjectStatePatch(tx, projectID, access.Project.Name, run.LockRevision, scriptProjectStateStageKey(stage.Key), stage.ArtifactID, content, actorID, false); err != nil {
				return err
			}
		}
		now := time.Now()
		stage.Status = model.ManjuScriptStageStatusLocked
		stage.ConfirmedBy = actorID
		stage.ConfirmedAt = &now
		stage.Error = ""
		if err := tx.SaveManjuScriptStage(stage); err != nil {
			return err
		}
		next, err := activateNextScriptStage(tx, run, stage.Position)
		if err != nil {
			return err
		}
		run.CurrentStage = next
		if run.Mode == ScriptModeFullPipeline && next != "" && next != ScriptStageSourceRegistration {
			autoStageKey = next
		}
		return tx.SaveManjuScriptRun(run)
	})
	if err != nil {
		return nil, err
	}
	if autoStageKey != "" {
		_, _ = s.RunScriptStage(actorID, projectID, runID, autoStageKey, ScriptStageRunInput{})
	}
	return s.ScriptRunDetail(actorID, projectID, runID)
}

func (s *Service) DecideRoundtable(actorID, projectID, runID string, input RoundtableDecisionInput) (*ScriptRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	input.Decision = RoundtableDecision(strings.TrimSpace(string(input.Decision)))
	input.Note = strings.TrimSpace(input.Note)
	if input.Decision != RoundtableProceed && input.Decision != RoundtableRefine {
		return nil, ErrRoundtableDecisionRequired
	}

	autoStageKey := ""
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canConfirmScript(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuScriptRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrScriptRunNotFound
		}
		if run.Status != model.ManjuScriptRunStatusActive {
			return ErrInvalidScriptRun
		}
		review, err := tx.ManjuScriptStage(runID, ScriptStageRoundtableReview)
		if err != nil {
			return err
		}
		if review == nil {
			return ErrScriptStageNotFound
		}
		if review.Status != model.ManjuScriptStageStatusAwaitingDecision ||
			strings.TrimSpace(review.ArtifactID) == "" {
			return ErrScriptStageNotReady
		}
		reviewArtifact, err := scriptArtifactByID(tx, runID, review.ArtifactID)
		if err != nil {
			return err
		}
		if reviewArtifact == nil {
			return ErrScriptStageNotReady
		}
		grade, err := roundtableReviewGrade(reviewArtifact.Content)
		if err != nil {
			return err
		}
		riskRequired := grade == RoundtableGradeB || grade == RoundtableGradeNotReady
		riskAccepted := input.Decision == RoundtableProceed && input.AcceptQualityRisk
		if input.Decision == RoundtableProceed && riskRequired && !riskAccepted {
			return ErrRoundtableQualityRiskRequired
		}
		now := time.Now()
		if err := tx.CreateManjuRoundtableDecision(&model.ManjuRoundtableDecision{
			ProjectID:          projectID,
			RunID:              runID,
			StageID:            review.ID,
			Round:              review.Version,
			Version:            reviewArtifact.Version,
			Decision:           string(input.Decision),
			ActorID:            actorID,
			Note:               input.Note,
			ReviewArtifactID:   reviewArtifact.ID,
			ReviewArtifactHash: reviewArtifact.ContentHash,
			Grade:              grade,
			RiskAccepted:       riskAccepted,
			CreatedAt:          now,
		}); err != nil {
			return err
		}

		switch input.Decision {
		case RoundtableProceed:
			review.Status = model.ManjuScriptStageStatusLocked
			review.ConfirmedBy = actorID
			review.ConfirmedAt = &now
			review.Error = ""
			if err := tx.SaveManjuScriptStage(review); err != nil {
				return err
			}
			next, err := activateNextScriptStage(tx, run, review.Position)
			if err != nil {
				return err
			}
			run.CurrentStage = next
			if run.Mode == ScriptModeFullPipeline && next != "" && next != ScriptStageSourceRegistration {
				autoStageKey = next
			}
			return tx.SaveManjuScriptRun(run)
		case RoundtableRefine:
			review.Status = model.ManjuScriptStageStatusStale
			review.ConfirmedBy = ""
			review.ConfirmedAt = nil
			review.Error = "用户选择精修或重写，等待新剧本版本"
			if err := tx.SaveManjuScriptStage(review); err != nil {
				return err
			}
			scriptDraft, err := tx.ManjuScriptStage(runID, ScriptStageScriptDraft)
			if err != nil {
				return err
			}
			if scriptDraft == nil {
				return ErrScriptStageNotFound
			}
			scriptDraft.Version++
			scriptDraft.Status = model.ManjuScriptStageStatusInProgress
			scriptDraft.TaskID = ""
			scriptDraft.Error = ""
			scriptDraft.ConfirmedBy = ""
			scriptDraft.ConfirmedAt = nil
			scriptDraft.CompletedAt = nil
			scriptDraft.StartedAt = &now
			scriptDraft.InputJSON = scriptStagePreparedVersionInput(scriptDraft.Version)
			if err := tx.SaveManjuScriptStage(scriptDraft); err != nil {
				return err
			}
			run.Status = model.ManjuScriptRunStatusActive
			run.CurrentStage = ScriptStageScriptDraft
			return tx.SaveManjuScriptRun(run)
		default:
			return ErrRoundtableDecisionRequired
		}
	})
	if err != nil {
		return nil, err
	}
	if autoStageKey != "" {
		_, _ = s.RunScriptStage(actorID, projectID, runID, autoStageKey, ScriptStageRunInput{})
	}
	return s.ScriptRunDetail(actorID, projectID, runID)
}

func scriptStagePreparedVersion(raw string) int64 {
	var input struct {
		PreparedVersion int64 `json:"preparedVersion"`
	}
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &input) != nil {
		return 0
	}
	return input.PreparedVersion
}

func scriptStagePreparedVersionInput(version int64) string {
	encoded, _ := json.Marshal(map[string]int64{"preparedVersion": version})
	return string(encoded)
}

func scriptStageDispatchVersion(stage model.ManjuScriptStage) int64 {
	if preparedVersion := scriptStagePreparedVersion(stage.InputJSON); preparedVersion > 0 {
		return preparedVersion
	}
	return stage.Version + 1
}

func markDownstreamScriptStagesStale(tx *repository.Repository, runID string, position int) error {
	stages, err := tx.ManjuScriptStages(runID)
	if err != nil {
		return err
	}
	for index := range stages {
		if stages[index].Position <= position {
			continue
		}
		if stages[index].Status == model.ManjuScriptStageStatusNotStarted {
			continue
		}
		stages[index].Status = model.ManjuScriptStageStatusStale
		stages[index].Error = "上游产物已更新，本阶段需要重新执行"
		if err := tx.SaveManjuScriptStage(&stages[index]); err != nil {
			return err
		}
	}
	return nil
}

func canConfirmScript(role string) bool {
	return role == model.ManjuMemberRoleOwner || role == model.ManjuMemberRoleProducer || role == model.ManjuMemberRoleWriter || role == model.ManjuMemberRoleReviewer
}

func (s *Service) ScriptRunDetail(actorID, projectID, runID string) (*ScriptRunDetail, error) {
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
	run, err := s.repo.ManjuScriptRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrScriptRunNotFound
	}
	if access.Role == "" {
		return nil, ErrForbidden
	}
	if err := s.repairLegacyChapterSourceStage(projectID, runID); err != nil {
		return nil, err
	}
	autoStageKey, err := s.reconcileScriptStageTasks(actorID, projectID, runID)
	if err != nil {
		return nil, err
	}
	if autoStageKey != "" && s.scriptTaskCreate != nil {
		_, _ = s.RunScriptStage(actorID, projectID, runID, autoStageKey, ScriptStageRunInput{})
	}
	run, err = s.repo.ManjuScriptRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrScriptRunNotFound
	}
	stages, err := s.repo.ManjuScriptStages(runID)
	if err != nil {
		return nil, err
	}
	source, err := s.repo.ManjuScriptSource(runID)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.repo.ManjuScriptArtifacts(runID)
	if err != nil {
		return nil, err
	}
	decisions, err := s.repo.ManjuRoundtableDecisionsForRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	artifactByID := make(map[string]model.ManjuScriptArtifact, len(artifacts))
	for _, artifact := range artifacts {
		artifactByID[artifact.ID] = artifact
	}
	detail := &ScriptRunDetail{
		Run:                 scriptRunView(*run),
		Stages:              make([]ScriptStageView, 0, len(stages)),
		Artifacts:           make([]ScriptArtifactView, 0, len(artifacts)),
		RoundtableDecisions: make([]RoundtableDecisionView, 0, len(decisions)),
	}
	if source != nil {
		view, err := scriptSourceView(*source)
		if err != nil {
			return nil, err
		}
		detail.Source = view
	}
	for _, stage := range stages {
		view := scriptStageView(stage)
		if stage.Key == ScriptStageRoundtableReview && stage.ArtifactID != "" {
			if artifact, ok := artifactByID[stage.ArtifactID]; ok {
				grade, gradeErr := roundtableReviewGrade(artifact.Content)
				if gradeErr == nil {
					view.ReviewGrade = grade
				} else {
					view.ReviewGrade = RoundtableGradeUnknown
				}
			}
		}
		if stage.Status == model.ManjuScriptStageStatusBlocked {
			draft, draftErr := s.repo.LatestManjuStageDraft(runID, stage.ID)
			if draftErr != nil {
				return nil, draftErr
			}
			view.Draft = stageDraftView(draft)
		}
		detail.Stages = append(detail.Stages, view)
	}
	for _, artifact := range artifacts {
		detail.Artifacts = append(detail.Artifacts, ScriptArtifactView{
			ID: artifact.ID, StageID: artifact.StageID, Kind: artifact.Kind, Version: artifact.Version,
			Content: artifact.Content, ContentHash: artifact.ContentHash, TaskID: artifact.TaskID, CreatedAt: artifact.CreatedAt,
		})
	}
	for _, decision := range decisions {
		detail.RoundtableDecisions = append(detail.RoundtableDecisions, RoundtableDecisionView{
			ID: decision.ID, ProjectID: decision.ProjectID, RunID: decision.RunID, StageID: decision.StageID,
			Round: decision.Round, Version: decision.Version, Decision: decision.Decision, ActorID: decision.ActorID,
			Note: decision.Note, ReviewArtifactID: decision.ReviewArtifactID, ReviewArtifactHash: decision.ReviewArtifactHash,
			Grade: decision.Grade, RiskAccepted: decision.RiskAccepted, CreatedAt: decision.CreatedAt,
		})
	}
	return detail, nil
}

func (s *Service) repairLegacyChapterSourceStage(projectID, runID string) error {
	return s.repo.Transaction(func(tx *repository.Repository) error {
		var runs []struct {
			ID               string
			Status           string
			CreatedBy        string
			SourceDocumentID string
		}
		if err := tx.DB().Table("manju_script_runs").Where("project_id = ?", projectID).Find(&runs).Error; err != nil {
			return err
		}
		runIndex := -1
		for index := range runs {
			if runs[index].ID == runID {
				runIndex = index
				break
			}
		}
		if runIndex < 0 || strings.TrimSpace(runs[runIndex].SourceDocumentID) == "" || runs[runIndex].Status != "ACTIVE" {
			return nil
		}
		var stages []struct {
			ID       string
			Key      string
			Status   string
			Position int
		}
		if err := tx.DB().Table("manju_script_stages").Where("run_id = ?", runID).Find(&stages).Error; err != nil {
			return err
		}
		stageIndex := -1
		for index := range stages {
			if stages[index].Key == ScriptStageSourceRegistration {
				stageIndex = index
				break
			}
		}
		if stageIndex < 0 || stages[stageIndex].Status != "AWAITING_CONFIRMATION" {
			return nil
		}
		now := time.Now()
		if err := tx.DB().Table("manju_script_stages").Where("id = ?", stages[stageIndex].ID).Updates(map[string]any{
			"status": "LOCKED", "confirmed_by": runs[runIndex].CreatedBy, "confirmed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		next := ""
		for index := range stages {
			if stages[index].Position <= stages[stageIndex].Position || stages[index].Status == "LOCKED" {
				continue
			}
			next = stages[index].Key
			if stages[index].Status == "NOT_STARTED" || stages[index].Status == "BLOCKED" || stages[index].Status == "STALE" {
				if err := tx.DB().Table("manju_script_stages").Where("id = ?", stages[index].ID).Updates(map[string]any{
					"status": "IN_PROGRESS", "started_at": now, "error": "", "updated_at": now,
				}).Error; err != nil {
					return err
				}
			}
			break
		}
		return tx.DB().Table("manju_script_runs").Where("id = ?", runs[runIndex].ID).Updates(map[string]any{
			"current_stage": next, "updated_at": now,
		}).Error
	})
}

func canEditScript(role string) bool {
	return role == model.ManjuMemberRoleOwner || role == model.ManjuMemberRoleProducer || role == model.ManjuMemberRoleWriter
}

func scriptBranchForInput(source, inputKind, trendResolution string) (string, error) {
	switch source {
	case SourceOriginal:
		return ScriptBranchOriginal, nil
	case SourceAdaptation:
		switch inputKind {
		case ScriptInputNovel, ScriptInputOutline:
			return ScriptBranchAdaptation, nil
		case ScriptInputFinished:
			return ScriptBranchPolish, nil
		}
	case SourceBenchmark:
		return ScriptBranchBenchmark, nil
	case SourceTrend:
		switch trendResolution {
		case TrendResolutionOriginal:
			return ScriptBranchOriginal, nil
		case TrendResolutionBenchmark:
			return ScriptBranchBenchmark, nil
		default:
			return "", ErrTrendResolutionRequired
		}
	}
	return "", ErrInvalidScriptRun
}

func scriptStageDefinitions(branch string) []scriptStageDefinition {
	pipeline := []scriptStageDefinition{
		{Key: ScriptStageSourceRegistration, Title: "来源登记"},
		{Key: ScriptStageIPDevelopment, Title: "IP 与角色开发"},
		{Key: ScriptStageSeriesBible, Title: "全剧开发圣经"},
		{Key: ScriptStageEpisodePlanning, Title: "分集规划"},
		{Key: ScriptStageScriptDraft, Title: "正式剧本"},
		{Key: ScriptStageRoundtableReview, Title: "五席圆桌审核"},
	}
	switch branch {
	case ScriptBranchBenchmark:
		return append([]scriptStageDefinition{{Key: ScriptStageReconIntelligence, Title: "对标情报卡"}}, pipeline...)
	case ScriptBranchPolish:
		return []scriptStageDefinition{
			{Key: ScriptStageSourceRegistration, Title: "外来剧本登记"},
			{Key: ScriptStagePolishDraft, Title: "补表演层与调度层"},
			{Key: ScriptStagePolishFindings, Title: "润色发现清单"},
			{Key: ScriptStagePolishConfirmation, Title: "润色回签"},
		}
	default:
		return pipeline
	}
}

func validateScriptSourceForBranch(branch string, input ScriptSourceInput) error {
	switch branch {
	case ScriptBranchAdaptation:
		if input.AuthorizationScope == "" || input.RangeLabel == "" {
			return fmt.Errorf("%w: 小说改编必须登记授权范围与本次章节/段落", ErrInvalidScriptSource)
		}
	case ScriptBranchBenchmark:
		if input.RangeLabel == "" {
			return fmt.Errorf("%w: 对标改编必须登记采风材料范围", ErrInvalidScriptSource)
		}
	case ScriptBranchPolish:
		if input.RangeLabel == "" {
			input.RangeLabel = "完整剧本"
		}
	}
	return nil
}

func activateNextScriptStage(tx *repository.Repository, run *model.ManjuScriptRun, currentPosition int) (string, error) {
	stages, err := tx.ManjuScriptStages(run.ID)
	if err != nil {
		return "", err
	}
	now := time.Now()
	for index := range stages {
		if stages[index].Position <= currentPosition {
			continue
		}
		if stages[index].Status == "LOCKED" {
			continue
		}
		if stages[index].Status == model.ManjuScriptStageStatusNotStarted || stages[index].Status == model.ManjuScriptStageStatusBlocked || stages[index].Status == model.ManjuScriptStageStatusStale {
			stages[index].Status = model.ManjuScriptStageStatusInProgress
			stages[index].StartedAt = &now
			stages[index].Error = ""
			stages[index].TaskID = ""
			stages[index].ArtifactID = ""
			stages[index].InputJSON = ""
			stages[index].ConfirmedBy = ""
			stages[index].ConfirmedAt = nil
			stages[index].CompletedAt = nil
			if err := tx.SaveManjuScriptStage(&stages[index]); err != nil {
				return "", err
			}
			return stages[index].Key, nil
		}
		return stages[index].Key, nil
	}
	run.Status = model.ManjuScriptRunStatusCompleted
	return "", nil
}

func scriptRunView(run model.ManjuScriptRun) ScriptRunView {
	var chapterIDs []string
	if strings.TrimSpace(run.SourceChapterIDsJSON) != "" {
		_ = json.Unmarshal([]byte(run.SourceChapterIDsJSON), &chapterIDs)
	}
	return ScriptRunView{
		ID: run.ID, ProjectID: run.ProjectID, Branch: run.Branch, Mode: run.Mode,
		InputKind: run.InputKind, TrendResolution: run.TrendResolution, Status: run.Status,
		CurrentStage: run.CurrentStage, LockRevision: run.LockRevision, SourceID: run.SourceID,
		UnitTitle: run.UnitTitle, SourceDocumentID: run.SourceDocumentID, SourceChapterIDs: chapterIDs,
		SourceRangeLabel: run.SourceRangeLabel, SourceSetHash: run.SourceSetHash,
		BaseRunID: run.BaseRunID, ReuseGlobalStages: run.ReuseGlobalStages,
		CreatedBy: run.CreatedBy, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt,
	}
}

func scriptSourceView(source model.ManjuScriptSource) (*ScriptSourceView, error) {
	var metadata map[string]string
	if strings.TrimSpace(source.MetadataJSON) != "" {
		if err := json.Unmarshal([]byte(source.MetadataJSON), &metadata); err != nil {
			return nil, fmt.Errorf("解析来源元数据失败: %w", err)
		}
	}
	return &ScriptSourceView{
		ID: source.ID, ProjectID: source.ProjectID, RunID: source.RunID, Branch: source.Branch,
		Title: source.Title, AuthorizationScope: source.AuthorizationScope, RangeLabel: source.RangeLabel,
		ContentHash: source.ContentHash, CharacterCount: source.CharacterCount, ExtractionStatus: source.ExtractionStatus,
		Metadata: metadata, CreatedBy: source.CreatedBy, CreatedAt: source.CreatedAt, UpdatedAt: source.UpdatedAt,
	}, nil
}

func scriptStageView(stage model.ManjuScriptStage) ScriptStageView {
	return ScriptStageView{
		ID: stage.ID, Key: stage.Key, Title: stage.Title, Position: stage.Position, Status: stage.Status,
		Version: stage.Version, TaskID: stage.TaskID, ArtifactID: stage.ArtifactID, Error: stage.Error,
		ConfirmedBy: stage.ConfirmedBy, ConfirmedAt: stage.ConfirmedAt, StartedAt: stage.StartedAt,
		CompletedAt: stage.CompletedAt, UpdatedAt: stage.UpdatedAt,
	}
}

func reuseGlobalScriptStages(tx *repository.Repository, baseRunID, targetRunID string, targetLockRevision int64, actorID string, now time.Time) error {
	type scriptStageRow struct {
		ID         string
		RunID      string
		Key        string
		Status     string
		Version    int64
		ArtifactID string
		Error      string
	}
	type scriptArtifactRow struct {
		ID          string
		ProjectID   string
		RunID       string
		StageID     string
		Kind        string
		Version     int64
		Content     string
		ContentHash string
		TaskID      string
		CreatedBy   string
		CreatedAt   time.Time
	}
	var baseStages []scriptStageRow
	if err := tx.DB().Table("manju_script_stages").Where("run_id = ?", baseRunID).Find(&baseStages).Error; err != nil {
		return err
	}
	var baseArtifacts []scriptArtifactRow
	if err := tx.DB().Table("manju_script_artifacts").Where("run_id = ?", baseRunID).Find(&baseArtifacts).Error; err != nil {
		return err
	}
	var targetStages []scriptStageRow
	if err := tx.DB().Table("manju_script_stages").Where("run_id = ?", targetRunID).Find(&targetStages).Error; err != nil {
		return err
	}
	for targetIndex := range targetStages {
		target := &targetStages[targetIndex]
		if target.Key != ScriptStageIPDevelopment && target.Key != ScriptStageSeriesBible {
			continue
		}
		baseStageIndex := -1
		for index := range baseStages {
			if baseStages[index].Key == target.Key && baseStages[index].Status == "LOCKED" && strings.TrimSpace(baseStages[index].ArtifactID) != "" {
				baseStageIndex = index
				break
			}
		}
		if baseStageIndex < 0 {
			continue
		}
		artifactIndex := -1
		for index := range baseArtifacts {
			if baseArtifacts[index].ID == baseStages[baseStageIndex].ArtifactID {
				artifactIndex = index
				break
			}
		}
		if artifactIndex < 0 {
			continue
		}
		copied := baseArtifacts[artifactIndex]
		copied.ID = uuid.NewString()
		copied.RunID = targetRunID
		copied.StageID = target.ID
		copied.Version = 1
		copied.CreatedBy = actorID
		copied.CreatedAt = now
		copied.TaskID = ""
		if err := tx.DB().Table("manju_script_artifacts").Create(&copied).Error; err != nil {
			return err
		}
		if err := tx.DB().Table("manju_script_stages").Where("id = ?", target.ID).Updates(map[string]any{
			"status": "LOCKED", "version": 1, "artifact_id": copied.ID,
			"confirmed_by": actorID, "confirmed_at": now, "completed_at": now,
			"started_at": now, "error": "", "updated_at": now,
		}).Error; err != nil {
			return err
		}
	}
	if targetLockRevision <= 0 {
		return ErrInvalidScriptRun
	}
	return nil
}

func projectSourceLibraryPrompt(tx *repository.Repository, projectID string) (string, error) {
	type documentRow struct {
		ID                 string
		Title              string
		AuthorizationScope string
		ContentHash        string
	}
	type chapterRow struct {
		DocumentID  string
		Position    int
		Title       string
		ContentText string
		ContentHash string
	}
	var documents []documentRow
	if err := tx.DB().Table("manju_source_documents").Where("project_id = ?", projectID).Order("created_at ASC").Find(&documents).Error; err != nil {
		return "", err
	}
	var chapters []chapterRow
	if err := tx.DB().Table("manju_source_chapters").Where("project_id = ?", projectID).Order("document_id ASC, position ASC").Find(&chapters).Error; err != nil {
		return "", err
	}
	documentByID := make(map[string]documentRow, len(documents))
	for _, document := range documents {
		documentByID[document.ID] = document
	}
	var out strings.Builder
	for _, chapter := range chapters {
		document := documentByID[chapter.DocumentID]
		fmt.Fprintf(&out, "\n--- %s / %s ---\n授权范围：%s\n章节哈希：%s\n%s\n",
			document.Title, chapter.Title, document.AuthorizationScope, chapter.ContentHash, strings.TrimSpace(chapter.ContentText))
	}
	return strings.TrimSpace(out.String()), nil
}
