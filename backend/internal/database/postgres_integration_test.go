package database

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func freePort(t *testing.T) uint32 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("取空闲端口失败: %v", err)
	}
	defer listener.Close()
	return uint32(listener.Addr().(*net.TCPAddr).Port)
}

// 真实 PostgreSQL 上验证：驱动可用、托管迁移不删表、连接池按驱动分流。
// 需要 BEEFTV_TEST_POSTGRES=1 才运行。
func TestPostgresHostedSchemaMigration(t *testing.T) {
	if os.Getenv("BEEFTV_TEST_POSTGRES") != "1" {
		t.Skip("设置 BEEFTV_TEST_POSTGRES=1 才运行 PostgreSQL 集成测试")
	}
	port := freePort(t)
	// embedded-postgres 默认把归档放到用户主目录，本机只读；改到工作区内的临时目录。
	cacheDir := filepath.Join(t.TempDir(), "embedded-postgres")
	instance := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(port).
		Database("beeftv_test").
		Username("beeftv").
		Password("beeftv").
		Locale("C").
		CachePath(cacheDir).
		RuntimePath(t.TempDir()).
		StartTimeout(90 * time.Second))
	if err := instance.Start(); err != nil {
		t.Fatalf("启动内嵌 PostgreSQL 失败: %v", err)
	}
	defer func() { _ = instance.Stop() }()

	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=beeftv password=beeftv dbname=beeftv_test sslmode=disable", port)
	db, err := Open(Config{Driver: DriverPostgres, DSN: dsn})
	if err != nil {
		t.Fatalf("连接 PostgreSQL 失败: %v", err)
	}
	if err := ConfigurePool(db, DriverPostgres); err != nil {
		t.Fatalf("配置连接池失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接失败: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 25 {
		t.Fatalf("postgres 连接池上限应为 25，得到 %d", got)
	}
	if err := db.Exec("CREATE TABLE credit_accounts (id TEXT PRIMARY KEY, balance BIGINT)").Error; err != nil {
		t.Fatalf("建托管表失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if !db.Migrator().HasTable("credit_accounts") {
		t.Fatal("托管迁移不得删除 credit_accounts")
	}
	if !db.Migrator().HasTable("projects") {
		t.Fatal("托管迁移必须建出 projects 表")
	}
}
