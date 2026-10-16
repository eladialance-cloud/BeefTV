package task

import (
	"strings"

	"infinite-canvas/backend/internal/model"
)

// ReserveTaskCost prices the admitted task and freezes the estimate before the
// row is persisted. Retry starts a new billing attempt so the previous
// attempt's idempotency keys are never replayed. A nil cost port (desktop)
// reserves nothing and reports no error.
func (s *Service) ReserveTaskCost(userID string, task *model.Task, retry bool) (*CostReservation, error) {
	if s == nil || s.deps.Cost == nil || task == nil {
		return nil, nil
	}
	return s.deps.Cost.ReserveTaskCost(CostReserveRequest{
		UserID:         userID,
		TaskID:         task.ID,
		TaskType:       task.Type,
		Operation:      task.Operation,
		LogicalModelID: task.LogicalModelID,
		ChannelModelID: task.ChannelModelID,
		InputJSON:      task.InputJSON,
		Retry:          retry,
	})
}

// SettleTaskCost converts a succeeded task's freeze into actual usage. It is a
// no-op when billing is disabled or the task predates the cost ledger.
func (s *Service) SettleTaskCost(userID, taskID string, actualCredits int64, reason string) error {
	if s == nil || s.deps.Cost == nil {
		return nil
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	return s.deps.Cost.SettleTaskCost(CostSettleRequest{
		UserID:        userID,
		TaskID:        taskID,
		ActualCredits: actualCredits,
		Reason:        reason,
	})
}

// ReleaseTaskCost returns a failed or cancelled task's freeze to the wallet.
func (s *Service) ReleaseTaskCost(userID, taskID, reason string) error {
	if s == nil || s.deps.Cost == nil {
		return nil
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	return s.deps.Cost.ReleaseTaskCost(CostReleaseRequest{
		UserID: userID,
		TaskID: taskID,
		Reason: reason,
	})
}

// releaseReservation drops a freeze whose task never made it to disk. It is
// best effort: the submission error is the one the caller must surface.
func (s *Service) releaseReservation(userID, taskID string, reservation *CostReservation, reason string) {
	if reservation == nil || reservation.Free {
		return
	}
	_ = s.ReleaseTaskCost(userID, taskID, reason)
}
