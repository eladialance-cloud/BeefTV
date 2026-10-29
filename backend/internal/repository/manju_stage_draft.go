package repository

import (
	"errors"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) CreateManjuStageDraft(draft *model.ManjuStageDraft) error {
	return r.db.Create(draft).Error
}

func (r *Repository) CreateManjuArtifactCorrection(correction *model.ManjuArtifactCorrection) error {
	return r.db.Create(correction).Error
}

func (r *Repository) ManjuStageDrafts(runID, stageID string) ([]model.ManjuStageDraft, error) {
	var items []model.ManjuStageDraft
	err := r.db.Where("run_id = ? AND stage_id = ?", runID, stageID).
		Order("version ASC").
		Find(&items).Error
	return items, err
}

func (r *Repository) LatestManjuStageDraft(runID, stageID string) (*model.ManjuStageDraft, error) {
	var item model.ManjuStageDraft
	err := r.db.Where("run_id = ? AND stage_id = ?", runID, stageID).
		Order("version DESC").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) NextManjuStageDraftVersion(runID, stageID string) (int, error) {
	var latest int
	if err := r.db.Model(&model.ManjuStageDraft{}).
		Where("run_id = ? AND stage_id = ?", runID, stageID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&latest).Error; err != nil {
		return 0, err
	}
	return latest + 1, nil
}
