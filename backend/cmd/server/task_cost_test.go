package main

import (
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var taskCostTestTime = time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)

func newTaskCostEnv(t *testing.T) (*taskCostAdapter, *repository.Repository, *credits.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "task-cost.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(
		&model.CreditAccount{}, &model.CreditLedgerEntry{},
		&model.ModelPricing{}, &model.ChannelModelPriceTier{}, &model.LogicalModelPriceSKU{},
		&model.TaskCostCharge{},
	); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	wallet := credits.New(repo, credits.Config{})
	pricing := credits.NewPricing(repo, credits.Config{})
	adapter := newTaskCostAdapter(repo, wallet, pricing)
	adapter.now = func() time.Time { return taskCostTestTime }
	return adapter, repo, wallet
}

func seedFixedPriceRule(t *testing.T, repo *repository.Repository, ruleID string, unitPriceMicrocredits int64) {
	t.Helper()
	rule := &model.PricingRule{
		ID:                    ruleID,
		Kind:                  model.PricingKindModel,
		Name:                  "图片默认价",
		LogicalModelID:        "lm-1",
		Capability:            "image",
		BillingMode:           "fixed_request",
		UnitPriceMicrocredits: unitPriceMicrocredits,
		Enabled:               true,
		EffectiveFrom:         taskCostTestTime.Add(-time.Hour),
		CreatedAt:             taskCostTestTime,
		UpdatedAt:             taskCostTestTime,
	}
	if err := repo.CreatePricingRule(model.PricingKindModel, rule); err != nil {
		t.Fatal(err)
	}
}

func fundTestWallet(t *testing.T, wallet *credits.Service, userID string, amount int64) {
	t.Helper()
	if _, err := wallet.Grant(userID, amount, "admin", "测试发积分", "test:grant:"+userID); err != nil {
		t.Fatalf("发放积分失败: %v", err)
	}
}

func imageCostRequest(userID, taskID string) localtask.CostReserveRequest {
	return localtask.CostReserveRequest{
		UserID:         userID,
		TaskID:         taskID,
		TaskType:       "canvas_image",
		LogicalModelID: "lm-1",
		ChannelModelID: "cm-1",
		InputJSON:      `{"prompt":"hello"}`,
	}
}

func TestTaskCostReserveAndSettle(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	fundTestWallet(t, wallet, "u1", 100)
	seedFixedPriceRule(t, repo, "rule-image", 2_000_000)

	reservation, err := adapter.ReserveTaskCost(imageCostRequest("u1", "task-1"))
	if err != nil {
		t.Fatalf("冻结失败: %v", err)
	}
	if reservation == nil || reservation.Free {
		t.Fatalf("应产生付费冻结，得到 %#v", reservation)
	}
	if reservation.Attempt != 1 || reservation.EstimatedCredits != 2 {
		t.Fatalf("冻结快照不符: %#v", reservation)
	}
	account, err := wallet.Account("u1")
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 100 || account.Frozen != 2 {
		t.Fatalf("冻结后钱包应为 100/2，得到 %d/%d", account.Balance, account.Frozen)
	}

	if err := adapter.SettleTaskCost(localtask.CostSettleRequest{UserID: "u1", TaskID: "task-1", Reason: "完成"}); err != nil {
		t.Fatalf("结算失败: %v", err)
	}
	account, err = wallet.Account("u1")
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 98 || account.Frozen != 0 {
		t.Fatalf("结算后钱包应为 98/0，得到 %d/%d", account.Balance, account.Frozen)
	}
	if _, found, err := repo.ReservedTaskCostCharge("task-1"); err != nil || found {
		t.Fatalf("结算后不应再有未决冻结: found=%v err=%v", found, err)
	}

	// 再次结算必须幂等：没有未决冻结时是空操作，绝不能二次扣费。
	if err := adapter.SettleTaskCost(localtask.CostSettleRequest{UserID: "u1", TaskID: "task-1", Reason: "完成"}); err != nil {
		t.Fatalf("重复结算应无副作用: %v", err)
	}
	account, _ = wallet.Account("u1")
	if account.Balance != 98 {
		t.Fatalf("重复结算后余额应保持 98，得到 %d", account.Balance)
	}
}

func TestTaskCostReserveFailsClosedWithoutBalance(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	seedFixedPriceRule(t, repo, "rule-image", 5_000_000)

	if _, err := adapter.ReserveTaskCost(imageCostRequest("u1", "task-poor")); err == nil {
		t.Fatal("余额不足时冻结必须失败")
	}
	account, err := wallet.Account("u1")
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 0 || account.Frozen != 0 {
		t.Fatalf("失败冻结不应改动钱包，得到 %d/%d", account.Balance, account.Frozen)
	}
	if _, found, err := repo.ReservedTaskCostCharge("task-poor"); err != nil || found {
		t.Fatalf("失败冻结不应留下快照: found=%v err=%v", found, err)
	}
}

func TestTaskCostReleaseReturnsFreeze(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	fundTestWallet(t, wallet, "u1", 50)
	seedFixedPriceRule(t, repo, "rule-image", 10_000_000)

	if _, err := adapter.ReserveTaskCost(imageCostRequest("u1", "task-2")); err != nil {
		t.Fatalf("冻结失败: %v", err)
	}
	if err := adapter.ReleaseTaskCost(localtask.CostReleaseRequest{UserID: "u1", TaskID: "task-2", Reason: "任务失败"}); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	account, err := wallet.Account("u1")
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 50 || account.Frozen != 0 {
		t.Fatalf("释放后钱包应为 50/0，得到 %d/%d", account.Balance, account.Frozen)
	}
	if _, err := wallet.SettleAttempt("u1", "task-2", 1, 10, 10); err == nil {
		t.Fatal("已释放的冻结不能再被结算")
	}
}

