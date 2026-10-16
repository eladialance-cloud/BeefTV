package repository

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTaskCostRepo(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:task-cost-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TaskCostCharge{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return New(db)
}

func seedTaskCostCharge(t *testing.T, repo *Repository, taskID string, attempt int, credits int64) *model.TaskCostCharge {
	t.Helper()
	charge := &model.TaskCostCharge{
		ID:               newRepositoryID(),
		TaskID:           taskID,
		UserID:           "u1",
		Attempt:          attempt,
		TaskType:         "canvas_image",
		Capability:       "image",
		BillingMode:      "fixed_request",
		RuleID:           "rule-1",
		RuleKind:         model.PricingKindModel,
		Quantity:         1,
		EstimatedCredits: credits,
		Status:           model.TaskCostStatusReserved,
		CreatedAt:        time.Now().UTC(),
	}
	if err := repo.CreateTaskCostCharge(charge); err != nil {
		t.Fatal(err)
	}
	return charge
}

func TestNextTaskCostAttemptIncrementsPerTask(t *testing.T) {
	repo := newTaskCostRepo(t)
	if attempt, err := repo.NextTaskCostAttempt("task-a"); err != nil || attempt != 1 {
		t.Fatalf("空表首次尝试应为 1: attempt=%d err=%v", attempt, err)
	}
	seedTaskCostCharge(t, repo, "task-a", 1, 5)
	seedTaskCostCharge(t, repo, "task-b", 7, 5)
	if attempt, err := repo.NextTaskCostAttempt("task-a"); err != nil || attempt != 2 {
		t.Fatalf("task-a 下次尝试应为 2: attempt=%d err=%v", attempt, err)
	}
	if attempt, err := repo.NextTaskCostAttempt("task-b"); err != nil || attempt != 8 {
		t.Fatalf("task-b 下次尝试应为 8: attempt=%d err=%v", attempt, err)
	}
}

func TestReservedTaskCostChargeOnlySeesOutstandingFreeze(t *testing.T) {
	repo := newTaskCostRepo(t)
	seedTaskCostCharge(t, repo, "task-a", 1, 5)

	charge, found, err := repo.ReservedTaskCostCharge("task-a")
	if err != nil || !found {
		t.Fatalf("应读到未决快照: found=%v err=%v", found, err)
	}
	if charge.Attempt != 1 || charge.EstimatedCredits != 5 {
		t.Fatalf("快照内容不符: %#v", charge)
	}

	applied, err := repo.SettleTaskCostCharge("task-a", 1, 5, time.Now().UTC())
	if err != nil || !applied {
		t.Fatalf("首次结算应生效: applied=%v err=%v", applied, err)
	}
	if _, found, err := repo.ReservedTaskCostCharge("task-a"); err != nil || found {
		t.Fatalf("结算后不应再有未决快照: found=%v err=%v", found, err)
	}
	if applied, err := repo.SettleTaskCostCharge("task-a", 1, 5, time.Now().UTC()); err != nil || applied {
		t.Fatalf("重复结算必须幂等: applied=%v err=%v", applied, err)
	}
}

func TestReleaseTaskCostChargeRecordsReason(t *testing.T) {
	repo := newTaskCostRepo(t)
	seedTaskCostCharge(t, repo, "task-a", 2, 9)

	applied, err := repo.ReleaseTaskCostCharge("task-a", 2, "任务失败", time.Now().UTC())
	if err != nil || !applied {
		t.Fatalf("释放应生效: applied=%v err=%v", applied, err)
	}
	var stored model.TaskCostCharge
	if err := repo.db.First(&stored, "task_id = ? AND attempt = ?", "task-a", 2).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.TaskCostStatusReleased || stored.ReleaseReason != "任务失败" {
		t.Fatalf("释放状态未落库: %#v", stored)
	}
	if applied, err := repo.ReleaseTaskCostCharge("task-a", 2, "任务失败", time.Now().UTC()); err != nil || applied {
		t.Fatalf("重复释放必须幂等: applied=%v err=%v", applied, err)
	}
}

func TestDeleteTaskCostChargeOnlyRemovesReserved(t *testing.T) {
	repo := newTaskCostRepo(t)
	seedTaskCostCharge(t, repo, "task-a", 1, 4)

	if err := repo.DeleteTaskCostCharge("task-a", 1); err != nil {
		t.Fatalf("删除未决快照失败: %v", err)
	}
	var count int64
	if err := repo.db.Model(&model.TaskCostCharge{}).Where("task_id = ?", "task-a").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("未决快照应被删除，剩余 %d", count)
	}
}
