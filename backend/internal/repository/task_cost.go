package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// TaskCostSettlement is the audit payload for closing a reserved charge as
// settled: the credits actually charged, the real usage quantity behind them,
// where that quantity came from, and whether the freeze capped it.
type TaskCostSettlement struct {
	ActualCredits  int64
	ActualQuantity int64
	Source         string
	Capped         bool
	At             time.Time
}

// TaskUsageSummary aggregates the provider usage that was recorded for one
// task across its API call logs. Settlement reads it instead of trusting the
// frozen estimate when the upstream actually reported usage.
type TaskUsageSummary struct {
	SucceededCalls int
	UsageAvailable bool
	InputTokens    int64
	OutputTokens   int64
	CachedTokens   int64
	MediaCount     int
	VideoSeconds   int
}

// TaskUsageSummary reads the succeeded API call logs for a task and folds them
// into one usage view. Media and video quantities only count the create call so
// polling the same provider job never double counts.
func (r *Repository) TaskUsageSummary(taskID string) (TaskUsageSummary, error) {
	var summary TaskUsageSummary
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return summary, nil
	}
	var row struct {
		Calls          int
		UsageAvailable int
		InputTokens    int64
		OutputTokens   int64
		CachedTokens   int64
		MediaCount     int
		VideoSeconds   int
	}
	err := r.db.Model(&model.ApiCallLog{}).
		Select(`COUNT(*) AS calls,
			COALESCE(MAX(CASE WHEN usage_available THEN 1 ELSE 0 END), 0) AS usage_available,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(cached_tokens), 0) AS cached_tokens,
			COALESCE(SUM(CASE WHEN request_kind = 'create' THEN media_count ELSE 0 END), 0) AS media_count,
			COALESCE(SUM(CASE WHEN request_kind = 'create' THEN video_seconds ELSE 0 END), 0) AS video_seconds`).
		Where("task_id = ? AND status = ?", taskID, model.ApiCallStatusSucceeded).
		Scan(&row).Error
	if err != nil {
		return summary, err
	}
	return TaskUsageSummary{
		SucceededCalls: row.Calls,
		UsageAvailable: row.UsageAvailable > 0,
		InputTokens:    row.InputTokens,
		OutputTokens:   row.OutputTokens,
		CachedTokens:   row.CachedTokens,
		MediaCount:     row.MediaCount,
		VideoSeconds:   row.VideoSeconds,
	}, nil
}

// CreateTaskCostCharge persists one attempt's pricing snapshot before the
// freeze is taken, so a crash between the two steps leaves a reserved row that
// reconciliation can release instead of a silently unfrozen task.
func (r *Repository) CreateTaskCostCharge(charge *model.TaskCostCharge) error {
	if charge == nil {
		return errors.New("任务计费快照不能为空")
	}
	return r.db.Create(charge).Error
}

// NextTaskCostAttempt returns the attempt number a new reservation should use:
// one more than the highest recorded attempt for the task.
func (r *Repository) NextTaskCostAttempt(taskID string) (int, error) {
	var attempt int
	err := r.db.Model(&model.TaskCostCharge{}).
		Where("task_id = ?", taskID).
		Select("COALESCE(MAX(attempt), 0)").
		Scan(&attempt).Error
	if err != nil {
		return 0, err
	}
	return attempt + 1, nil
}

// ReservedTaskCostCharge reads the single outstanding reservation for a task.
// found=false means the task has no unsettled charge: a legacy task admitted
// before billing existed, or an attempt already settled or released.
func (r *Repository) ReservedTaskCostCharge(taskID string) (*model.TaskCostCharge, bool, error) {
	var item model.TaskCostCharge
	err := r.db.
		Where("task_id = ? AND status = ?", taskID, model.TaskCostStatusReserved).
		Order("attempt DESC").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &item, true, nil
}

// SettleTaskCostCharge closes a reservation with the real usage. It is
// idempotent: a replayed call reports applied=false and touches nothing.
func (r *Repository) SettleTaskCostCharge(taskID string, attempt int, settlement TaskCostSettlement) (bool, error) {
	return r.closeTaskCostCharge(taskID, attempt, model.TaskCostStatusSettled, settlement, "")
}

// ReleaseTaskCostCharge returns a reservation to the wallet. It is idempotent
// in the same way as SettleTaskCostCharge.
func (r *Repository) ReleaseTaskCostCharge(taskID string, attempt int, reason string, at time.Time) (bool, error) {
	return r.closeTaskCostCharge(taskID, attempt, model.TaskCostStatusReleased, TaskCostSettlement{At: at}, reason)
}

func (r *Repository) closeTaskCostCharge(taskID string, attempt int, status string, settlement TaskCostSettlement, reason string) (bool, error) {
	updates := map[string]any{
		"status":         status,
		"actual_credits": settlement.ActualCredits,
	}
	if status == model.TaskCostStatusSettled {
		updates["settled_at"] = settlement.At.UTC()
		updates["actual_quantity"] = settlement.ActualQuantity
		updates["settlement_source"] = settlement.Source
		updates["capped_at_estimate"] = settlement.Capped
	} else {
		updates["released_at"] = settlement.At.UTC()
		updates["release_reason"] = reason
	}
	result := r.db.Model(&model.TaskCostCharge{}).
		Where("task_id = ? AND attempt = ? AND status = ?", taskID, attempt, model.TaskCostStatusReserved).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// DeleteTaskCostCharge removes a snapshot whose freeze never landed. It is used
// only to roll back a reservation that failed in the same submission.
func (r *Repository) DeleteTaskCostCharge(taskID string, attempt int) error {
	return r.db.
		Where("task_id = ? AND attempt = ? AND status = ?", taskID, attempt, model.TaskCostStatusReserved).
		Delete(&model.TaskCostCharge{}).Error
}
