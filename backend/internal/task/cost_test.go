package task

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

type recordingCost struct {
	mu         sync.Mutex
	reserves   []CostReserveRequest
	settles    []CostSettleRequest
	releases   []CostReleaseRequest
	reserveErr error
	free       bool
}

func (c *recordingCost) ReserveTaskCost(request CostReserveRequest) (*CostReservation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reserves = append(c.reserves, request)
	if c.reserveErr != nil {
		return nil, c.reserveErr
	}
	if c.free {
		return &CostReservation{Free: true}, nil
	}
	return &CostReservation{Attempt: 1, EstimatedCredits: 3, RuleID: "rule-1"}, nil
}

func (c *recordingCost) SettleTaskCost(request CostSettleRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.settles = append(c.settles, request)
	return nil
}

func (c *recordingCost) ReleaseTaskCost(request CostReleaseRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releases = append(c.releases, request)
	return nil
}

func (c *recordingCost) reserved() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.reserves)
}

func (c *recordingCost) released() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.releases)
}

type countingPersist struct {
	inner Persistence
	calls atomic.Int32
	err   error
}

func (p *countingPersist) CreateAdmitted(task *model.Task, limit int) error {
	p.calls.Add(1)
	if p.err != nil {
		return p.err
	}
	return p.inner.CreateAdmitted(task, limit)
}

func TestAdmitReservesCostBeforePersist(t *testing.T) {
	store := newMemStore()
	cost := &recordingCost{}
	persist := &countingPersist{inner: store}
	svc := domainService(store, func(deps *Dependencies) {
		deps.Cost = cost
		deps.Persist = persist
	})

	if _, err := svc.CreateTask("u1", imageReq("a cat", "proposal:gp-cost")); err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if cost.reserved() != 1 {
		t.Fatalf("提交必须先冻结一次，得到 %d", cost.reserved())
	}
	if persist.calls.Load() != 1 {
		t.Fatalf("冻结成功后应入库一次，得到 %d", persist.calls.Load())
	}
}

func TestAdmitBlocksTaskWhenFreezeFails(t *testing.T) {
	store := newMemStore()
	cost := &recordingCost{reserveErr: kernel.QuotaExceeded("可用积分不足")}
	persist := &countingPersist{inner: store}
	svc := domainService(store, func(deps *Dependencies) {
		deps.Cost = cost
		deps.Persist = persist
	})

	if _, err := svc.CreateTask("u1", imageReq("a cat", "proposal:gp-poor")); err == nil {
		t.Fatal("余额不足时必须拒绝提交")
	}
	if persist.calls.Load() != 0 {
		t.Fatalf("余额不足时任务不能入库，入库调用 %d 次", persist.calls.Load())
	}
	tasks, err := store.List("u1", 10, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("余额不足时不应留下任务，得到 %d 条", len(tasks))
	}
}

func TestAdmitReleasesFreezeWhenPersistFails(t *testing.T) {
	store := newMemStore()
	cost := &recordingCost{}
	persist := &countingPersist{inner: store, err: errors.New("磁盘写入失败")}
	svc := domainService(store, func(deps *Dependencies) {
		deps.Cost = cost
		deps.Persist = persist
	})

	if _, err := svc.CreateTask("u1", imageReq("a cat", "proposal:gp-disk")); err == nil {
		t.Fatal("入库失败必须向上返回错误")
	}
	if cost.released() != 1 {
		t.Fatalf("入库失败必须释放冻结，得到 %d 次释放", cost.released())
	}
}

func TestTextReplayAdmissionSkipsFreeze(t *testing.T) {
	store := newMemStore()
	cost := &recordingCost{}
	svc := domainService(store, func(deps *Dependencies) {
		deps.Cost = cost
	})

	if _, err := svc.CreateTask("u1", textReplayReq("hello", "proposal:gp-replay")); err != nil {
		t.Fatalf("文本回放任务应正常创建: %v", err)
	}
	if cost.reserved() != 0 {
		t.Fatalf("前端自管的文本回放任务不应冻结积分，得到 %d", cost.reserved())
	}
}

func TestRetryReservesNewBillingAttempt(t *testing.T) {
	store := newMemStore()
	store.tasks["task-1"] = model.Task{
		ID: "task-1", UserID: "u1", ProjectID: "canvas-1", Type: "canvas_image",
		Status: model.TaskStatusFailed, Stage: "任务失败", Error: "上游失败",
		InputJSON: `{"prompt":"a cat"}`,
	}
	cost := &recordingCost{}
	svc := domainService(store, func(deps *Dependencies) {
		deps.Cost = cost
	})

	if _, err := svc.Retry("u1", "task-1"); err != nil {
		t.Fatalf("重试失败: %v", err)
	}
	cost.mu.Lock()
	reserves := append([]CostReserveRequest(nil), cost.reserves...)
	cost.mu.Unlock()
	if len(reserves) != 1 {
		t.Fatalf("重试必须重新冻结一次，得到 %d", len(reserves))
	}
	if !reserves[0].Retry {
		t.Fatal("重试冻结必须标记为新的计费尝试")
	}
	if cost.released() != 0 {
		t.Fatalf("重试成功不应释放冻结，得到 %d", cost.released())
	}
}

func TestCancelReleasesFreeze(t *testing.T) {
	store := newMemStore()
	cost := &recordingCost{}
	svc := domainService(store, func(deps *Dependencies) {
		deps.Cost = cost
	})
	created, err := svc.CreateTask("u1", imageReq("a cat", "proposal:gp-cancel"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Cancel(nil, "u1", created.ID); err != nil {
		t.Fatalf("取消任务失败: %v", err)
	}
	if cost.released() != 1 {
		t.Fatalf("取消必须释放冻结，得到 %d", cost.released())
	}
}
