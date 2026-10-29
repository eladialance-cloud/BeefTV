package database

import (
	"fmt"
	"math"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const hostedCreditMicrocreditsMigrationVersion int64 = 1

type hostedSchemaMigration struct {
	Version   int64     `gorm:"primaryKey"`
	Name      string    `gorm:"size:120;not null"`
	AppliedAt time.Time `gorm:"not null"`
}

func (hostedSchemaMigration) TableName() string { return "hosted_schema_migrations" }

// migrateHostedCreditsToMicrocredits upgrades every existing whole-credit
// balance and quote to the fixed-point unit. The migration marker is inserted
// in the same transaction before the updates, so concurrent restarts or a
// retried startup cannot multiply balances twice.
func migrateHostedCreditsToMicrocredits(db *gorm.DB) error {
	if err := db.AutoMigrate(&hostedSchemaMigration{}); err != nil {
		return fmt.Errorf("创建托管结构迁移表失败: %w", err)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		marker := &hostedSchemaMigration{
			Version:   hostedCreditMicrocreditsMigrationVersion,
			Name:      "credits-microcredits-v1",
			AppliedAt: time.Now().UTC(),
		}
		claimed := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(marker)
		if claimed.Error != nil {
			return fmt.Errorf("登记微积分迁移失败: %w", claimed.Error)
		}
		if claimed.RowsAffected == 0 {
			return nil
		}
		scale := model.MicrocreditsPerCredit
		scaled := func(column string) string {
			return fmt.Sprintf(
				"CASE WHEN %[1]s > %[2]d THEN %[3]d WHEN %[1]s < %[4]d THEN %[5]d ELSE %[1]s * %[6]d END",
				column, math.MaxInt64/scale, int64(math.MaxInt64), math.MinInt64/scale, int64(math.MinInt64), scale,
			)
		}
		statements := []struct {
			name string
			sql  string
		}{
			{
				name: "credit_accounts",
				sql:  "UPDATE credit_accounts SET balance = " + scaled("balance") + ", frozen = " + scaled("frozen"),
			},
			{
				name: "credit_ledger_entries",
				sql: "UPDATE credit_ledger_entries SET amount = " + scaled("amount") +
					", balance_before = " + scaled("balance_before") +
					", balance_after = " + scaled("balance_after") +
					", frozen_before = " + scaled("frozen_before") +
					", frozen_after = " + scaled("frozen_after"),
			},
			{
				name: "model_pricings",
				sql:  "UPDATE model_pricings SET minimum_charge_credits = " + scaled("minimum_charge_credits"),
			},
			{
				name: "channel_model_price_tiers",
				sql:  "UPDATE channel_model_price_tiers SET minimum_charge_credits = " + scaled("minimum_charge_credits"),
			},
			{
				name: "logical_model_price_skus",
				sql:  "UPDATE logical_model_price_skus SET minimum_charge_credits = " + scaled("minimum_charge_credits"),
			},
			{
				name: "task_cost_charges",
				sql: "UPDATE task_cost_charges SET minimum_charge_credits = " + scaled("minimum_charge_credits") +
					", estimated_credits = " + scaled("estimated_credits") +
					", actual_credits = " + scaled("actual_credits"),
			},
		}
		for _, statement := range statements {
			if err := tx.Exec(statement.sql).Error; err != nil {
				return fmt.Errorf("迁移 %s 到微积分失败: %w", statement.name, err)
			}
		}
		return nil
	})
}
