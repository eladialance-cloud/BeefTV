package database

import (
	"path/filepath"
	"testing"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openHostedTestDB 打开一个临时 SQLite 库，并在用例结束时释放文件句柄，
// 否则 Windows 上 TempDir 清理会因句柄占用而失败。
func openHostedTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

// 托管迁移必须保留托管表：这是与本地迁移最关键的行为差异。
func TestMigrateHostedSchemaKeepsHostedTables(t *testing.T) {
	db := openHostedTestDB(t, "hosted.db")
	if err := db.Exec("CREATE TABLE credit_accounts (id TEXT PRIMARY KEY, balance INTEGER)").Error; err != nil {
		t.Fatalf("建托管表失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if !db.Migrator().HasTable("credit_accounts") {
		t.Fatal("托管迁移不得删除 credit_accounts")
	}
}

func TestMigrateHostedSchemaIsIdempotent(t *testing.T) {
	db := openHostedTestDB(t, "hosted-twice.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("重复迁移必须幂等: %v", err)
	}
}

func TestRequireHostedSchemaFailsOnEmptyDatabase(t *testing.T) {
	db := openHostedTestDB(t, "empty.db")
	if err := RequireHostedSchema(db); err == nil {
		t.Fatal("空库必须报缺少核心表")
	}
}

// 托管表必须由托管迁移建立，且不能进入本地模型清单。
func TestMigrateHostedSchemaCreatesStorageLocations(t *testing.T) {
	db := openHostedTestDB(t, "storage-locations.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if !db.Migrator().HasTable("storage_locations") {
		t.Fatal("托管迁移必须建出 storage_locations")
	}
	for _, item := range LocalModels() {
		if _, ok := item.(*model.StorageLocation); ok {
			t.Fatal("storage_locations 属于托管表，不得进入 LocalModels")
		}
	}
}

// 账号与会话表同样只能由托管迁移建立，且不得进入本地模型清单。
func TestMigrateHostedSchemaCreatesAccountTables(t *testing.T) {
	db := openHostedTestDB(t, "accounts.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	for _, table := range []string{"users", "user_identities", "auth_sessions"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("托管迁移必须建出 %s", table)
		}
	}
	for _, item := range LocalModels() {
		switch item.(type) {
		case *model.User, *model.UserIdentity, *model.AuthSession:
			t.Fatalf("账号与会话表属于托管表，不得进入 LocalModels: %T", item)
		}
	}
}
