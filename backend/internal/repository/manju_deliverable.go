package repository

import (
	"errors"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) CreateManjuDeliverables(items []model.ManjuDeliverable) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

func (r *Repository) ManjuDeliverablesForRun(projectID, runID string) ([]model.ManjuDeliverable, error) {
	var items []model.ManjuDeliverable
	err := r.db.
		Where("project_id = ? AND production_run_id = ?", projectID, runID).
		Order("created_at ASC").
		Find(&items).Error
	return items, err
}

func (r *Repository) LatestManjuDeliverables(projectID string) ([]model.ManjuDeliverable, error) {
	var latest model.ManjuDeliverable
	err := r.db.
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []model.ManjuDeliverable{}, nil
	}
	if err != nil {
		return nil, err
	}
	return r.ManjuDeliverablesForRun(projectID, latest.ProductionRunID)
}
