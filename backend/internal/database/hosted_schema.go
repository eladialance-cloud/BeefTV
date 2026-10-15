package database

import (
	"fmt"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// MigrateHostedSchema 是 server/hosted 形态的结构入口。它只做 GORM AutoMigrate，
// 不执行本地形态的 SQLite 结构修复，也绝不删除托管表。
func MigrateHostedSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(LocalModels()...); err != nil {
		return fmt.Errorf("托管结构迁移失败: %w", err)
	}
	return nil
}

// RequireHostedSchema 在关闭自动迁移时做最小可用性校验：核心表必须存在。
func RequireHostedSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Project{}) {
		return fmt.Errorf("托管数据库缺少核心表 projects，请先执行迁移")
	}
	return nil
}
