package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"github.com/google/uuid"
)

func (r *Repository) CreateManjuRoundtableDecision(entry *model.ManjuRoundtableDecision) error {
	if entry == nil {
		return errors.New("manju roundtable decision entry is nil")
	}
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	return r.db.Create(entry).Error
}

func (r *Repository) ManjuRoundtableDecisionsForRun(projectID, runID string) ([]model.ManjuRoundtableDecision, error) {
	var entries []model.ManjuRoundtableDecision
	err := r.db.
		Where("project_id = ? AND run_id = ?", projectID, runID).
		Order("round ASC, created_at ASC").
		Find(&entries).Error
	return entries, err
}
