package main

import (
	"errors"
	"strings"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type pricingIdentityLookup interface {
	ChannelModelByKey(channelID string, modelKey string) (*model.ChannelModel, error)
	LogicalModelsBySourceChannelModelID(channelModelID string) ([]model.LogicalModel, error)
	LogicalModelsByOnlyActiveChannelModelID(channelModelID string) ([]model.LogicalModel, error)
}

type pricingQuoteResolver struct {
	pricing *credits.PricingService
	lookup  pricingIdentityLookup
}

func newPricingQuoteResolver(pricing *credits.PricingService, lookup pricingIdentityLookup) *pricingQuoteResolver {
	return &pricingQuoteResolver{pricing: pricing, lookup: lookup}
}

func (r *pricingQuoteResolver) Quote(request credits.QuoteRequest) (*credits.Quote, error) {
	if err := enrichPricingQuoteIdentity(&request, r.lookup); err != nil {
		return nil, err
	}
	return r.pricing.Quote(request)
}

// enrichPricingQuoteIdentity supports Canvas sessions persisted before the
// public catalog began retaining channel-model IDs. The server can recover the
// executable model and its source logical model from the model key.
func enrichPricingQuoteIdentity(request *credits.QuoteRequest, lookup pricingIdentityLookup) error {
	if request == nil || lookup == nil {
		return nil
	}
	request.ModelKey = strings.TrimPrefix(strings.TrimSpace(request.ModelKey), "models/")
	if request.ChannelModelID == "" && request.ChannelID != "" && request.ModelKey != "" {
		channelModel, err := lookup.ChannelModelByKey(request.ChannelID, request.ModelKey)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		request.ChannelModelID = channelModel.ID
	}
	if request.LogicalModelID != "" || request.ChannelModelID == "" {
		return nil
	}

	logicalModels, err := lookup.LogicalModelsBySourceChannelModelID(request.ChannelModelID)
	if err != nil {
		return err
	}
	if len(logicalModels) == 0 {
		logicalModels, err = lookup.LogicalModelsByOnlyActiveChannelModelID(request.ChannelModelID)
		if err != nil {
			return err
		}
	}
	for _, logicalModel := range logicalModels {
		if !logicalModel.Enabled || !strings.EqualFold(logicalModel.Capability, request.Capability) {
			continue
		}
		request.LogicalModelID = logicalModel.ID
		break
	}
	return nil
}
