package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) SaveManjuCanvasImport(item *model.ManjuCanvasImport) error {
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}, {Name: "stage"}},
		DoUpdates: clause.Assignments(map[string]any{
			"asset_run_id":      item.AssetRunID,
			"production_run_id": item.ProductionRunID,
			"canvas_id":         item.CanvasID,
			"package_hash":      item.PackageHash,
			"task_count":        item.TaskCount,
			"status":            item.Status,
			"created_by":        item.CreatedBy,
			"updated_at":        now,
		}),
	}).Create(item).Error; err != nil {
		return err
	}
	stored, err := r.ManjuCanvasImportForStage(item.ProjectID, item.Stage)
	if err != nil {
		return err
	}
	if stored != nil {
		*item = *stored
	}
	return nil
}

func (r *Repository) ManjuCanvasImportForStage(projectID, stage string) (*model.ManjuCanvasImport, error) {
	var item model.ManjuCanvasImport
	err := r.db.First(&item, "project_id = ? AND stage = ? AND status = ?", projectID, stage, model.ManjuCanvasImportActive).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) ManjuCanvasImports(projectID string) ([]model.ManjuCanvasImport, error) {
	var items []model.ManjuCanvasImport
	err := r.db.Where("project_id = ?", projectID).Order("updated_at DESC").Find(&items).Error
	return items, err
}

func (r *Repository) ManjuCanvasImportForRun(projectID, stage, productionRunID string) (*model.ManjuCanvasImport, error) {
	var item model.ManjuCanvasImport
	err := r.db.First(
		&item,
		"project_id = ? AND stage = ? AND production_run_id = ? AND status = ?",
		projectID,
		stage,
		productionRunID,
		model.ManjuCanvasImportActive,
	).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
