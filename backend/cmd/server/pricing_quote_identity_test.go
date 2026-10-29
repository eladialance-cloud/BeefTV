package main

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type fakePricingIdentityLookup struct {
	channelModel *model.ChannelModel
	logical      []model.LogicalModel
}

func (f fakePricingIdentityLookup) ChannelModelByKey(string, string) (*model.ChannelModel, error) {
	if f.channelModel == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.channelModel, nil
}

func (f fakePricingIdentityLookup) LogicalModelsBySourceChannelModelID(string) ([]model.LogicalModel, error) {
	return f.logical, nil
}

func (f fakePricingIdentityLookup) LogicalModelsByOnlyActiveChannelModelID(string) ([]model.LogicalModel, error) {
	return nil, nil
}

func TestPricingQuoteIdentityResolvesLegacyCanvasModelSelection(t *testing.T) {
	lookup := fakePricingIdentityLookup{
		channelModel: &model.ChannelModel{
			ID: "channel-model-1", ChannelID: "channel-1", ModelKey: "MiniMax-H3", Capability: "video",
		},
		logical: []model.LogicalModel{{
			ID: "logical-video", Capability: "video", Enabled: true,
		}},
	}
	request := credits.QuoteRequest{
		ChannelID:  "channel-1",
		ModelKey:   "MiniMax-H3",
		Capability: "video",
	}
	if err := enrichPricingQuoteIdentity(&request, lookup); err != nil {
		t.Fatal(err)
	}
	if request.ChannelModelID != "channel-model-1" || request.LogicalModelID != "logical-video" {
		t.Fatalf("identity = %#v", request)
	}
}

func TestPricingQuoteIdentityIgnoresMissingLegacyModelLookup(t *testing.T) {
	lookup := fakePricingIdentityLookup{}
	request := credits.QuoteRequest{ChannelID: "channel-1", ModelKey: "missing", Capability: "video"}
	if err := enrichPricingQuoteIdentity(&request, lookup); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatal(err)
		}
	}
	if request.ChannelModelID != "" || request.LogicalModelID != "" {
		t.Fatalf("identity = %#v", request)
	}
}
