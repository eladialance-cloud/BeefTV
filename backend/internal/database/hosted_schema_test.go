package database

import (
	"math"
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

func TestMigrateHostedSchemaUpgradesPaymentNotificationIndex(t *testing.T) {
	db := openHostedTestDB(t, "payment-notification-index.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if err := db.Migrator().DropIndex(&model.PaymentNotification{}, "idx_payment_notification_provider_id"); err != nil {
		t.Fatalf("删除复合索引失败: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_payment_notification_provider_id ON payment_notifications(provider_notification_id)").Error; err != nil {
		t.Fatalf("模拟旧单列索引失败: %v", err)
	}

	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("升级迁移失败: %v", err)
	}
	indexes, err := db.Migrator().GetIndexes(&model.PaymentNotification{})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if index.Name() != "idx_payment_notification_provider_id" {
			continue
		}
		columns := index.Columns()
		if len(columns) != 2 || ((columns[0] != "provider" || columns[1] != "provider_notification_id") &&
			(columns[0] != "provider_notification_id" || columns[1] != "provider")) {
			t.Fatalf("升级后必须为复合索引，得到 %v", columns)
		}
		if err := db.Create(&model.PaymentNotification{ID: "n-1", Provider: model.PaymentProviderAlipay, ProviderNotificationID: "notify-1"}).Error; err != nil {
			t.Fatalf("写入支付宝通知失败: %v", err)
		}
		if err := db.Create(&model.PaymentNotification{ID: "n-2", Provider: model.PaymentProviderWeChat, ProviderNotificationID: "notify-1"}).Error; err != nil {
			t.Fatalf("不同渠道使用同一通知号应被允许: %v", err)
		}
		return
	}
	t.Fatal("升级后缺少支付回调唯一索引")
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
// 积分账户与流水同样只能由托管迁移建立，且不得进入本地模型清单。
func TestMigrateHostedSchemaCreatesCreditTables(t *testing.T) {
	db := openHostedTestDB(t, "credits.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	for _, table := range []string{"credit_accounts", "credit_ledger_entries"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("托管迁移必须建出 %s", table)
		}
	}
	for _, item := range LocalModels() {
		switch item.(type) {
		case *model.CreditAccount, *model.CreditLedgerEntry:
			t.Fatalf("积分表属于托管表，不得进入 LocalModels: %T", item)
		}
	}
}

