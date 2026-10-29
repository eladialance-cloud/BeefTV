package model

import "time"

// Pricing rule kinds. The rules live in three hosted tables so the admin
// console can edit each layer independently.
const (
	PricingKindModel       = "model"
	PricingKindChannelTier = "channel_tier"
	PricingKindLogicalSKU  = "logical_sku"
)

// PricingRule is the transport-independent view used by the hosted pricing
// service. The three GORM entities below persist the same shape under their
// business-specific table names.
type PricingRule struct {
	ID                      string            `json:"id"`
	Kind                    string            `json:"kind"`
	Name                    string            `json:"name,omitempty"`
	LogicalModelID          string            `json:"logicalModelId,omitempty"`
	ChannelID               string            `json:"channelId,omitempty"`
	ChannelModelID          string            `json:"channelModelId,omitempty"`
	ChannelModelPriceTierID string            `json:"channelModelPriceTierId,omitempty"`
	Capability              string            `json:"capability,omitempty"`
	BillingMode             string            `json:"billingMode,omitempty"`
	Selector                map[string]string `json:"selector,omitempty"`
	UnitPriceMicrocredits   int64             `json:"unitPriceMicrocredits"`
	// MinimumChargeCredits is a legacy field name whose persisted and API value
	// is now expressed in microcredits.
	MinimumChargeCredits int64      `json:"minimumChargeCredits"`
	Enabled              bool       `json:"enabled"`
	SortOrder            int        `json:"sortOrder"`
	EffectiveFrom        time.Time  `json:"effectiveFrom"`
	EffectiveTo          *time.Time `json:"effectiveTo,omitempty"`
	Note                 string     `json:"note,omitempty"`
	CreatedBy            string     `json:"createdBy,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

// ModelPricing is the logical-model default price. It is the last fallback
// when no public SKU or executable channel-model tier matches a quote.
type ModelPricing struct {
	ID                    string     `json:"id" gorm:"primaryKey;size:36"`
	Name                  string     `json:"name" gorm:"size:120"`
	LogicalModelID        string     `json:"logicalModelId" gorm:"size:36;index"`
	Capability            string     `json:"capability" gorm:"size:32;index"`
	BillingMode           string     `json:"billingMode" gorm:"size:32;index"`
	SelectorKey           string     `json:"-" gorm:"size:500;not null;default:{}"`
	SelectorJSON          string     `json:"-" gorm:"type:text;not null;default:{}"`
	UnitPriceMicrocredits int64      `json:"unitPriceMicrocredits"`
	MinimumChargeCredits  int64      `json:"minimumChargeCredits"`
	Enabled               bool       `json:"enabled" gorm:"index"`
	SortOrder             int        `json:"sortOrder" gorm:"not null;default:0"`
	EffectiveFrom         time.Time  `json:"effectiveFrom" gorm:"index"`
	EffectiveTo           *time.Time `json:"effectiveTo" gorm:"index"`
	Note                  string     `json:"note" gorm:"type:text"`
	CreatedBy             string     `json:"createdBy" gorm:"size:36;index"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

func (ModelPricing) TableName() string { return "model_pricings" }

// ChannelModelPriceTier prices an executable channel-model selector. It is the
// layer the task runtime uses when it already knows the concrete route.
type ChannelModelPriceTier struct {
	ID                    string     `json:"id" gorm:"primaryKey;size:36"`
	Name                  string     `json:"name" gorm:"size:120"`
	ChannelID             string     `json:"channelId" gorm:"size:36;index"`
	ChannelModelID        string     `json:"channelModelId" gorm:"size:36;index"`
	Capability            string     `json:"capability" gorm:"size:32;index"`
	BillingMode           string     `json:"billingMode" gorm:"size:32;index"`
	SelectorKey           string     `json:"-" gorm:"size:500;not null;default:{}"`
	SelectorJSON          string     `json:"-" gorm:"type:text;not null;default:{}"`
	UnitPriceMicrocredits int64      `json:"unitPriceMicrocredits"`
	MinimumChargeCredits  int64      `json:"minimumChargeCredits"`
	Enabled               bool       `json:"enabled" gorm:"index"`
	SortOrder             int        `json:"sortOrder" gorm:"not null;default:0"`
	EffectiveFrom         time.Time  `json:"effectiveFrom" gorm:"index"`
	EffectiveTo           *time.Time `json:"effectiveTo" gorm:"index"`
	Note                  string     `json:"note" gorm:"type:text"`
	CreatedBy             string     `json:"createdBy" gorm:"size:36;index"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

func (ChannelModelPriceTier) TableName() string { return "channel_model_price_tiers" }

// LogicalModelPriceSKU is a user-facing purchasable option under a logical
// model. A logical model can have multiple SKUs with different selectors and
// prices; the optional channel tier records the executable route it prices.
type LogicalModelPriceSKU struct {
	ID                      string     `json:"id" gorm:"primaryKey;size:36"`
	Name                    string     `json:"name" gorm:"size:120"`
	LogicalModelID          string     `json:"logicalModelId" gorm:"size:36;index"`
	ChannelModelPriceTierID string     `json:"channelModelPriceTierId" gorm:"size:36;index"`
	Capability              string     `json:"capability" gorm:"size:32;index"`
	BillingMode             string     `json:"billingMode" gorm:"size:32;index"`
	SelectorKey             string     `json:"-" gorm:"size:500;not null;default:{}"`
	SelectorJSON            string     `json:"-" gorm:"type:text;not null;default:{}"`
	UnitPriceMicrocredits   int64      `json:"unitPriceMicrocredits"`
	MinimumChargeCredits    int64      `json:"minimumChargeCredits"`
	Enabled                 bool       `json:"enabled" gorm:"index"`
	SortOrder               int        `json:"sortOrder" gorm:"not null;default:0"`
	EffectiveFrom           time.Time  `json:"effectiveFrom" gorm:"index"`
	EffectiveTo             *time.Time `json:"effectiveTo" gorm:"index"`
	Note                    string     `json:"note" gorm:"type:text"`
	CreatedBy               string     `json:"createdBy" gorm:"size:36;index"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

func (LogicalModelPriceSKU) TableName() string { return "logical_model_price_skus" }
