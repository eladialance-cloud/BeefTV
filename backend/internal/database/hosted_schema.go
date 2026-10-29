package database

import (
	"fmt"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// MigrateHostedSchema 是 server/hosted 形态的结构入口。它只做 GORM AutoMigrate，
// 不执行本地形态的 SQLite 结构修复，也绝不删除托管表。
func MigrateHostedSchema(db *gorm.DB) error {
	if err := normalizePaymentNotificationIndex(db); err != nil {
		return err
	}
	if err := normalizeAgentOpRecordOpID(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(HostedModels()...); err != nil {
		return fmt.Errorf("托管结构迁移失败: %w", err)
	}
	if err := migrateHostedCreditsToMicrocredits(db); err != nil {
		return err
	}
	// 托管启动路径仍会解析默认工作区（bootstrap.Open 需要它构建 WorkspaceID），
	// 所以新库必须和本地迁移一样先有一行工作区，否则服务端起不来。
	if err := ensureLocalWorkspace(db); err != nil {
		return err
	}
	return nil
}

// normalizeAgentOpRecordOpID upgrades the early varchar(80) primary-key
// column. Canvas effect IDs include the task and node identity and can exceed
// 80 characters; PostgreSQL does not widen an existing column during
// AutoMigrate when only the model tag changes.
func normalizeAgentOpRecordOpID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.AgentOpRecord{}) {
		return nil
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	if err := db.Exec("ALTER TABLE agent_op_records ALTER COLUMN op_id TYPE varchar(255)").Error; err != nil {
		return fmt.Errorf("升级操作记录 ID 长度失败: %w", err)
	}
	return nil
}

// normalizePaymentNotificationIndex upgrades the early single-column unique
// index to the intended (provider, provider_notification_id) composite key.
// AutoMigrate does not rewrite an existing index with the same name, so this
// narrow preflight keeps upgraded databases aligned with fresh installs.
func normalizePaymentNotificationIndex(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.PaymentNotification{}) {
		return nil
	}
	indexes, err := db.Migrator().GetIndexes(&model.PaymentNotification{})
	if err != nil {
		return fmt.Errorf("读取支付回调索引失败: %w", err)
	}
	for _, index := range indexes {
		if index.Name() != "idx_payment_notification_provider_id" {
			continue
		}
		columns := index.Columns()
		if len(columns) == 2 && ((columns[0] == "provider" && columns[1] == "provider_notification_id") ||
			(columns[0] == "provider_notification_id" && columns[1] == "provider")) {
			return nil
		}
		if err := db.Migrator().DropIndex(&model.PaymentNotification{}, index.Name()); err != nil {
			return fmt.Errorf("升级支付回调索引失败: %w", err)
		}
		return nil
	}
	return nil
}

