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

func TestCreateAuthSessionReplacesActiveSessionsForUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth-sessions.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	for _, user := range []model.User{
		{ID: "user-1", Username: "alice", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "user-2", Username: "bob", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := New(db)
	first := &model.AuthSession{ID: "session-1", UserID: "user-1", TokenHash: "hash-1", ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateAuthSession(first); err != nil {
		t.Fatal(err)
	}
	other := &model.AuthSession{ID: "session-other", UserID: "user-2", TokenHash: "hash-other", ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateAuthSession(other); err != nil {
		t.Fatal(err)
	}
	second := &model.AuthSession{ID: "session-2", UserID: "user-1", TokenHash: "hash-2", ExpiresAt: now.Add(time.Hour), CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)}
	if err := repo.CreateAuthSession(second); err != nil {
		t.Fatal(err)
	}

	var stored []model.AuthSession
	if err := db.Order("created_at, id").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 3 {
		t.Fatalf("sessions=%d", len(stored))
	}
	byID := make(map[string]model.AuthSession, len(stored))
	for _, item := range stored {
		byID[item.ID] = item
	}
	if byID["session-1"].RevokedAt == nil {
		t.Fatal("旧会话必须被撤销")
	}
	if byID["session-2"].RevokedAt != nil {
		t.Fatal("新会话必须保持有效")
	}
	if byID["session-other"].RevokedAt != nil {
		t.Fatal("其他用户的会话不得受影响")
	}
}
