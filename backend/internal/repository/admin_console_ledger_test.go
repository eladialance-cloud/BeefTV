package repository

import (
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAdminLedgerPageJoinsTaskModelAndTokenCategory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-ledger.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Project{}, &model.Task{}, &model.CreditLedgerEntry{},
		&model.TaskCostCharge{}, &model.ModelChannel{}, &model.ChannelModel{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := db.Create(&model.User{
		ID: "user-1", Username: "alice", DisplayName: "Alice", Role: model.UserRoleUser,
		Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Project{
		ID: "project-1", UserID: "user-1", Name: "漫剧项目", Type: "canvas",
		Status: model.ProjectStatusActive, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ModelChannel{
		ID: "channel-1", Scope: model.ChannelScopeSystem, Enabled: true,
		Name: "ToAPIs", PublicAlias: "ToAPIs媒体", CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ChannelModel{
		ID: "model-1", ChannelID: "channel-1", ModelKey: "seedance-2",
		DisplayName: "Seedance 2", Capability: "video", Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Task{
		ID: "task-video", UserID: "user-1", ProjectID: "project-1",
		Type: "canvas_video", Status: model.TaskStatusSucceeded,
		Stage: "任务完成", Operation: "reference_to_video", Model: "seedance-2",
		ChannelModelID: "model-1", ProviderRequestID: "provider-task-1",
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TaskCostCharge{
		ID: "cost-1", TaskID: "task-video", UserID: "user-1", Attempt: 1,
		TaskType: "canvas_video", Capability: "video", BillingMode: "per_second",
		EstimatedCredits: 1500000000, ActualCredits: 1500000000, ActualQuantity: 15,
		Status: model.TaskCostStatusSettled, CreatedAt: now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditLedgerEntry{
		ID: "ledger-video", UserID: "user-1", IdempotencyKey: "task-video:settle",
		Kind: "consume", Stage: "settle", TaskID: "task-video",
		Amount: 1500000000, BalanceBefore: 2500000000, BalanceAfter: 1000000000,
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditLedgerEntry{
		ID: "ledger-adjust", UserID: "user-1", IdempotencyKey: "adjust:promo",
		Kind: "adjust", Stage: "credit", Amount: 100000000,
		BalanceBefore: 1000000000, BalanceAfter: 1100000000,
		OperatorID: "admin-1", Reason: "活动奖励", CreatedAt: now.Add(time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}

	repo := New(db)
	rows, total, err := repo.AdminLedgerPage(AdminLedgerFilter{Token: "video", Model: "seedance"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("video filter rows=%d total=%d", len(rows), total)
	}
	row := rows[0]
	if row.TokenName != "视频" || row.Model != "Seedance 2" || row.ChannelName != "ToAPIs媒体" {
		t.Fatalf("ledger classification = %+v", row)
	}
	if row.ProjectName != "漫剧项目" || row.TaskType != "canvas_video" ||
		row.TaskStatus != string(model.TaskStatusSucceeded) || row.ProviderRequestID != "provider-task-1" {
		t.Fatalf("task detail = %+v", row)
	}
	if row.ActualCredits != 1500000000 || row.CostStatus != model.TaskCostStatusSettled {
		t.Fatalf("task cost detail = %+v", row)
	}

	rows, total, err = repo.AdminLedgerPage(AdminLedgerFilter{Kind: "adjust"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].TokenName != "账户" || rows[0].Reason != "活动奖励" {
		t.Fatalf("manual adjustment = %+v total=%d", rows, total)
	}
}
