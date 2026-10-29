package repository

import (
	"errors"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) CreateManjuProjectState(snapshot *model.ManjuProjectStateSnapshot) error {
	return r.db.Create(snapshot).Error
}

func (r *Repository) ProjectByID(projectID string) (*model.Project, error) {
	var project model.Project
	if err := r.db.First(&project, "id = ?", projectID).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *Repository) LatestManjuProjectState(projectID string, lockRevision int64) (*model.ManjuProjectStateSnapshot, error) {
	var snapshot model.ManjuProjectStateSnapshot
	query := r.db.Where("project_id = ?", projectID)
	if lockRevision > 0 {
		query = query.Where("lock_revision = ?", lockRevision)
	}
	err := query.Order("created_at DESC").First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *Repository) ManjuProjectStateByArtifact(
	projectID string,
	lockRevision int64,
	stageKey, artifactID string,
) (*model.ManjuProjectStateSnapshot, error) {
	var snapshot model.ManjuProjectStateSnapshot
	err := r.db.
		Where(
			"project_id = ? AND lock_revision = ? AND stage_key = ? AND artifact_id = ?",
			projectID,
			lockRevision,
			stageKey,
			artifactID,
		).
		Order("created_at DESC").
		First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}
