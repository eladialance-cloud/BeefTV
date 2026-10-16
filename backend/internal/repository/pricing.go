package repository

import (
	"errors"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// PricingRules returns all rules in one pricing layer, ordered for the admin
// console. The service filters by effective interval and selector later.
func (r *Repository) PricingRules(kind string) ([]model.PricingRule, error) {
	switch kind {
	case model.PricingKindModel:
		var items []model.ModelPricing
		if err := r.db.Order("sort_order asc, effective_from desc, created_at desc, id desc").Find(&items).Error; err != nil {
			return nil, err
		}
		rules := make([]model.PricingRule, 0, len(items))
		for _, item := range items {
			rules = append(rules, modelPricingRule(item))
		}
		return rules, nil
	case model.PricingKindChannelTier:
		var items []model.ChannelModelPriceTier
		if err := r.db.Order("sort_order asc, effective_from desc, created_at desc, id desc").Find(&items).Error; err != nil {
			return nil, err
		}
		rules := make([]model.PricingRule, 0, len(items))
		for _, item := range items {
			rules = append(rules, channelTierPricingRule(item))
		}
		return rules, nil
	case model.PricingKindLogicalSKU:
		var items []model.LogicalModelPriceSKU
		if err := r.db.Order("sort_order asc, effective_from desc, created_at desc, id desc").Find(&items).Error; err != nil {
			return nil, err
		}
		rules := make([]model.PricingRule, 0, len(items))
		for _, item := range items {
			rules = append(rules, logicalSKUPricingRule(item))
		}
		return rules, nil
	default:
		return nil, errors.New("未知价格规则类型")
	}
}

// PricingRule reads one rule from the requested table. found=false means the
// row does not exist; it is not an error.
func (r *Repository) PricingRule(kind string, id string) (*model.PricingRule, bool, error) {
	switch kind {
	case model.PricingKindModel:
		var item model.ModelPricing
		err := r.db.First(&item, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		rule := modelPricingRule(item)
		return &rule, true, nil
	case model.PricingKindChannelTier:
		var item model.ChannelModelPriceTier
		err := r.db.First(&item, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		rule := channelTierPricingRule(item)
		return &rule, true, nil
	case model.PricingKindLogicalSKU:
		var item model.LogicalModelPriceSKU
		err := r.db.First(&item, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		rule := logicalSKUPricingRule(item)
		return &rule, true, nil
	default:
		return nil, false, errors.New("未知价格规则类型")
	}
}

// CreatePricingRule writes one rule into its own hosted table.
func (r *Repository) CreatePricingRule(kind string, rule *model.PricingRule) error {
	if rule == nil {
		return errors.New("价格规则不能为空")
	}
	switch kind {
	case model.PricingKindModel:
		item := modelPricingFromRule(rule)
		return r.db.Create(&item).Error
	case model.PricingKindChannelTier:
		item := channelTierFromRule(rule)
		return r.db.Create(&item).Error
	case model.PricingKindLogicalSKU:
		item := logicalSKUFromRule(rule)
		return r.db.Create(&item).Error
	default:
		return errors.New("未知价格规则类型")
	}
}

// UpdatePricingRule updates mutable columns without changing the row identity
// or creation metadata.
func (r *Repository) UpdatePricingRule(kind string, rule *model.PricingRule) error {
	if rule == nil {
		return errors.New("价格规则不能为空")
	}
	switch kind {
	case model.PricingKindModel:
		item := modelPricingFromRule(rule)
		return updatePricingTable(r.db, kind, item.ID, map[string]any{
			"name": item.Name, "logical_model_id": item.LogicalModelID,
			"capability": item.Capability, "billing_mode": item.BillingMode,
			"selector_key": item.SelectorKey, "selector_json": item.SelectorJSON,
			"unit_price_microcredits": item.UnitPriceMicrocredits,
			"minimum_charge_credits":  item.MinimumChargeCredits,
			"enabled":                 item.Enabled, "sort_order": item.SortOrder,
			"effective_from": item.EffectiveFrom, "effective_to": item.EffectiveTo,
			"note": item.Note, "updated_at": item.UpdatedAt,
		})
	case model.PricingKindChannelTier:
		item := channelTierFromRule(rule)
		return updatePricingTable(r.db, kind, item.ID, map[string]any{
			"name": item.Name, "channel_id": item.ChannelID, "channel_model_id": item.ChannelModelID,
			"capability": item.Capability, "billing_mode": item.BillingMode,
			"selector_key": item.SelectorKey, "selector_json": item.SelectorJSON,
			"unit_price_microcredits": item.UnitPriceMicrocredits,
			"minimum_charge_credits":  item.MinimumChargeCredits,
			"enabled":                 item.Enabled, "sort_order": item.SortOrder,
			"effective_from": item.EffectiveFrom, "effective_to": item.EffectiveTo,
			"note": item.Note, "updated_at": item.UpdatedAt,
		})
	case model.PricingKindLogicalSKU:
		item := logicalSKUFromRule(rule)
		return updatePricingTable(r.db, kind, item.ID, map[string]any{
			"name": item.Name, "logical_model_id": item.LogicalModelID,
			"channel_model_price_tier_id": item.ChannelModelPriceTierID,
			"capability":                  item.Capability, "billing_mode": item.BillingMode,
			"selector_key": item.SelectorKey, "selector_json": item.SelectorJSON,
			"unit_price_microcredits": item.UnitPriceMicrocredits,
			"minimum_charge_credits":  item.MinimumChargeCredits,
			"enabled":                 item.Enabled, "sort_order": item.SortOrder,
			"effective_from": item.EffectiveFrom, "effective_to": item.EffectiveTo,
			"note": item.Note, "updated_at": item.UpdatedAt,
		})
	default:
		return errors.New("未知价格规则类型")
	}
}

// DeletePricingRule removes one rule from its table.
func (r *Repository) DeletePricingRule(kind string, id string) error {
	modelValue, ok := pricingModelForKind(kind)
	if !ok {
		return errors.New("未知价格规则类型")
	}
	result := r.db.Delete(modelValue, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func updatePricingTable(db *gorm.DB, kind string, id string, updates map[string]any) error {
	modelValue, ok := pricingModelForKind(kind)
	if !ok {
		return errors.New("未知价格规则类型")
	}
	result := db.Model(modelValue).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func pricingModelForKind(kind string) (any, bool) {
	switch kind {
	case model.PricingKindModel:
		return &model.ModelPricing{}, true
	case model.PricingKindChannelTier:
		return &model.ChannelModelPriceTier{}, true
	case model.PricingKindLogicalSKU:
		return &model.LogicalModelPriceSKU{}, true
	default:
		return nil, false
	}
}

func modelPricingRule(item model.ModelPricing) model.PricingRule {
	return model.PricingRule{
		ID: item.ID, Kind: model.PricingKindModel, Name: item.Name,
		LogicalModelID: item.LogicalModelID, Capability: item.Capability, BillingMode: item.BillingMode,
		Selector:              model.DecodeSKUSelector(item.SelectorJSON),
		UnitPriceMicrocredits: item.UnitPriceMicrocredits, MinimumChargeCredits: item.MinimumChargeCredits,
		Enabled: item.Enabled, SortOrder: item.SortOrder,
		EffectiveFrom: item.EffectiveFrom, EffectiveTo: item.EffectiveTo,
		Note: item.Note, CreatedBy: item.CreatedBy, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func channelTierPricingRule(item model.ChannelModelPriceTier) model.PricingRule {
	return model.PricingRule{
		ID: item.ID, Kind: model.PricingKindChannelTier, Name: item.Name,
		ChannelID: item.ChannelID, ChannelModelID: item.ChannelModelID,
		Capability: item.Capability, BillingMode: item.BillingMode,
		Selector:              model.DecodeSKUSelector(item.SelectorJSON),
		UnitPriceMicrocredits: item.UnitPriceMicrocredits, MinimumChargeCredits: item.MinimumChargeCredits,
		Enabled: item.Enabled, SortOrder: item.SortOrder,
		EffectiveFrom: item.EffectiveFrom, EffectiveTo: item.EffectiveTo,
		Note: item.Note, CreatedBy: item.CreatedBy, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func logicalSKUPricingRule(item model.LogicalModelPriceSKU) model.PricingRule {
	return model.PricingRule{
		ID: item.ID, Kind: model.PricingKindLogicalSKU, Name: item.Name,
		LogicalModelID: item.LogicalModelID, ChannelModelPriceTierID: item.ChannelModelPriceTierID,
		Capability: item.Capability, BillingMode: item.BillingMode,
		Selector:              model.DecodeSKUSelector(item.SelectorJSON),
		UnitPriceMicrocredits: item.UnitPriceMicrocredits, MinimumChargeCredits: item.MinimumChargeCredits,
		Enabled: item.Enabled, SortOrder: item.SortOrder,
		EffectiveFrom: item.EffectiveFrom, EffectiveTo: item.EffectiveTo,
		Note: item.Note, CreatedBy: item.CreatedBy, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func modelPricingFromRule(rule *model.PricingRule) model.ModelPricing {
	selectorKey, selectorJSON := pricingSelectorFields(rule.Selector)
	return model.ModelPricing{
		ID: rule.ID, Name: rule.Name, LogicalModelID: rule.LogicalModelID,
		Capability: rule.Capability, BillingMode: rule.BillingMode,
		SelectorKey: selectorKey, SelectorJSON: selectorJSON,
		UnitPriceMicrocredits: rule.UnitPriceMicrocredits, MinimumChargeCredits: rule.MinimumChargeCredits,
		Enabled: rule.Enabled, SortOrder: rule.SortOrder,
		EffectiveFrom: rule.EffectiveFrom, EffectiveTo: rule.EffectiveTo,
		Note: rule.Note, CreatedBy: rule.CreatedBy, CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}
}

func channelTierFromRule(rule *model.PricingRule) model.ChannelModelPriceTier {
	selectorKey, selectorJSON := pricingSelectorFields(rule.Selector)
	return model.ChannelModelPriceTier{
		ID: rule.ID, Name: rule.Name, ChannelID: rule.ChannelID, ChannelModelID: rule.ChannelModelID,
		Capability: rule.Capability, BillingMode: rule.BillingMode,
		SelectorKey: selectorKey, SelectorJSON: selectorJSON,
		UnitPriceMicrocredits: rule.UnitPriceMicrocredits, MinimumChargeCredits: rule.MinimumChargeCredits,
		Enabled: rule.Enabled, SortOrder: rule.SortOrder,
		EffectiveFrom: rule.EffectiveFrom, EffectiveTo: rule.EffectiveTo,
		Note: rule.Note, CreatedBy: rule.CreatedBy, CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}
}

func logicalSKUFromRule(rule *model.PricingRule) model.LogicalModelPriceSKU {
	selectorKey, selectorJSON := pricingSelectorFields(rule.Selector)
	return model.LogicalModelPriceSKU{
		ID: rule.ID, Name: rule.Name, LogicalModelID: rule.LogicalModelID,
		ChannelModelPriceTierID: rule.ChannelModelPriceTierID,
		Capability:              rule.Capability, BillingMode: rule.BillingMode,
		SelectorKey: selectorKey, SelectorJSON: selectorJSON,
		UnitPriceMicrocredits: rule.UnitPriceMicrocredits, MinimumChargeCredits: rule.MinimumChargeCredits,
		Enabled: rule.Enabled, SortOrder: rule.SortOrder,
		EffectiveFrom: rule.EffectiveFrom, EffectiveTo: rule.EffectiveTo,
		Note: rule.Note, CreatedBy: rule.CreatedBy, CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}
}

func pricingSelectorFields(selector map[string]string) (string, string) {
	normalized, key, err := model.CanonicalSKUSelector(selector)
	if err != nil {
		return "{}", "{}"
	}
	encoded := "{}"
	if len(normalized) > 0 {
		encoded = key
	}
	return key, encoded
}
