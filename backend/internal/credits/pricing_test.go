package credits

import (
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

type memoryPricingRepository struct {
	rules map[string][]model.PricingRule
}

func newMemoryPricingRepository() *memoryPricingRepository {
	return &memoryPricingRepository{rules: map[string][]model.PricingRule{}}
}

func (r *memoryPricingRepository) PricingRules(kind string) ([]model.PricingRule, error) {
	return append([]model.PricingRule(nil), r.rules[kind]...), nil
}

func (r *memoryPricingRepository) PricingRule(kind string, id string) (*model.PricingRule, bool, error) {
	for index := range r.rules[kind] {
		if r.rules[kind][index].ID == id {
			item := r.rules[kind][index]
			return &item, true, nil
		}
	}
	return nil, false, nil
}

func (r *memoryPricingRepository) CreatePricingRule(kind string, rule *model.PricingRule) error {
	r.rules[kind] = append(r.rules[kind], *rule)
	return nil
}

func (r *memoryPricingRepository) UpdatePricingRule(kind string, rule *model.PricingRule) error {
	for index := range r.rules[kind] {
		if r.rules[kind][index].ID != rule.ID {
			continue
		}
		updated := *rule
		r.rules[kind][index] = updated
		return nil
	}
	return errors.New("pricing rule not found")
}

func (r *memoryPricingRepository) DeletePricingRule(kind string, id string) error {
	items := r.rules[kind]
	for index := range items {
		if items[index].ID != id {
			continue
		}
		r.rules[kind] = append(items[:index], items[index+1:]...)
		return nil
	}
	return errors.New("pricing rule not found")
}

func (r *memoryPricingRepository) AppendAdminAudit(*model.AdminAuditEvent) error { return nil }

func pricingTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func pricingTimePtr(value time.Time) *time.Time { return &value }

func TestPricingQuoteUsesLatestRuleByEffectiveInterval(t *testing.T) {
	repo := newMemoryPricingRepository()
	start := pricingTime("2026-10-01T00:00:00Z")
	repo.rules[model.PricingKindModel] = []model.PricingRule{
		{
			ID: "price-old", Kind: model.PricingKindModel, LogicalModelID: "lm-video",
			Capability: "video", BillingMode: "per_second", UnitPriceMicrocredits: 100000,
			Enabled: true, EffectiveFrom: start, EffectiveTo: pricingTimePtr(start.Add(24 * time.Hour)),
		},
		{
			ID: "price-new", Kind: model.PricingKindModel, LogicalModelID: "lm-video",
			Capability: "video", BillingMode: "per_second", UnitPriceMicrocredits: 200000,
			Enabled: true, EffectiveFrom: start.Add(24 * time.Hour),
		},
	}
	service := NewPricing(repo, Config{})

	oldQuote, err := service.Quote(QuoteRequest{
		LogicalModelID: "lm-video", Capability: "video", BillingMode: "per_second",
		Quantity: 10, At: start.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if oldQuote.RuleID != "price-old" || oldQuote.EstimatedCredits != 1 {
		t.Fatalf("旧生效区间必须按旧价，得到 %#v", oldQuote)
	}

	newQuote, err := service.Quote(QuoteRequest{
		LogicalModelID: "lm-video", Capability: "video", BillingMode: "per_second",
		Quantity: 10, At: start.Add(25 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if newQuote.RuleID != "price-new" || newQuote.EstimatedCredits != 2 {
		t.Fatalf("新生效区间必须按新价，得到 %#v", newQuote)
	}
}

func TestPricingQuotePrefersLogicalModelSKUForRequestedSelector(t *testing.T) {
	repo := newMemoryPricingRepository()
	start := pricingTime("2026-10-01T00:00:00Z")
	repo.rules[model.PricingKindLogicalSKU] = []model.PricingRule{
		{
			ID: "sku-720", Kind: model.PricingKindLogicalSKU, LogicalModelID: "lm-video",
			Capability: "video", BillingMode: "per_second", Selector: map[string]string{"quality": "720p"},
			UnitPriceMicrocredits: 120000, MinimumChargeCredits: 2,
			Enabled: true, EffectiveFrom: start,
		},
		{
			ID: "sku-1080", Kind: model.PricingKindLogicalSKU, LogicalModelID: "lm-video",
			Capability: "video", BillingMode: "per_second", Selector: map[string]string{"quality": "1080p"},
			UnitPriceMicrocredits: 220000, MinimumChargeCredits: 3,
			Enabled: true, EffectiveFrom: start,
		},
	}
	service := NewPricing(repo, Config{})

	quote, err := service.Quote(QuoteRequest{
		LogicalModelID: "lm-video", Capability: "video", BillingMode: "per_second",
		Selector: map[string]string{"quality": "1080p"}, Quantity: 10, At: start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.RuleID != "sku-1080" || quote.RuleKind != model.PricingKindLogicalSKU || quote.EstimatedCredits != 3 {
		t.Fatalf("同一逻辑模型必须能选到不同 SKU，得到 %#v", quote)
	}
}

func TestPricingQuoteFallsBackToChannelTierThenModelPricing(t *testing.T) {
	repo := newMemoryPricingRepository()
	start := pricingTime("2026-10-01T00:00:00Z")
	repo.rules[model.PricingKindChannelTier] = []model.PricingRule{
		{
			ID: "tier-1", Kind: model.PricingKindChannelTier, ChannelID: "channel-1", ChannelModelID: "cm-1",
			Capability: "image", BillingMode: "fixed_request", UnitPriceMicrocredits: 300000,
			Enabled: true, EffectiveFrom: start,
		},
	}
	repo.rules[model.PricingKindModel] = []model.PricingRule{
		{
			ID: "model-default", Kind: model.PricingKindModel, LogicalModelID: "lm-image",
			Capability: "image", BillingMode: "fixed_request", UnitPriceMicrocredits: 500000,
			Enabled: true, EffectiveFrom: start,
		},
	}
	service := NewPricing(repo, Config{})

	channelQuote, err := service.Quote(QuoteRequest{
		LogicalModelID: "lm-image", ChannelID: "channel-1", ChannelModelID: "cm-1",
		Capability: "image", BillingMode: "fixed_request", Quantity: 1, At: start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if channelQuote.RuleID != "tier-1" || channelQuote.RuleKind != model.PricingKindChannelTier {
		t.Fatalf("有渠道档时必须优先渠道档，得到 %#v", channelQuote)
	}

	modelQuote, err := service.Quote(QuoteRequest{
		LogicalModelID: "lm-image", Capability: "image", BillingMode: "fixed_request",
		Quantity: 1, At: start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if modelQuote.RuleID != "model-default" || modelQuote.RuleKind != model.PricingKindModel {
		t.Fatalf("没有渠道档时必须回落模型默认价，得到 %#v", modelQuote)
	}
}

func TestPricingQuoteRoundsUpAndHonorsMinimumCharge(t *testing.T) {
	repo := newMemoryPricingRepository()
	start := pricingTime("2026-10-01T00:00:00Z")
	repo.rules[model.PricingKindModel] = []model.PricingRule{
		{
			ID: "rounded", Kind: model.PricingKindModel, LogicalModelID: "lm-text",
			Capability: "text", BillingMode: "per_1k_tokens", UnitPriceMicrocredits: 125000,
			MinimumChargeCredits: 2, Enabled: true, EffectiveFrom: start,
		},
	}
	service := NewPricing(repo, Config{})

	quote, err := service.Quote(QuoteRequest{
		LogicalModelID: "lm-text", Capability: "text", BillingMode: "per_1k_tokens",
		Quantity: 1, At: start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.EstimatedCredits != 2 {
		t.Fatalf("不足最低价时按最低价计费，得到 %#v", quote)
	}
}

func TestPricingSaveRequiresAdminAndValidatesKindFields(t *testing.T) {
	repo := newMemoryPricingRepository()
	service := NewPricing(repo, Config{Now: func() time.Time { return pricingTime("2026-10-01T00:00:00Z") }})

	if _, err := service.Save(&model.User{ID: "u-1", Role: model.UserRoleUser}, model.PricingKindModel, "", PricingInput{}); err == nil {
		t.Fatal("普通用户保存价格规则必须被拒绝")
	}

	admin := &model.User{ID: "admin-1", Role: model.UserRoleAdmin}
	if _, err := service.Save(admin, model.PricingKindModel, "", PricingInput{}); err == nil {
		t.Fatal("缺少逻辑模型、能力或单价时必须拒绝")
	}

	created, err := service.Save(admin, model.PricingKindLogicalSKU, "", PricingInput{
		Name: "720p", LogicalModelID: "lm-video", Capability: "video",
		BillingMode: "per_second", Selector: map[string]string{"quality": "720p"},
		UnitPriceMicrocredits: 100000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Kind != model.PricingKindLogicalSKU || !created.Enabled {
		t.Fatalf("默认创建为启用状态，得到 %#v", created)
	}

	if _, err := service.Save(admin, model.PricingKindLogicalSKU, "", PricingInput{
		LogicalModelID: "lm-video", Capability: "video", BillingMode: "per_second",
		UnitPriceMicrocredits: 100000,
	}); err == nil {
		t.Fatal("逻辑模型 SKU 必须写选择器，否则无法区分不同价格档")
	}
}

func TestPricingQuoteRejectsUnknownRule(t *testing.T) {
	service := NewPricing(newMemoryPricingRepository(), Config{})
	_, err := service.Quote(QuoteRequest{
		LogicalModelID: "missing", Capability: "video", BillingMode: "per_second",
		Quantity: 1, At: pricingTime("2026-10-01T00:00:00Z"),
	})
	var appErr *kernel.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Status != kernel.CodeNotFound {
		t.Fatalf("没有有效价格规则时必须返回 404 类错误，得到 %v", err)
	}
}

func TestPricingQuotePrefersSmallerSortOrderAtSameEffectiveTime(t *testing.T) {
	repo := newMemoryPricingRepository()
	start := pricingTime("2026-10-01T00:00:00Z")
	repo.rules[model.PricingKindLogicalSKU] = []model.PricingRule{
		{
			ID: "sku-slow", Kind: model.PricingKindLogicalSKU, LogicalModelID: "lm-video",
			Capability: "video", BillingMode: "per_second", Selector: map[string]string{"quality": "720p"},
			UnitPriceMicrocredits: 200000, SortOrder: 20, Enabled: true, EffectiveFrom: start,
		},
		{
			ID: "sku-first", Kind: model.PricingKindLogicalSKU, LogicalModelID: "lm-video",
			Capability: "video", BillingMode: "per_second", Selector: map[string]string{"quality": "720p"},
			UnitPriceMicrocredits: 100000, SortOrder: 10, Enabled: true, EffectiveFrom: start,
		},
	}
	service := NewPricing(repo, Config{})

	quote, err := service.Quote(QuoteRequest{
		LogicalModelID: "lm-video", Capability: "video", BillingMode: "per_second",
		Selector: map[string]string{"quality": "720p"}, Quantity: 10, At: start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.RuleID != "sku-first" || quote.EstimatedCredits != 1 {
		t.Fatalf("同生效时间必须按 sortOrder 升序决胜，得到 %#v", quote)
	}
}

func TestPricingSaveCanResetMinimumChargeToZero(t *testing.T) {
	repo := newMemoryPricingRepository()
	service := NewPricing(repo, Config{Now: func() time.Time { return pricingTime("2026-10-01T00:00:00Z") }})
	admin := &model.User{ID: "admin-1", Role: model.UserRoleAdmin}
	minimum := int64(5)
	created, err := service.Save(admin, model.PricingKindModel, "", PricingInput{
		Name: "默认价", LogicalModelID: "lm-video", Capability: "video",
		BillingMode: "per_second", UnitPriceMicrocredits: 100000,
		MinimumChargeCredits: &minimum,
	})
	if err != nil {
		t.Fatal(err)
	}

	zero := int64(0)
	updated, err := service.Save(admin, model.PricingKindModel, created.ID, PricingInput{
		MinimumChargeCredits: &zero,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.MinimumChargeCredits != 0 {
		t.Fatalf("最低扣费必须可以显式改回 0，得到 %#v", updated)
	}
}