// HostedModels 是 server/hosted 形态的结构：本地业务模型加托管侧控制面模型。
// 托管表绝不能进入 LocalModels，否则 desktop 迁移会先建后删。
func HostedModels() []any {
	models := LocalModels()
	return append(models,
		// 账号与会话（托管专属，桌面形态不建也不碰）。
		&model.User{}, &model.UserIdentity{}, &model.AuthSession{},
		// 邀请码与核销记录（托管专属）。
		&model.InviteCode{}, &model.InviteCodeRedemption{},
		// 管理员写操作审计（托管专属：桌面形态把它当退场表，绝不建）。
		&model.AdminAuditEvent{},
		// 管理端内容治理（托管专属）。
		&model.AdminContentModeration{},
		// 平台级对象存储配置。
		&model.StorageLocation{},
		// 积分账户与不可变流水（托管专属）。
		&model.CreditAccount{}, &model.CreditLedgerEntry{},
		// 模型价格规则（托管专属，运行时按生效区间解析）。
		&model.ModelPricing{}, &model.ChannelModelPriceTier{}, &model.LogicalModelPriceSKU{},
		// 运营配置的创作模板目录（工作流、提示词、画风，托管专属）。
		&model.CreationCatalogItem{},
		// 任务报价快照与冻结额（托管专属，按任务尝试记账）。
		&model.TaskCostCharge{},
		// 支付渠道配置（托管专属，密钥加密存储）。
		&model.PaymentProviderConfig{},
		// 充值商品、支付订单与回调记录（托管专属）。
		&model.TopupProduct{}, &model.PaymentOrder{}, &model.PaymentNotification{},
		// 漫剧项目协作与不可变 P0 决策回执（托管专属）。
		&model.ManjuProjectMember{}, &model.ManjuLockReceipt{}, &model.ManjuProjectModelBinding{},
		// 漫剧 V7.4.26 跨阶段 project_state.json 不可变快照（托管专属）。
		&model.ManjuProjectStateSnapshot{},
		// 规则版本快照、逐阶段读取回执、完整模型交换与统一工件台账。
		&model.ManjuRuleSnapshot{}, &model.ManjuRuleReadReceipt{},
		&model.ManjuModelExchange{}, &model.ManjuArtifactLedger{},
		// Project-level novel source library and immutable chapter selection rows.
		&model.ManjuSourceDocument{}, &model.ManjuSourceChapter{},
		// 漫剧 P1 剧本运行、阶段、来源账本与不可变产物（托管专属）。
		&model.ManjuScriptRun{}, &model.ManjuScriptStage{}, &model.ManjuScriptSource{}, &model.ManjuScriptArtifact{},
		&model.ManjuRoundtableDecision{},
		// 漫剧 P2 数字资产运行、阶段、不可变交付物与 M2 参考图登记（托管专属）。
		&model.ManjuAssetRun{}, &model.ManjuAssetStage{}, &model.ManjuAssetArtifact{}, &model.ManjuAssetReference{},
		// 漫剧 P3-P5 分镜、投喂与质检运行、阶段与不可变交付物（托管专属）。
		&model.ManjuProductionRun{}, &model.ManjuProductionStage{}, &model.ManjuProductionArtifact{},
		&model.ManjuDeliverable{},
		&model.ManjuProductionPrevis{}, &model.ManjuProductionImage{},
		&model.ManjuProductionVideo{},
		&model.ManjuStageDraft{}, &model.ManjuArtifactCorrection{},
		&model.ManjuCanvasImport{},
	)
}

// hostedRequiredTables 是托管服务端起服务前必须存在的表：本地业务表加托管控制面表。
// 只检查 projects 会放过「托管表根本没建」的库（server 形态曾在 SQLite 上踩过），
// 所以账号、积分、支付、存储与本地核心表都要一起检查。
func hostedRequiredTables() []string {
	return []string{
		"workspaces", "projects", "canvas_projects", "tasks",
		"users", "user_identities", "auth_sessions",
		"storage_locations", "credit_accounts", "credit_ledger_entries",
		"model_pricings", "channel_model_price_tiers", "logical_model_price_skus",
		"creation_catalog_items",
		"invite_codes", "invite_code_redemptions",
		"topup_products", "payment_provider_configs", "payment_orders", "payment_notifications",
		"task_cost_charges", "admin_audit_events",
		"admin_content_moderations",
		"manju_project_members", "manju_lock_receipts", "manju_project_model_bindings",
		"manju_project_states",
		"manju_rule_snapshots", "manju_rule_read_receipts",
		"manju_model_exchanges", "manju_artifact_ledger",
		"manju_source_documents", "manju_source_chapters",
		"manju_script_runs", "manju_script_stages", "manju_script_sources", "manju_script_artifacts",
		"manju_roundtable_decisions",
		"manju_asset_runs", "manju_asset_stages", "manju_asset_artifacts", "manju_asset_references",
		"manju_production_runs", "manju_production_stages", "manju_production_artifacts",
		"manju_deliverables",
		"manju_production_previs", "manju_production_images",
		"manju_production_videos",
		"manju_stage_drafts", "manju_artifact_corrections",
		"manju_canvas_imports",
	}
}

// RequireHostedSchema 在关闭自动迁移时做最小可用性校验：托管表必须存在。
func RequireHostedSchema(db *gorm.DB) error {
	for _, table := range hostedRequiredTables() {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("托管数据库缺少表 %s，请先执行迁移", table)
		}
	}
	return nil
}
