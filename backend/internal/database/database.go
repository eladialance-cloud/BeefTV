package database

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	Driver  string
	DSN     string
	DataDir string
}

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// NormalizeDriver 统一驱动名，空值按 sqlite 处理。
func NormalizeDriver(driver string) string {
	value := strings.ToLower(strings.TrimSpace(driver))
	switch value {
	case "", DriverSQLite:
		return DriverSQLite
	case DriverPostgres, "postgresql", "pg":
		return DriverPostgres
	default:
		return value
	}
}

// IsSQLite 判断是否走本地单文件库。
func IsSQLite(driver string) bool {
	return NormalizeDriver(driver) == DriverSQLite
}

// Open 按驱动建立连接。sqlite 使用本地文件，postgres 必须显式提供 DSN。
func Open(config Config) (*gorm.DB, error) {
	switch driver := NormalizeDriver(config.Driver); driver {
	case DriverSQLite:
		dsn := strings.TrimSpace(config.DSN)
		if dsn == "" {
			if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
				return nil, err
			}
			dsn = config.DataDir + "/open_ai_canvas.db?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on&_synchronous=NORMAL"
		}
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	case DriverPostgres:
		dsn := strings.TrimSpace(config.DSN)
		if dsn == "" {
			return nil, fmt.Errorf("postgres 需要 DATABASE_URL")
		}
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("不支持的数据库驱动：%s", driver)
	}
}

// ConfigurePool 按驱动设置连接池。sqlite 必须串行写；postgres 面向并发 web 用户，
// 连接数要有上限，避免长耗时生成请求打满服务端连接。
func ConfigurePool(db *gorm.DB, driver string) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if IsSQLite(driver) {
		// SQLite serializes writers. A single shared connection avoids intermittent
		// SQLITE_BUSY failures under concurrent autosave/task updates while WAL
		// still keeps reads cheap for this single-process desktop application.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		return nil
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return nil
}
