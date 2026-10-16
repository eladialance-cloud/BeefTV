package app

import localtask "infinite-canvas/backend/internal/task"

// SetTaskCostPort installs the hosted task-cost implementation. It is called by
// the server composition root before the HTTP server starts; the desktop
// profile never calls it, so the port stays nil and every charge is a no-op.
func (s *Service) SetTaskCostPort(port localtask.Cost) {
	if s == nil {
		return
	}
	s.taskCostMu.Lock()
	defer s.taskCostMu.Unlock()
	s.taskCostPort = port
}

func (s *Service) taskCost() localtask.Cost {
	if s == nil {
		return nil
	}
	s.taskCostMu.RLock()
	defer s.taskCostMu.RUnlock()
	return s.taskCostPort
}

// taskCostBinding is the late-bound adapter handed to the task domain. The
// domain is constructed before the hosted routes are registered, so the binding
// must read the port on every call instead of capturing it at wiring time.
type taskCostBinding struct{ s *Service }

func (b taskCostBinding) ReserveTaskCost(request localtask.CostReserveRequest) (*localtask.CostReservation, error) {
	port := b.s.taskCost()
	if port == nil {
		return nil, nil
	}
	return port.ReserveTaskCost(request)
}

func (b taskCostBinding) SettleTaskCost(request localtask.CostSettleRequest) error {
	port := b.s.taskCost()
	if port == nil {
		return nil
	}
	return port.SettleTaskCost(request)
}

func (b taskCostBinding) ReleaseTaskCost(request localtask.CostReleaseRequest) error {
	port := b.s.taskCost()
	if port == nil {
		return nil
	}
	return port.ReleaseTaskCost(request)
}

// taskCostLifecycleAdapter exposes the lifecycle-shaped settle/release calls
// used by the terminal coordinator without importing the credits package.
type taskCostLifecycleAdapter struct{ s *Service }

func (a taskCostLifecycleAdapter) SettleTaskCost(userID, taskID string, actualCredits int64, reason string) error {
	port := a.s.taskCost()
	if port == nil {
		return nil
	}
	return port.SettleTaskCost(localtask.CostSettleRequest{
		UserID: userID, TaskID: taskID, ActualCredits: actualCredits, Reason: reason,
	})
}

func (a taskCostLifecycleAdapter) ReleaseTaskCost(userID, taskID, reason string) error {
	port := a.s.taskCost()
	if port == nil {
		return nil
	}
	return port.ReleaseTaskCost(localtask.CostReleaseRequest{UserID: userID, TaskID: taskID, Reason: reason})
}
