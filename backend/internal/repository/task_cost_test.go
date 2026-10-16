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
	if err := db.AutoMigrate(&model.TaskCostCharge{}, &model.ApiCallLog{}); err != nil {
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

	applied, err := repo.SettleTaskCostCharge("task-a", 1, TaskCostSettlement{ActualCredits: 5, At: time.Now().UTC()})
	if err != nil || !applied {
		t.Fatalf("首次结算应生效: applied=%v err=%v", applied, err)
	}
	if _, found, err := repo.ReservedTaskCostCharge("task-a"); err != nil || found {
		t.Fatalf("结算后不应再有未决快照: found=%v err=%v", found, err)
	}
	if applied, err := repo.SettleTaskCostCharge("task-a", 1, TaskCostSettlement{ActualCredits: 5, At: time.Now().UTC()}); err != nil || applied {
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

func seedTaskUsageLog(t *testing.T, repo *Repository, taskID, requestKind, capability string, log model.ApiCallLog) {
	t.Helper()
	log.ID = newRepositoryID()
	log.TaskID = taskID
	log.RequestKind = requestKind
	log.Capability = capability
	if log.Status == "" {
		log.Status = model.ApiCallStatusSucceeded
	}
	log.CreatedAt = time.Now().UTC()
	if err := repo.db.Create(&log).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTaskUsageSummaryAggregatesSucceededCalls(t *testing.T) {
	repo := newTaskCostRepo(t)
	seedTaskUsageLog(t, repo, "task-usage", "create", "text", model.ApiCallLog{
		InputTokens: 800, OutputTokens: 700, UsageAvailable: true,
	})
	seedTaskUsageLog(t, repo, "task-usage", "poll", "text", model.ApiCallLog{
		InputTokens: 100, OutputTokens: 100, UsageAvailable: true,
	})
	seedTaskUsageLog(t, repo, "task-usage", "create", "image", model.ApiCallLog{MediaCount: 2})
	seedTaskUsageLog(t, repo, "task-usage", "poll", "image", model.ApiCallLog{MediaCount: 2})
	seedTaskUsageLog(t, repo, "task-usage", "create", "video", model.ApiCallLog{VideoSeconds: 5})
	// A failed call must never contribute to the bill.
	seedTaskUsageLog(t, repo, "task-usage", "create", "text", model.ApiCallLog{
		Status: model.ApiCallStatusFailed, InputTokens: 9_999, OutputTokens: 9_999, UsageAvailable: true,
	})

	summary, err := repo.TaskUsageSummary("task-usage")
	if err != nil {
		t.Fatal(err)
	}
	if !summary.UsageAvailable || summary.InputTokens != 900 || summary.OutputTokens != 800 {
		t.Fatalf("token 汇总不符: %#v", summary)
	}
	if summary.MediaCount != 2 {
		t.Fatalf("媒体数量应只算 create，得到 %d", summary.MediaCount)
	}
	if summary.VideoSeconds != 5 {
		t.Fatalf("视频秒数不符，得到 %d", summary.VideoSeconds)
	}
	if summary.SucceededCalls != 5 {
		t.Fatalf("成功调用数应为 5，得到 %d", summary.SucceededCalls)
	}
}

func TestTaskUsageSummaryWithoutReceiptsIsEmpty(t *testing.T) {
	repo := newTaskCostRepo(t)
	summary, err := repo.TaskUsageSummary("task-none")
	if err != nil {
		t.Fatal(err)
	}
	if summary.SucceededCalls != 0 || summary.UsageAvailable || summary.MediaCount != 0 || summary.VideoSeconds != 0 {
		t.Fatalf("无调用时应为空汇总，得到 %#v", summary)
	}
}

func TestSettleTaskCostChargeRecordsUsageAudit(t *testing.T) {
	repo := newTaskCostRepo(t)
	seedTaskCostCharge(t, repo, "task-audit", 1, 40)
	applied, err := repo.SettleTaskCostCharge("task-audit", 1, TaskCostSettlement{
		ActualCredits: 17, ActualQuantity: 3, Source: "provider_usage", Capped: true, At: time.Now().UTC(),
	})
	if err != nil || !applied {
		t.Fatalf("结算应生效: applied=%v err=%v", applied, err)
	}
	var stored model.TaskCostCharge
	if err := repo.db.First(&stored, "task_id = ? AND attempt = ?", "task-audit", 1).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ActualCredits != 17 || stored.ActualQuantity != 3 || stored.SettlementSource != "provider_usage" || !stored.CappedAtEstimate {
		t.Fatalf("结算审计字段未落库: %#v", stored)
	}
}
