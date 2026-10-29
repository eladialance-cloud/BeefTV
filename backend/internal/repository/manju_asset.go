package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) CreateManjuAssetRun(run *model.ManjuAssetRun) error {
	return r.db.Create(run).Error
}

func (r *Repository) CreateManjuAssetStages(stages []model.ManjuAssetStage) error {
	if len(stages) == 0 {
		return nil
	}
	return r.db.Create(&stages).Error
}

func (r *Repository) ManjuAssetRun(projectID, runID string) (*model.ManjuAssetRun, error) {
	var run model.ManjuAssetRun
	err := r.db.First(&run, "id = ? AND project_id = ?", runID, projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) ManjuAssetRuns(projectID string) ([]model.ManjuAssetRun, error) {
	var runs []model.ManjuAssetRun
	err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&runs).Error
	return runs, err
}

func (r *Repository) SupersedeActiveManjuAssetRuns(projectID string) error {
	return r.db.Model(&model.ManjuAssetRun{}).
		Where("project_id = ? AND status = ?", projectID, model.ManjuAssetRunStatusActive).
		Update("status", model.ManjuAssetRunStatusSuperseded).Error
}

// SupersedeManjuConsumersForScriptRun retires P2/P3 runs that consumed a P1
// run whose artifact is being regenerated. Historical rows and media remain
// intact; only their status changes so stale consumers cannot be selected.
func (r *Repository) SupersedeManjuConsumersForScriptRun(projectID, scriptRunID string) error {
	var assetRunIDs []string
	if err := r.db.Model(&model.ManjuAssetRun{}).
		Where("project_id = ? AND script_run_id = ?", projectID, scriptRunID).
		Pluck("id", &assetRunIDs).Error; err != nil {
		return err
	}
	now := time.Now()
	if err := r.db.Model(&model.ManjuAssetRun{}).
		Where("project_id = ? AND id IN ? AND status IN ?", projectID, assetRunIDs, []string{
			model.ManjuAssetRunStatusActive,
			model.ManjuAssetRunStatusCompleted,
		}).
		Updates(map[string]any{
			"status": model.ManjuAssetRunStatusSuperseded, "current_stage": "", "updated_at": now,
		}).Error; err != nil {
		return err
	}
	if len(assetRunIDs) == 0 {
		return nil
	}
	return r.db.Model(&model.ManjuProductionRun{}).
		Where("project_id = ? AND asset_run_id IN ? AND status IN ?", projectID, assetRunIDs, []string{
			model.ManjuProductionRunStatusActive,
			model.ManjuProductionRunStatusCompleted,
		}).
		Updates(map[string]any{
			"status": model.ManjuProductionRunStatusSuperseded, "current_stage": "", "updated_at": now,
		}).Error
}

// SupersedeManjuProductionRunsForAssetRun retires P3-P5 consumers when a P2
// artifact changes. Artifacts, images, and videos stay readable as history.
func (r *Repository) SupersedeManjuProductionRunsForAssetRun(projectID, assetRunID string) error {
	return r.db.Model(&model.ManjuProductionRun{}).
		Where("project_id = ? AND asset_run_id = ? AND status IN ?", projectID, assetRunID, []string{
			model.ManjuProductionRunStatusActive,
			model.ManjuProductionRunStatusCompleted,
		}).
		Updates(map[string]any{
			"status": model.ManjuProductionRunStatusSuperseded, "current_stage": "", "updated_at": time.Now(),
		}).Error
}

func (r *Repository) AbandonManjuRunsBeforeLockRevision(projectID string, currentRevision int64) error {
	now := time.Now()
	if err := r.db.Model(&model.ManjuScriptRun{}).
		Where("project_id = ? AND status = ? AND lock_revision <> ?", projectID, model.ManjuScriptRunStatusActive, currentRevision).
		Updates(map[string]any{
			"status": model.ManjuScriptRunStatusAbandoned, "current_stage": "", "updated_at": now,
		}).Error; err != nil {
		return err
	}
	if err := r.db.Model(&model.ManjuAssetRun{}).
		Where("project_id = ? AND status = ? AND lock_revision <> ?", projectID, model.ManjuAssetRunStatusActive, currentRevision).
		Updates(map[string]any{
			"status": model.ManjuAssetRunStatusSuperseded, "current_stage": "", "updated_at": now,
		}).Error; err != nil {
		return err
	}
	return r.db.Model(&model.ManjuProductionRun{}).
		Where("project_id = ? AND status = ? AND lock_revision <> ?", projectID, model.ManjuProductionRunStatusActive, currentRevision).
		Updates(map[string]any{
			"status": model.ManjuProductionRunStatusSuperseded, "current_stage": "", "updated_at": now,
		}).Error
}

func (r *Repository) ManjuAssetStages(runID string) ([]model.ManjuAssetStage, error) {
	var stages []model.ManjuAssetStage
	err := r.db.Where("run_id = ?", runID).Order("position ASC").Find(&stages).Error
	return stages, err
}

func (r *Repository) ManjuAssetStage(runID, key string) (*model.ManjuAssetStage, error) {
	var stage model.ManjuAssetStage
	err := r.db.First(&stage, "run_id = ? AND key = ?", runID, key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stage, nil
}

func (r *Repository) SaveManjuAssetStage(stage *model.ManjuAssetStage) error {
	stage.UpdatedAt = time.Now()
	return r.db.Save(stage).Error
}

func (r *Repository) SaveManjuAssetRun(run *model.ManjuAssetRun) error {
	run.UpdatedAt = time.Now()
	return r.db.Save(run).Error
}

func (r *Repository) CreateManjuAssetArtifact(artifact *model.ManjuAssetArtifact) error {
	return r.db.Create(artifact).Error
}

func (r *Repository) ManjuAssetArtifacts(runID string) ([]model.ManjuAssetArtifact, error) {
	var artifacts []model.ManjuAssetArtifact
	err := r.db.Where("run_id = ?", runID).Order("created_at ASC").Find(&artifacts).Error
	return artifacts, err
}

func (r *Repository) UpsertManjuAssetReference(reference *model.ManjuAssetReference) error {
	now := time.Now()
	reference.CreatedAt = now
	reference.UpdatedAt = now
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "run_id"}, {Name: "code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"asset_type":  reference.AssetType,
			"resource_id": reference.ResourceID,
			"url":         reference.URL,
			"status":      reference.Status,
			"notes":       reference.Notes,
			"created_by":  reference.CreatedBy,
			"updated_at":  now,
		}),
	}).Create(reference).Error; err != nil {
		return err
	}
	stored, err := r.ManjuAssetReference(reference.RunID, reference.Code)
	if err != nil {
		return err
	}
	if stored != nil {
		*reference = *stored
	}
	return nil
}

func (r *Repository) ManjuAssetReference(runID, code string) (*model.ManjuAssetReference, error) {
	var reference model.ManjuAssetReference
	err := r.db.First(&reference, "run_id = ? AND code = ?", runID, code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &reference, nil
}

func (r *Repository) ManjuAssetReferences(runID string) ([]model.ManjuAssetReference, error) {
	var references []model.ManjuAssetReference
	err := r.db.Where("run_id = ?", runID).Order("created_at ASC").Find(&references).Error
	return references, err
}
