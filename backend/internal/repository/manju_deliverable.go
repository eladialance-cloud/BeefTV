package repository

import (
	"infinite-canvas/backend/internal/model"
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

func (r *Repository) ManjuDeliverablesForProject(projectID string) ([]model.ManjuDeliverable, error) {
	var items []model.ManjuDeliverable
	err := r.db.
		Where("project_id = ?", projectID).
		Order("created_at ASC").
		Find(&items).Error
	return items, err
}