func TestMigrateHostedSchemaConvertsLegacyCreditValuesToMicrocreditsOnce(t *testing.T) {
	db := openHostedTestDB(t, "credit-microcredits.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if err := db.Where("version = ?", hostedCreditMicrocreditsMigrationVersion).Delete(&hostedSchemaMigration{}).Error; err != nil {
		t.Fatalf("删除迁移标记失败: %v", err)
	}
	if err := db.Create(&model.CreditAccount{
		ID: "wallet-legacy", UserID: "user-legacy", Balance: 10, Frozen: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditLedgerEntry{
		ID: "ledger-legacy", UserID: "user-legacy", IdempotencyKey: "legacy:grant",
		Kind: "grant", Stage: "credit", Amount: 10, BalanceBefore: 0, BalanceAfter: 10,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ChannelModelPriceTier{
		ID: "tier-legacy", ChannelID: "channel-1", ChannelModelID: "model-1",
		Capability: "image", BillingMode: "per_image", MinimumChargeCredits: 1,
		UnitPriceMicrocredits: 250000, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("升级微积分失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("重复升级必须幂等: %v", err)
	}

	var account model.CreditAccount
	if err := db.First(&account, "id = ?", "wallet-legacy").Error; err != nil {
		t.Fatal(err)
	}
	if account.Balance != 10_000_000 || account.Frozen != 2_000_000 {
		t.Fatalf("账户余额应按百万倍迁移一次，得到 %#v", account)
	}
	var entry model.CreditLedgerEntry
	if err := db.First(&entry, "id = ?", "ledger-legacy").Error; err != nil {
		t.Fatal(err)
	}
	if entry.Amount != 10_000_000 || entry.BalanceAfter != 10_000_000 {
		t.Fatalf("流水金额应按百万倍迁移一次，得到 %#v", entry)
	}
	var tier model.ChannelModelPriceTier
	if err := db.First(&tier, "id = ?", "tier-legacy").Error; err != nil {
		t.Fatal(err)
	}
	if tier.MinimumChargeCredits != 1_000_000 || tier.UnitPriceMicrocredits != 250000 {
		t.Fatalf("最低扣费应迁移且单价不得重复放大，得到 %#v", tier)
	}
}

func TestMigrateHostedSchemaCapsUnrepresentableLegacyCreditValues(t *testing.T) {
	db := openHostedTestDB(t, "credit-microcredits-overflow.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if err := db.Where("version = ?", hostedCreditMicrocreditsMigrationVersion).Delete(&hostedSchemaMigration{}).Error; err != nil {
		t.Fatalf("删除迁移标记失败: %v", err)
	}
	const oversized = int64(10_000_000_000_000_000)
	if err := db.Create(&model.CreditAccount{
		ID: "wallet-overflow", UserID: "user-overflow", Balance: oversized, Frozen: oversized,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CreditLedgerEntry{
		ID: "ledger-overflow", UserID: "user-overflow", IdempotencyKey: "legacy:overflow",
		Kind: "grant", Stage: "credit", Amount: oversized, BalanceAfter: oversized,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("极端历史余额不能让托管迁移失败: %v", err)
	}
	var account model.CreditAccount
	if err := db.First(&account, "id = ?", "wallet-overflow").Error; err != nil {
		t.Fatal(err)
	}
	if account.Balance != math.MaxInt64 || account.Frozen != math.MaxInt64 {
		t.Fatalf("不可表示的账户余额应封顶到 bigint 上限，得到 %#v", account)
	}
	var entry model.CreditLedgerEntry
	if err := db.First(&entry, "id = ?", "ledger-overflow").Error; err != nil {
		t.Fatal(err)
	}
	if entry.Amount != math.MaxInt64 || entry.BalanceAfter != math.MaxInt64 {
		t.Fatalf("不可表示的流水金额应封顶到 bigint 上限，得到 %#v", entry)
	}
}

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

// 支付渠道配置表同样只能由托管迁移建立，且不得进入本地模型清单。
func TestMigrateHostedSchemaCreatesPaymentProviderTables(t *testing.T) {
	db := openHostedTestDB(t, "payment-providers.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if !db.Migrator().HasTable("payment_provider_configs") {
		t.Fatal("托管迁移必须建出 payment_provider_configs")
	}
	for _, table := range []string{"topup_products", "payment_orders", "payment_notifications"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("托管迁移必须建出 %s", table)
		}
	}
	for _, item := range LocalModels() {
		if _, ok := item.(*model.PaymentProviderConfig); ok {
			t.Fatal("payment_provider_configs 属于托管表，不得进入 LocalModels")
		}
		switch item.(type) {
		case *model.TopupProduct, *model.PaymentOrder, *model.PaymentNotification:
			t.Fatalf("充值订单表属于托管表，不得进入 LocalModels: %T", item)
		}
	}
}

// 价格规则表同样只能由托管迁移建立，且不得进入本地模型清单。
func TestMigrateHostedSchemaCreatesPricingTables(t *testing.T) {
	db := openHostedTestDB(t, "pricing.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	for _, table := range []string{"model_pricings", "channel_model_price_tiers", "logical_model_price_skus"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("托管迁移必须建出 %s", table)
		}
	}
	if !db.Migrator().HasTable("task_cost_charges") {
		t.Fatal("托管迁移必须建出任务计费快照表 task_cost_charges")
	}
	for _, item := range LocalModels() {
		switch item.(type) {
		case *model.ModelPricing, *model.ChannelModelPriceTier, *model.LogicalModelPriceSKU, *model.TaskCostCharge:
			t.Fatalf("价格规则表属于托管表，不得进入 LocalModels: %T", item)
		}
	}
}

func TestMigrateHostedSchemaCreatesManjuTables(t *testing.T) {
	db := openHostedTestDB(t, "manju.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	for _, table := range []string{
		"manju_project_members", "manju_lock_receipts", "manju_project_model_bindings",
		"manju_project_states",
		"manju_rule_snapshots", "manju_rule_read_receipts", "manju_model_exchanges", "manju_artifact_ledger",
		"manju_script_runs", "manju_script_stages", "manju_script_sources", "manju_script_artifacts",
		"manju_asset_runs", "manju_asset_stages", "manju_asset_artifacts", "manju_asset_references",
		"manju_production_runs", "manju_production_stages", "manju_production_artifacts",
		"manju_deliverables",
		"manju_production_images", "manju_production_videos",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("托管迁移必须建出 %s", table)
		}
	}
	for _, item := range LocalModels() {
		switch item.(type) {
		case *model.ManjuProjectMember, *model.ManjuLockReceipt, *model.ManjuProjectModelBinding,
			*model.ManjuProjectStateSnapshot,
			*model.ManjuRuleSnapshot, *model.ManjuRuleReadReceipt,
			*model.ManjuModelExchange, *model.ManjuArtifactLedger,
			*model.ManjuScriptRun, *model.ManjuScriptStage, *model.ManjuScriptSource, *model.ManjuScriptArtifact,
			*model.ManjuAssetRun, *model.ManjuAssetStage, *model.ManjuAssetArtifact, *model.ManjuAssetReference,
			*model.ManjuProductionRun, *model.ManjuProductionStage, *model.ManjuProductionArtifact,
			*model.ManjuDeliverable,
			*model.ManjuProductionImage, *model.ManjuProductionVideo:
			t.Fatalf("漫剧成员与锁表属于托管表，不得进入 LocalModels: %T", item)
		}
	}
}

func TestMigrateHostedSchemaCreatesManjuDeliverableRunKindConstraint(t *testing.T) {
	db := openHostedTestDB(t, "manju-deliverables.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	indexes, err := db.Migrator().GetIndexes(&model.ManjuDeliverable{})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if index.Name() != "idx_manju_deliverable_run_kind" {
			continue
		}
		columns := index.Columns()
		if len(columns) == 2 && columns[0] == "production_run_id" && columns[1] == "kind" {
			return
		}
		t.Fatalf("deliverable unique index columns = %v", columns)
	}
	t.Fatal("托管迁移必须建立 manju_deliverables(production_run_id, kind) 唯一约束")
}
