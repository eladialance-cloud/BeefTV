package repository

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newPricingRepo(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:pricing-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModelPricing{}, &model.ChannelModelPriceTier{}, &model.LogicalModelPriceSKU{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return New(db), db
}

func TestPricingRulesRoundTripAllKinds(t *testing.T) {
	repo, _ := newPricingRepo(t)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rules := []model.PricingRule{
		{
			ID: "model-1", Kind: model.PricingKindModel, Name: "默认价",
			LogicalModelID: "lm-video", Capability: "video", BillingMode: "per_second",
			Selector: map[string]string{"quality": "720p"}, UnitPriceMicrocredits: 100000,
			Enabled: true, EffectiveFrom: start,
		},
		{
			ID: "tier-1", Kind: model.PricingKindChannelTier, Name: "渠道档",
			ChannelID: "channel-1", ChannelModelID: "cm-1", Capability: "video", BillingMode: "per_second",
			Selector: map[string]string{"quality": "720p"}, UnitPriceMicrocredits: 80000,
			Enabled: true, EffectiveFrom: start,
		},
		{
			ID: "sku-720", Kind: model.PricingKindLogicalSKU, Name: "720p",
			LogicalModelID: "lm-video", ChannelModelPriceTierID: "tier-1",
			Capability: "video", BillingMode: "per_second", Selector: map[string]string{"quality": "720p"},
			UnitPriceMicrocredits: 120000, MinimumChargeCredits: 2,
			Enabled: true, EffectiveFrom: start,
		},
	}
	for index := range rules {
		if err := repo.CreatePricingRule(rules[index].Kind, &rules[index]); err != nil {
			t.Fatalf("创建 %s 价格规则失败: %v", rules[index].Kind, err)
		}
	}

	items, err := repo.PricingRules(model.PricingKindLogicalSKU)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "sku-720" {
		t.Fatalf("按类型读取失败: %#v", items)
	}
	if items[0].Selector["quality"] != "720p" || items[0].ChannelModelPriceTierID != "tier-1" {
		t.Fatalf("选择器或渠道档必须完整回读: %#v", items[0])
	}

	items[0].UnitPriceMicrocredits = 150000
	if err := repo.UpdatePricingRule(model.PricingKindLogicalSKU, &items[0]); err != nil {
		t.Fatal(err)
	}
	updated, found, err := repo.PricingRule(model.PricingKindLogicalSKU, "sku-720")
	if err != nil || !found {
		t.Fatalf("更新后读取失败: found=%v err=%v", found, err)
	}
	if updated.UnitPriceMicrocredits != 150000 {
		t.Fatalf("更新单价未落库: %#v", updated)
	}

	if err := repo.DeletePricingRule(model.PricingKindLogicalSKU, "sku-720"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repo.PricingRule(model.PricingKindLogicalSKU, "sku-720"); err != nil || found {
		t.Fatalf("删除后不应再读到规则: found=%v err=%v", found, err)
	}
}

func TestPricingRulesKeepKindsIsolated(t *testing.T) {
	repo, _ := newPricingRepo(t)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	modelRule := model.PricingRule{
		ID: "model-1", Kind: model.PricingKindModel, LogicalModelID: "lm-1",
		Capability: "image", BillingMode: "fixed_request", UnitPriceMicrocredits: 100000,
		Enabled: true, EffectiveFrom: start,
	}
	tierRule := model.PricingRule{
		ID: "tier-1", Kind: model.PricingKindChannelTier, ChannelID: "c-1", ChannelModelID: "cm-1",
		Capability: "image", BillingMode: "fixed_request", UnitPriceMicrocredits: 80000,
		Enabled: true, EffectiveFrom: start,
	}
	if err := repo.CreatePricingRule(model.PricingKindModel, &modelRule); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreatePricingRule(model.PricingKindChannelTier, &tierRule); err != nil {
		t.Fatal(err)
	}
	modelItems, err := repo.PricingRules(model.PricingKindModel)
	if err != nil {
		t.Fatal(err)
	}
	if len(modelItems) != 1 || modelItems[0].ID != "model-1" {
		t.Fatalf("模型默认价查询串到了其他表: %#v", modelItems)
	}
	tierItems, err := repo.PricingRules(model.PricingKindChannelTier)
	if err != nil {
		t.Fatal(err)
	}
	if len(tierItems) != 1 || tierItems[0].ID != "tier-1" {
		t.Fatalf("渠道价格档查询串到了其他表: %#v", tierItems)
	}
}

func TestPricingRuleMissingReturnsFoundFalse(t *testing.T) {
	repo, _ := newPricingRepo(t)
	item, found, err := repo.PricingRule(model.PricingKindModel, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if found || item != nil {
		t.Fatalf("不存在的价格规则必须返回 found=false, item=%#v", item)
	}
}
