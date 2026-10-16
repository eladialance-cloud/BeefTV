package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

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
func (r *Repository) SettleTaskCostCharge(taskID string, attempt int, actual int64, at time.Time) (bool, error) {
	return r.closeTaskCostCharge(taskID, attempt, model.TaskCostStatusSettled, actual, "", at)
}

// ReleaseTaskCostCharge returns a reservation to the wallet. It is idempotent
// in the same way as SettleTaskCostCharge.
func (r *Repository) ReleaseTaskCostCharge(taskID string, attempt int, reason string, at time.Time) (bool, error) {
	return r.closeTaskCostCharge(taskID, attempt, model.TaskCostStatusReleased, 0, reason, at)
}

func (r *Repository) closeTaskCostCharge(taskID string, attempt int, status string, actual int64, reason string, at time.Time) (bool, error) {
	updates := map[string]any{
		"status":         status,
		"actual_credits": actual,
	}
	if status == model.TaskCostStatusSettled {
		updates["settled_at"] = at.UTC()
	} else {
		updates["released_at"] = at.UTC()
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
