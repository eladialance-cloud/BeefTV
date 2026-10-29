package main

import (
	"fmt"
	"testing"
	"time"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"
)

func TestTaskCostEstimatesCanvasTextTokensWithoutExplicitHint(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	fundTestWallet(t, wallet, "u1", 100)
	seedTokenPriceRule(t, repo, "rule-text-estimate", 1_000_000)

	request := localtask.CostReserveRequest{
		UserID:         "u1",
		TaskID:         "task-estimate",
		TaskType:       "canvas_text",
		LogicalModelID: "lm-1",
		ChannelModelID: "cm-1",
		InputJSON:      `{"prompt":"生成一段短剧分镜"}`,
	}
	reservation, err := adapter.ReserveTaskCost(request)
	if err != nil {
		t.Fatalf("画布文本任务没有显式 token 提示时也应能按预估冻结: %v", err)
	}
	if reservation == nil || reservation.Free || reservation.EstimatedCredits <= 0 {
		t.Fatalf("应产生文本预估冻结，得到 %#v", reservation)
	}
	charge, found, err := repo.ReservedTaskCostCharge("task-estimate")
	if err != nil || !found {
		t.Fatalf("缺少预冻结快照: found=%v err=%v", found, err)
	}
	if charge.BillingMode != "per_1k_tokens" || charge.Quantity <= 0 {
		t.Fatalf("文本预估应记录按千 token 计费用量: %#v", charge)
	}
}

func TestTaskCostChargesSystemChannelConfiguredOnlyInInput(t *testing.T) {
	adapter, repo, _ := newTaskCostEnv(t)
	channelID, channelModelID := seedTaskCostSystemChannel(t, repo)
	seedTaskCostChannelTier(t, repo, channelID, channelModelID, 2_000_000)

	request := imageCostRequest("u1", "task-raw-system")
	request.LogicalModelID = ""
	request.ChannelModelID = ""
	request.InputJSON = fmt.Sprintf(`{"prompt":"hello","config":{"channelId":%q,"model":"image-model"}}`, channelID)

	if _, err := adapter.ReserveTaskCost(request); err == nil {
		t.Fatal("a system channel task without resolved route IDs must not bypass billing")
	}
}

func TestTaskCostReservesSystemChannelConfiguredOnlyInInput(t *testing.T) {
	adapter, repo, wallet := newTaskCostEnv(t)
	fundTestWallet(t, wallet, "u1", 10)
	channelID, channelModelID := seedTaskCostSystemChannel(t, repo)
	seedTaskCostChannelTier(t, repo, channelID, channelModelID, 2_000_000)

	request := imageCostRequest("u1", "task-raw-system-paid")
	request.LogicalModelID = ""
	request.ChannelModelID = ""
	request.InputJSON = fmt.Sprintf(`{"prompt":"hello","config":{"channelId":%q,"model":"image-model"}}`, channelID)

	reservation, err := adapter.ReserveTaskCost(request)
	if err != nil {
		t.Fatalf("system channel price should be found from the input config: %v", err)
	}
	if reservation == nil || reservation.Free || reservation.EstimatedCredits != 2*credits.MicrocreditsPerCredit {
		t.Fatalf("system channel task must reserve the configured price: %#v", reservation)
	}
	charge, found, err := repo.ReservedTaskCostCharge(request.TaskID)
	if err != nil || !found || charge.RuleKind != model.PricingKindChannelTier {
		t.Fatalf("unexpected charge snapshot: found=%v err=%v charge=%#v", found, err, charge)
	}
}

func TestTaskCostRejectsSystemChannelWithoutPriceRule(t *testing.T) {
	adapter, repo, _ := newTaskCostEnv(t)
	channelID, _ := seedTaskCostSystemChannel(t, repo)

	request := imageCostRequest("u1", "task-raw-system-no-price")
	request.LogicalModelID = ""
	request.ChannelModelID = ""
	request.InputJSON = fmt.Sprintf(`{"prompt":"hello","config":{"channelId":%q,"model":"image-model"}}`, channelID)

	if _, err := adapter.ReserveTaskCost(request); err == nil {
		t.Fatal("a system channel task without a price rule must fail closed")
	}
	if _, found, err := repo.ReservedTaskCostCharge(request.TaskID); err != nil || found {
		t.Fatalf("rejected system channel task must not leave a reservation: found=%v err=%v", found, err)
	}
}

func seedTaskCostSystemChannel(t *testing.T, repo *repository.Repository) (string, string) {
	t.Helper()
	channel := &model.ModelChannel{
		ID: "channel-system", Scope: model.ChannelScopeSystem, Enabled: true, Name: "System image",
		BaseURL: "https://8.8.8.8/v1", APIFormat: "openai", ModelsJSON: `["image-model"]`,
	}
	item := &model.ChannelModel{
		ID: "cm-system", ChannelID: channel.ID, ModelKey: "image-model", ProviderModelKey: "upstream-image",
		Capability: "image", Protocol: model.ChannelInterfaceOpenAIImage, Enabled: true,
	}
	if err := repo.DB().Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.DB().Create(item).Error; err != nil {
		t.Fatal(err)
	}
	return channel.ID, item.ID
}

func seedTaskCostChannelTier(t *testing.T, repo *repository.Repository, channelID, channelModelID string, unitPriceMicrocredits int64) {
	t.Helper()
	rule := &model.PricingRule{
		ID: "rule-channel-tier", Kind: model.PricingKindChannelTier, Name: "System image tier",
		ChannelID: channelID, ChannelModelID: channelModelID, Capability: "image", BillingMode: "fixed_request",
		UnitPriceMicrocredits: unitPriceMicrocredits, Enabled: true,
		EffectiveFrom: taskCostTestTime.Add(-time.Hour), CreatedAt: taskCostTestTime, UpdatedAt: taskCostTestTime,
	}
	if err := repo.CreatePricingRule(model.PricingKindChannelTier, rule); err != nil {
		t.Fatal(err)
	}
}
