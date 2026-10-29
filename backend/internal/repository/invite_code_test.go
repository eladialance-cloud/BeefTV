package repository

import (
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newInviteCodeRepo(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:invite-codes-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserIdentity{}, &model.InviteCode{}, &model.InviteCodeRedemption{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return New(db), db
}

func TestConsumeInviteCodeCreatesUserAndDecrementsOnce(t *testing.T) {
	repo, db := newInviteCodeRepo(t)
	now := time.Now().UTC()
	code := model.InviteCode{
		ID:        "invite-1",
		Code:      "INVITE-ONE",
		Batch:     "test",
		MaxUses:   1,
		Status:    InviteCodeStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&code).Error; err != nil {
		t.Fatal(err)
	}

	firstUser := &model.User{ID: "user-1", Username: "first", DisplayName: "first", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	firstIdentity := &model.UserIdentity{ID: "identity-1", UserID: firstUser.ID, Provider: model.IdentityProviderPassword, Identifier: "first", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	if err := repo.ConsumeInviteCode("INVITE-ONE", firstUser, firstIdentity); err != nil {
		t.Fatalf("首次核销失败: %v", err)
	}

	var stored model.InviteCode
	if err := db.First(&stored, "id = ?", code.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.UsedCount != 1 {
		t.Fatalf("核销后次数应为 1，得到 %d", stored.UsedCount)
	}
	var users int64
	if err := db.Model(&model.User{}).Where("id = ?", firstUser.ID).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("首次核销后应创建用户，得到 %d 个", users)
	}

	secondUser := &model.User{ID: "user-2", Username: "second", DisplayName: "second", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	secondIdentity := &model.UserIdentity{ID: "identity-2", UserID: secondUser.ID, Provider: model.IdentityProviderPassword, Identifier: "second", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	err := repo.ConsumeInviteCode("INVITE-ONE", secondUser, secondIdentity)
	if !errors.Is(err, ErrInviteCodeExhausted) {
		t.Fatalf("同一邀请码超出次数应返回已用完，得到 %v", err)
	}
	var secondCount int64
	if err := db.Model(&model.User{}).Where("id = ?", secondUser.ID).Count(&secondCount).Error; err != nil {
		t.Fatal(err)
	}
	if secondCount != 0 {
		t.Fatal("次数用尽时不应创建第二个用户")
	}
}
