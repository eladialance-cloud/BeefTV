package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) ManjuProductionPrevis(projectID, runID string) (*model.ManjuProductionPrevis, error) {
	var item model.ManjuProductionPrevis
	err := r.db.First(&item, "project_id = ? AND run_id = ?", projectID, runID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) SaveManjuProductionPrevis(item *model.ManjuProductionPrevis) error {
	item.UpdatedAt = time.Now()
	return r.db.Save(item).Error
}