func TestTaskCostSettlementKeepsSnapshotAfterPriceChange(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	fundTestWallet(t, wallet, "u1", 100)
	seedFixedPriceRule(t, repo, "rule-image", 2_000_000)

	if _, err := adapter.ReserveTaskCost(imageCostRequest("u1", "task-3")); err != nil {
		t.Fatalf("冻结失败: %v", err)
	}
	// 结算前管理员改价：老任务必须仍按提交时的快照结算。
	updated := &model.PricingRule{
		ID: "rule-image", Kind: model.PricingKindModel, Name: "图片默认价",
		LogicalModelID: "lm-1", Capability: "image", BillingMode: "fixed_request",
		UnitPriceMicrocredits: 9_000_000, Enabled: true,
		EffectiveFrom: taskCostTestTime.Add(-time.Hour), CreatedAt: taskCostTestTime, UpdatedAt: taskCostTestTime,
	}
	if err := repo.UpdatePricingRule(model.PricingKindModel, updated); err != nil {
		t.Fatal(err)
	}
	if err := adapter.SettleTaskCost(localtask.CostSettleRequest{UserID: "u1", TaskID: "task-3"}); err != nil {
		t.Fatalf("结算失败: %v", err)
	}
	account, _ := wallet.Account("u1")
	if account.Balance != 98 {
		t.Fatalf("应按 2 积分快照结算，余额应为 98，得到 %d", account.Balance)
	}
}

func TestTaskCostRetryUsesNewAttempt(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	fundTestWallet(t, wallet, "u1", 100)
	seedFixedPriceRule(t, repo, "rule-image", 3_000_000)

	first, err := adapter.ReserveTaskCost(imageCostRequest("u1", "task-4"))
	if err != nil {
		t.Fatalf("首次冻结失败: %v", err)
	}
	if first.Attempt != 1 {
		t.Fatalf("首次尝试应为 1，得到 %d", first.Attempt)
	}
	if err := adapter.ReleaseTaskCost(localtask.CostReleaseRequest{UserID: "u1", TaskID: "task-4", Reason: "失败"}); err != nil {
		t.Fatalf("释放失败: %v", err)
	}

	retryRequest := imageCostRequest("u1", "task-4")
	retryRequest.Retry = true
	second, err := adapter.ReserveTaskCost(retryRequest)
	if err != nil {
		t.Fatalf("重试冻结失败: %v", err)
	}
	if second.Attempt != 2 {
		t.Fatalf("重试尝试应为 2，得到 %d", second.Attempt)
	}
	account, _ := wallet.Account("u1")
	if account.Balance != 100 || account.Frozen != 3 {
		t.Fatalf("重试后钱包应为 100/3，得到 %d/%d", account.Balance, account.Frozen)
	}
	entries, err := wallet.Ledger("u1", 10)
	if err != nil {
		t.Fatal(err)
	}
	taskEntries := 0
	for _, entry := range entries {
		if entry.TaskID == "task-4" {
			taskEntries++
		}
	}
	if taskEntries != 3 {
		t.Fatalf("两次尝试应产生 reserve/release/reserve 三条任务流水，得到 %d", taskEntries)
	}
	if _, found, err := repo.ReservedTaskCostCharge("task-4"); err != nil || !found {
		t.Fatalf("重试后应存在一条未决快照: found=%v err=%v", found, err)
	}
}

func TestTaskCostLeavesLocalExecutorsAndUnboundTasksFree(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	fundTestWallet(t, wallet, "u1", 10)

	local := imageCostRequest("u1", "task-local")
	local.TaskType = model.TaskTypeTimelineRender
	reservation, err := adapter.ReserveTaskCost(local)
	if err != nil {
		t.Fatalf("本地执行器不应阻塞提交: %v", err)
	}
	if reservation == nil || !reservation.Free {
		t.Fatalf("本地执行器无价格规则时应免费，得到 %#v", reservation)
	}

	unbound := imageCostRequest("u1", "task-unbound")
	unbound.LogicalModelID = ""
	unbound.ChannelModelID = ""
	reservation, err = adapter.ReserveTaskCost(unbound)
	if err != nil {
		t.Fatalf("自定义渠道任务不应阻塞提交: %v", err)
	}
	if reservation == nil || !reservation.Free {
		t.Fatalf("未绑定托管模型的任务应免费，得到 %#v", reservation)
	}

	account, _ := wallet.Account("u1")
	if account.Balance != 10 || account.Frozen != 0 {
		t.Fatalf("免费任务不应改动钱包，得到 %d/%d", account.Balance, account.Frozen)
	}
	if _, found, err := repo.ReservedTaskCostCharge("task-local"); err != nil || found {
		t.Fatalf("免费任务不应写快照: found=%v err=%v", found, err)
	}
}

func TestTaskCostCapabilityMapping(t *testing.T) {
	cases := map[string]string{
		"canvas_image":             "image",
		"canvas_video":             "video",
		"video_generation":         "video",
		"canvas_text":              "text",
		"canvas_audio":             "audio",
		model.TaskTypeTimelineRender: "local",
		model.TaskTypeDepthCapture:   "local",
	}
	for taskType, want := range cases {
		if got := taskCostCapability(taskType); got != want {
			t.Errorf("taskCostCapability(%q) = %q, want %q", taskType, got, want)
		}
	}
}
