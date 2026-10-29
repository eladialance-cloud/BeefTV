package app

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/modelcatalog"
)

func TestApplyPublicChannelPricesAddsEffectiveChannelTierPrices(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	channel := modelcatalog.PublicModelChannel{
		ID: "channel-1",
		ModelProfiles: []modelcatalog.PublicChannelModelProfile{
			{ID: "model-1", Model: "image-model", Capability: "image"},
		},
	}
	rules := []model.PricingRule{
		{
			ID: "price-default", Kind: model.PricingKindChannelTier,
			ChannelID: "channel-1", ChannelModelID: "model-1", Capability: "image",
			BillingMode: "per_image", UnitPriceMicrocredits: 250_000,
			MinimumChargeCredits: 500_000, Enabled: true, EffectiveFrom: now.Add(-time.Hour),
		},
		{
			ID: "price-2k", Kind: model.PricingKindChannelTier,
			ChannelID: "channel-1", ChannelModelID: "model-1", Capability: "image",
			BillingMode: "per_image", Selector: map[string]string{"quality": "2k"},
			UnitPriceMicrocredits: 1_000_000, MinimumChargeCredits: 1_000_000,
			Enabled: true, EffectiveFrom: now.Add(-time.Hour),
		},
		{
			ID: "price-future", Kind: model.PricingKindChannelTier,
			ChannelID: "channel-1", ChannelModelID: "model-1", Capability: "image",
			BillingMode: "per_image", UnitPriceMicrocredits: 9_000_000,
			Enabled: true, EffectiveFrom: now.Add(time.Hour),
		},
	}

	applyPublicChannelPrices(&channel, rules, now)

	prices := channel.ModelProfiles[0].Prices
	if len(prices) != 2 {
		t.Fatalf("effective prices = %#v", prices)
	}
	if prices[0].UnitPriceMicrocredits != 250_000 || prices[0].MinimumChargeMicrocredits != 500_000 {
		t.Fatalf("default price = %#v", prices[0])
	}
	if prices[1].Selector["quality"] != "2k" || prices[1].UnitPriceMicrocredits != 1_000_000 {
		t.Fatalf("selector price = %#v", prices[1])
	}
}
