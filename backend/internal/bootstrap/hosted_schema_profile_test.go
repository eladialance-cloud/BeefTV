package bootstrap

import (
	"context"
	"testing"
)

// 托管形态跑在 SQLite（服务端默认驱动）上时必须建托管表：结构选择要跟随 profile，
// 不能因为驱动是 sqlite 就走本地迁移。本地迁移不建 users/user_identities/
// payment_orders，还会在 retire-hosted-schema 里把托管表删掉，结果是新库第一次
// 注册直接 500。
func TestServerSchemaOnSQLiteIncludesHostedTables(t *testing.T) {
	runtime, err := Open(context.Background(), Config{
		Profile:         ProfileServer,
		DataDir:         t.TempDir(),
		DatabaseDriver:  "sqlite",
		ListenAddr:      "127.0.0.1:0",
		AutoMigrate:     true,
		ShutdownTimeout: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	if err := runtime.Start(); err != nil {
		t.Fatal(err)
	}
	// 托管库不写本地结构台账，就绪判定必须走托管表清单，否则服务端永远 503。
	if !runtime.Ready() {
		t.Fatal("server runtime on sqlite must report ready after start")
	}
	for _, table := range []string{
		"users", "user_identities", "auth_sessions",
		"credit_accounts", "credit_ledger_entries", "payment_orders",
		"payment_provider_configs", "storage_locations", "task_cost_charges",
		"admin_audit_events",
	} {
		if !runtime.db.Migrator().HasTable(table) {
			t.Fatalf("server schema on sqlite is missing hosted table %s", table)
		}
	}
}
