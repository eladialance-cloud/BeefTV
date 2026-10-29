package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) UpsertManjuRuleSnapshot(snapshot *model.ManjuRuleSnapshot) error {
	if snapshot == nil {
		return errors.New("manju rule snapshot is nil")
	}
	if snapshot.ID == "" {
		snapshot.ID = uuid.NewString()
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now()
	}
	if err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "lock_revision"}},
		DoNothing: true,
	}).Create(snapshot).Error; err != nil {
		return err
	}
	stored, err := r.ManjuRuleSnapshot(snapshot.ProjectID, snapshot.LockRevision)
	if err != nil {
		return err
	}
	*snapshot = *stored
	return nil
}

func (r *Repository) ManjuRuleSnapshot(projectID string, lockRevision int64) (*model.ManjuRuleSnapshot, error) {
	var snapshot model.ManjuRuleSnapshot
	err := r.db.First(&snapshot, "project_id = ? AND lock_revision = ?", projectID, lockRevision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *Repository) CreateManjuRuleReadReceipts(receipts []model.ManjuRuleReadReceipt) error {
	if len(receipts) == 0 {
		return nil
	}
	now := time.Now()
	for index := range receipts {
		if receipts[index].ID == "" {
			receipts[index].ID = uuid.NewString()
		}
		if receipts[index].CreatedAt.IsZero() {
			receipts[index].CreatedAt = now
		}
	}
	return r.db.Create(&receipts).Error
}

func (r *Repository) ManjuRuleReadReceiptsForStage(projectID, runID, stageID string) ([]model.ManjuRuleReadReceipt, error) {
	var receipts []model.ManjuRuleReadReceipt
	err := r.db.
		Where("project_id = ? AND run_id = ? AND stage_id = ?", projectID, runID, stageID).
		Order("created_at ASC").
		Find(&receipts).Error
	return receipts, err
}

func (r *Repository) ManjuRuleReadReceiptsForRequest(requestID string) ([]model.ManjuRuleReadReceipt, error) {
	var receipts []model.ManjuRuleReadReceipt
	err := r.db.Where("request_id = ?", requestID).Order("created_at ASC").Find(&receipts).Error
	return receipts, err
}

func (r *Repository) CreateManjuModelExchange(exchange *model.ManjuModelExchange) error {
	if exchange == nil {
		return errors.New("manju model exchange is nil")
	}
	if exchange.ID == "" {
		exchange.ID = uuid.NewString()
	}
	if exchange.Status == "" {
		exchange.Status = model.ManjuModelExchangeStatusQueued
	}
	if exchange.CreatedAt.IsZero() {
		exchange.CreatedAt = time.Now()
	}
	return r.db.Create(exchange).Error
}

func (r *Repository) CompleteManjuModelExchange(requestID, taskID, rawResponse, responseSHA256, status, failure string) error {
	now := time.Now()
	updates := map[string]any{
		"task_id":         taskID,
		"raw_response":    rawResponse,
		"response_sha256": responseSHA256,
		"status":          status,
		"error":           failure,
		"completed_at":    now,
	}
	return r.db.Model(&model.ManjuModelExchange{}).Where("request_id = ?", requestID).Updates(updates).Error
}

func (r *Repository) AttachManjuModelExchangeTask(requestID, taskID string) error {
	return r.db.Model(&model.ManjuModelExchange{}).
		Where("request_id = ?", requestID).
		Updates(map[string]any{"task_id": taskID, "status": model.ManjuModelExchangeStatusRunning}).Error
}

func (r *Repository) CompleteManjuModelExchangeByTaskID(taskID, rawResponse, responseSHA256, status, failure string) error {
	if taskID == "" {
		return nil
	}
	now := time.Now()
	return r.db.Model(&model.ManjuModelExchange{}).
		Where("task_id = ?", taskID).
		Updates(map[string]any{
			"raw_response":    rawResponse,
			"response_sha256": responseSHA256,
			"status":          status,
			"error":           failure,
			"completed_at":    now,
		}).Error
}

func (r *Repository) CreateManjuArtifactLedger(entry *model.ManjuArtifactLedger) error {
	if entry == nil {
		return errors.New("manju artifact ledger entry is nil")
	}
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.Visibility == "" {
		entry.Visibility = model.ManjuArtifactVisibilityInternal
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "artifact_id"}},
		DoNothing: true,
	}).Create(entry).Error
}

func (r *Repository) ManjuArtifactLedgerForProject(projectID string) ([]model.ManjuArtifactLedger, error) {
	var entries []model.ManjuArtifactLedger
	err := r.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&entries).Error
	return entries, err
}

func (r *Repository) ManjuUserVisibleArtifacts(projectID string) ([]model.ManjuArtifactLedger, error) {
	var entries []model.ManjuArtifactLedger
	err := r.db.
		Where("project_id = ? AND visibility = ?", projectID, model.ManjuArtifactVisibilityUser).
		Order("created_at ASC").
		Find(&entries).Error
	return entries, err
}
