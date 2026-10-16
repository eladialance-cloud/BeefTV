package database

import (
	"fmt"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// MigrateHostedSchema 是 server/hosted 形态的结构入口。它只做 GORM AutoMigrate，
// 不执行本地形态的 SQLite 结构修复，也绝不删除托管表。
func MigrateHostedSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(HostedModels()...); err != nil {
		return fmt.Errorf("托管结构迁移失败: %w", err)
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
	)
}

// RequireHostedSchema 在关闭自动迁移时做最小可用性校验：核心表必须存在。
func RequireHostedSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Project{}) {
		return fmt.Errorf("托管数据库缺少核心表 projects，请先执行迁移")
	}
	return nil
}
