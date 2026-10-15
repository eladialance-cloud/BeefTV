package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeDriverFallsBackToSQLite(t *testing.T) {
	if got := NormalizeDriver(""); got != DriverSQLite {
		t.Fatalf("空驱动应回退 sqlite，得到 %q", got)
	}
	if got := NormalizeDriver(" PostgreSQL "); got != DriverPostgres {
		t.Fatalf("驱动名应去空格并小写，得到 %q", got)
	}
	if !IsSQLite("") || IsSQLite("postgres") {
		t.Fatalf("IsSQLite 判定错误")
	}
}

func TestOpenPostgresRequiresDSN(t *testing.T) {
	if _, err := Open(Config{Driver: DriverPostgres}); err == nil {
		t.Fatal("postgres 缺少 DSN 时必须报错")
	}
}

func TestConfigurePoolSplitsByDriver(t *testing.T) {
	raw, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pool.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := ConfigurePool(raw, DriverSQLite); err != nil {
		t.Fatalf("sqlite 连接池配置失败: %v", err)
	}
	sqlDB, err := raw.DB()
	if err != nil {
		t.Fatalf("取底层连接失败: %v", err)
	}
	// Windows 下 TempDir 清理要求先释放 SQLite 文件句柄。
	t.Cleanup(func() { _ = sqlDB.Close() })
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("sqlite 必须单写连接，得到 %d", got)
	}
	if err := ConfigurePool(raw, DriverPostgres); err != nil {
		t.Fatalf("postgres 连接池配置失败: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 25 {
		t.Fatalf("postgres 连接池上限应为 25，得到 %d", got)
	}
}
