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
	if err := db.AutoMigrate(HostedModels()...); err != nil {
		return fmt.Errorf("托管结构迁移失败: %w", err)
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
		// 平台级对象存储配置。
		&model.StorageLocation{},
		// 积分账户与不可变流水（托管专属）。
		&model.CreditAccount{}, &model.CreditLedgerEntry{},
		// 模型价格规则（托管专属，运行时按生效区间解析）。
		&model.ModelPricing{}, &model.ChannelModelPriceTier{}, &model.LogicalModelPriceSKU{},
		// 支付渠道配置（托管专属，密钥加密存储）。
		&model.PaymentProviderConfig{},
		// 充值商品、支付订单与回调记录（托管专属）。
		&model.TopupProduct{}, &model.PaymentOrder{}, &model.PaymentNotification{},
	)
}

// RequireHostedSchema 在关闭自动迁移时做最小可用性校验：核心表必须存在。
func RequireHostedSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Project{}) {
		return fmt.Errorf("托管数据库缺少核心表 projects，请先执行迁移")
	}
	return nil
}
