package hostedadmin

import (
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAdminServiceTestEnv(t *testing.T) (*Service, *repository.Repository, *credits.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.UserIdentity{}, &model.AuthSession{},
		&model.CreditAccount{}, &model.CreditLedgerEntry{},
		&model.AdminAuditEvent{}, &model.AdminContentModeration{}, &model.SystemSetting{},
		&model.Project{}, &model.CanvasProject{}, &model.Asset{}, &model.Task{},
		&model.PaymentOrder{}, &model.Resource{}, &model.AssetVersion{}, &model.AssetRepresentation{},
	); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	creditService := credits.New(repo, credits.Config{})
	authService := auth.New(repo, auth.Config{})
	return New(repo, authService, creditService), repo, creditService
}

func seedAdmin(t *testing.T, repo *repository.Repository) *model.User {
	t.Helper()
	now := time.Now().UTC()
	user := &model.User{
		ID: "admin-1", Username: "admin", DisplayName: "Admin",
		Role: model.UserRoleAdmin, Status: model.UserStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func TestCreateUserGrantsInitialCreditsAndWritesAudit(t *testing.T) {
	svc, repo, creditService := newAdminServiceTestEnv(t)
	admin := seedAdmin(t, repo)

	view, err := svc.CreateUser(admin, CreateUserInput{
		Username: "new-user", DisplayName: "New User", Password: "password-1", InitialCredits: 25,
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}
	if view.Role != model.UserRoleUser || view.Status != model.UserStatusActive {
		t.Fatalf("新用户默认状态不符: %#v", view)
	}
	wallet, err := creditService.Account(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.Balance != credits.FromWholeCredits(25) {
		t.Fatalf("初始积分应为 25，得到 %d", wallet.Balance)
	}
	var auditCount int64
	if err := repo.DB().Model(&model.AdminAuditEvent{}).Where("action = ?", "user.create").Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("创建用户应写一条审计，得到 %d", auditCount)
	}
}

func TestUpdateUserProtectsLastActiveAdmin(t *testing.T) {
	svc, repo, _ := newAdminServiceTestEnv(t)
	admin := seedAdmin(t, repo)

	_, err := svc.UpdateUser(admin, admin.ID, UpdateUserInput{Role: model.UserRoleUser, Status: model.UserStatusActive})
	if err == nil {
		t.Fatal("最后一个管理员不得被降级")
	}
	_, err = svc.UpdateUser(admin, admin.ID, UpdateUserInput{Role: model.UserRoleAdmin, Status: model.UserStatusDisabled})
	if err == nil {
		t.Fatal("管理员不得停用自己")
	}
}

func TestModerateAssetRequiresReasonAndHidesAssetFromUserList(t *testing.T) {
	svc, repo, _ := newAdminServiceTestEnv(t)
	admin := seedAdmin(t, repo)
	now := time.Now().UTC()
	asset := &model.Asset{ID: "asset-1", UserID: admin.ID, Kind: "image", Title: "Blocked", CreatedAt: now, UpdatedAt: now}
	if err := repo.DB().Create(asset).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.ModerateAsset(admin, asset.ID, true, ""); err == nil {
		t.Fatal("下架素材必须填写原因")
	}
	if err := svc.ModerateAsset(admin, asset.ID, true, "违规内容"); err != nil {
		t.Fatalf("ModerateAsset error = %v", err)
	}
	items, total, err := repo.UserAssetsPage(admin.ID, 1, 20, repository.UserAssetPageFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("下架素材不应出现在用户素材库: total=%d items=%v", total, items)
	}
}

func TestSiteConfigValidatesAndPersists(t *testing.T) {
	svc, repo, _ := newAdminServiceTestEnv(t)
	admin := seedAdmin(t, repo)

	if _, err := svc.SaveSiteConfig(admin, SiteConfig{}); err == nil {
		t.Fatal("空站点名称必须被拒绝")
	}
	_, err := svc.SaveSiteConfig(admin, SiteConfig{SiteName: "VisiDeep AI", CreditName: "积分", Currency: "CNY"})
	if err != nil {
		t.Fatalf("SaveSiteConfig error = %v", err)
	}
	got, err := svc.SiteConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.SiteName != "VisiDeep AI" || got.CreditName != "积分" {
		t.Fatalf("站点配置回读不符: %#v", got)
	}
}

func TestAdminMutationsRequireAdmin(t *testing.T) {
	svc, _, _ := newAdminServiceTestEnv(t)
	user := &model.User{ID: "user-1", Username: "user", Role: model.UserRoleUser, Status: model.UserStatusActive}
	_, err := svc.CreateUser(user, CreateUserInput{Username: "other", Password: "password-1"})
	if err == nil || err.Error() != "需要管理员权限" {
		t.Fatalf("普通用户执行管理写操作必须被拒绝，得到 %v", err)
	}
}
