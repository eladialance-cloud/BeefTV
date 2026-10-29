package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"
)

// taskCostAdapter is the hosted-only implementation of the task domain's cost
// seam. It resolves a price rule, writes the immutable quote snapshot, freezes
// the estimate, and then settles or releases it at the task's terminal state.
//
// The adapter lives in cmd/server on purpose: internal/task and internal/app may
// not import internal/credits, and the desktop binary must not link them.
type taskCostAdapter struct {
	repo    *repository.Repository
	credits *credits.Service
	pricing *credits.PricingService
	now     func() time.Time
}

func newTaskCostAdapter(repo *repository.Repository, wallet *credits.Service, pricing *credits.PricingService) *taskCostAdapter {
	return &taskCostAdapter{
		repo:    repo,
		credits: wallet,
		pricing: pricing,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// ReserveTaskCost prices the task, snapshots the decision, and freezes the
// estimate. Provider-backed tasks fail closed when no rule matches; local
// executors and user-supplied workflow calls are intentionally free so hosted
// deployments keep working before every capability has a price.
func (a *taskCostAdapter) ReserveTaskCost(request localtask.CostReserveRequest) (*localtask.CostReservation, error) {
	if a == nil || a.repo == nil || a.credits == nil || a.pricing == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "任务计费服务尚未初始化")
	}
	taskID := strings.TrimSpace(request.TaskID)
	if taskID == "" {
		return nil, kernel.NewAppError(kernel.CodeInvalidArgument, "缺少任务标识")
	}
	input := decodeTaskCostInput(request.InputJSON)
	logicalModelID := strings.TrimSpace(request.LogicalModelID)
	channelID := taskCostChannelID(input)
	channelModelID := strings.TrimSpace(request.ChannelModelID)
	if channelModelID == "" && channelID != "" {
		modelKey := strings.TrimPrefix(strings.TrimSpace(taskCostString(input, "model")), "models/")
		if modelKey == "" {
			return nil, kernel.NotFound("当前系统渠道未指定模型，无法计价")
		}
		channelModel, err := a.repo.ChannelModelByKey(channelID, modelKey)
		if err != nil {
			return nil, kernel.NotFound("当前系统渠道未授权该模型，无法计价")
		}
		channelModelID = channelModel.ID
	}
	capability := taskCostCapability(request.TaskType)
	identity := credits.QuoteRequest{
		LogicalModelID: logicalModelID,
		ChannelID:      channelID,
		ChannelModelID: channelModelID,
		ModelKey:       strings.TrimPrefix(strings.TrimSpace(taskCostString(input, "model")), "models/"),
		Capability:     capability,
	}
	if err := enrichPricingQuoteIdentity(&identity, a.repo); err != nil {
		return nil, err
	}
	logicalModelID = identity.LogicalModelID
	channelID = identity.ChannelID
	channelModelID = identity.ChannelModelID
	if logicalModelID == "" && channelModelID == "" && channelID == "" {
		// Custom-channel and workflow tasks run on the user's own credentials.
		return &localtask.CostReservation{Free: true}, nil
	}
	selector := taskCostSelector(input)
	fallback := taskCostFallbackSettings(a, channelModelID, capability)
	if len(selector) == 0 && fallback.DefaultResolution != "" {
		selector = map[string]string{"quality": strings.ToLower(fallback.DefaultResolution)}
	}
	candidates := taskCostCandidates(capability, input)
	if capability == "video" && len(candidates) == 1 && fallback.DefaultDuration > 0 {
		candidates = append(candidates, taskCostCandidate{Mode: "per_second", Quantity: int64(fallback.DefaultDuration)})
	}

	var (
		quote *credits.Quote
		mode  string
	)
	for _, candidate := range candidates {
		quoted, err := a.pricing.Quote(credits.QuoteRequest{
			LogicalModelID: logicalModelID,
			ChannelID:      channelID,
			ChannelModelID: channelModelID,
			Capability:     capability,
			BillingMode:    candidate.Mode,
			Selector:       selector,
			Quantity:       candidate.Quantity,
			At:             a.now(),
		})
		if err == nil {
			quote, mode = quoted, candidate.Mode
			break
		}
		if !isNotFound(err) {
			return nil, err
		}
	}
	if quote == nil {
		if taskCostIsLocalExecutor(request.TaskType) {
			return &localtask.CostReservation{Free: true}, nil
		}
		quality := strings.TrimSpace(selector["quality"])
		if quality == "" {
			quality = "未指定"
		}
		return nil, kernel.NotFound(fmt.Sprintf(
			"未找到有效的模型价格规则，请联系管理员配置后再提交（能力=%s，分辨率=%s，时长=%d秒）",
			capability, quality, firstTaskCostCandidateQuantity(candidates),
		))
	}

	attempt, err := a.repo.NextTaskCostAttempt(taskID)
	if err != nil {
		return nil, err
	}
	charge := &model.TaskCostCharge{
		ID:                    kernel.NewID(),
		TaskID:                taskID,
		UserID:                request.UserID,
		Attempt:               attempt,
		TaskType:              request.TaskType,
		Capability:            capability,
		BillingMode:           mode,
		RuleID:                quote.RuleID,
		RuleKind:              quote.RuleKind,
		RuleName:              quote.RuleName,
		Quantity:              quote.Quantity,
		UnitPriceMicrocredits: quote.UnitPriceMicrocredits,
		MinimumChargeCredits:  quote.MinimumChargeCredits,
		EstimatedCredits:      quote.EstimatedCredits,
		Status:                model.TaskCostStatusReserved,
		CreatedAt:             a.now(),
	}
	if err := a.repo.CreateTaskCostCharge(charge); err != nil {
		return nil, err
	}
	if _, err := a.credits.ReserveAttempt(request.UserID, taskID, attempt, quote.EstimatedCredits); err != nil {
		// The freeze never landed: drop the snapshot so the next submit starts
		// from a clean attempt instead of inheriting a phantom reservation.
		_ = a.repo.DeleteTaskCostCharge(taskID, attempt)
		return nil, err
	}
	return &localtask.CostReservation{
		ChargeID:         charge.ID,
		Attempt:          attempt,
		RuleID:           quote.RuleID,
		RuleKind:         quote.RuleKind,
		BillingMode:      mode,
		EstimatedCredits: quote.EstimatedCredits,
	}, nil
}

type taskCostFallback struct {
	DefaultResolution string
	DefaultDuration   int
}

func taskCostFallbackSettings(adapter *taskCostAdapter, channelModelID, capability string) taskCostFallback {
	if adapter == nil || adapter.repo == nil || strings.TrimSpace(channelModelID) == "" {
		return taskCostFallback{}
	}
	channelModel, err := adapter.repo.ChannelModel(channelModelID)
	if err != nil || channelModel == nil || strings.TrimSpace(channelModel.CapabilityConfigJSON) == "" {
		return taskCostFallback{}
	}
	return taskCostFallbackSettingsFromJSON(channelModel.CapabilityConfigJSON, capability)
}

func taskCostFallbackSettingsFromJSON(raw, capability string) taskCostFallback {
	var config struct {
		Image struct {
			Quality struct {
				Default string `json:"default"`
			} `json:"quality"`
		} `json:"image"`
		Video struct {
			DefaultResolution string `json:"defaultResolution"`
			Duration          struct {
				Default int `json:"default"`
			} `json:"duration"`
		} `json:"video"`
	}
	if json.Unmarshal([]byte(raw), &config) != nil {
		return taskCostFallback{}
	}
	switch capability {
	case "image":
		return taskCostFallback{DefaultResolution: strings.TrimSpace(config.Image.Quality.Default)}
	case "video":
		return taskCostFallback{DefaultResolution: strings.TrimSpace(config.Video.DefaultResolution), DefaultDuration: config.Video.Duration.Default}
	default:
		return taskCostFallback{}
	}
}

func firstTaskCostCandidateQuantity(candidates []taskCostCandidate) int64 {
	for _, candidate := range candidates {
		if candidate.Mode != "fixed_request" && candidate.Quantity > 0 {
			return candidate.Quantity
		}
	}
	return 0
}

// SettleTaskCost converts the outstanding freeze into real usage. With no
// upstream usage receipt yet the estimate is settled as-is rather than
// inventing a number.
func (a *taskCostAdapter) SettleTaskCost(request localtask.CostSettleRequest) error {
	charge, ok, err := a.reservedCharge(request.UserID, request.TaskID)
	if err != nil || !ok {
		return err
	}
	settlement := a.settlement(charge, request.ActualCredits)
	if _, err := a.credits.SettleAttempt(charge.UserID, charge.TaskID, charge.Attempt, charge.EstimatedCredits, settlement.ActualCredits); err != nil {
		return err
	}
	settlement.At = a.now()
	if _, err := a.repo.SettleTaskCostCharge(charge.TaskID, charge.Attempt, settlement); err != nil {
		return err
	}
	return nil
}

// settlement decides what a succeeded task actually costs. A caller-supplied
// override wins; otherwise the provider's own usage receipts are re-priced
// against the frozen snapshot. Anything unmeasurable falls back to the frozen
// estimate, and real usage above the freeze is capped at the freeze so a user
// is never charged more than the amount authorized at submit time.
func (a *taskCostAdapter) settlement(charge *model.TaskCostCharge, override int64) repository.TaskCostSettlement {
	estimate := repository.TaskCostSettlement{
		ActualCredits:  charge.EstimatedCredits,
		ActualQuantity: charge.Quantity,
		Source:         "estimate",
	}
	if override > 0 {
		return repository.TaskCostSettlement{
			ActualCredits:  override,
			ActualQuantity: charge.Quantity,
			Source:         "caller",
			Capped:         override > charge.EstimatedCredits,
		}
	}
	usage, err := a.repo.TaskUsageSummary(charge.TaskID)
	if err != nil {
		return estimate
	}
	quantity, ok := taskUsageQuantity(charge.BillingMode, usage)
	if !ok {
		return estimate
	}
	computed, err := credits.CalculateCredits(charge.UnitPriceMicrocredits, quantity, charge.MinimumChargeCredits)
	if err != nil || computed <= 0 {
		return estimate
	}
	if computed > charge.EstimatedCredits {
		return repository.TaskCostSettlement{
			ActualCredits:  charge.EstimatedCredits,
			ActualQuantity: quantity,
			Source:         "provider_usage",
			Capped:         true,
		}
	}
	return repository.TaskCostSettlement{
		ActualCredits:  computed,
		ActualQuantity: quantity,
		Source:         "provider_usage",
	}
}

// taskUsageQuantity maps a billing mode onto the real quantity behind it. A
// mode with no measurable provider signal reports ok=false so settlement keeps
// the frozen estimate instead of guessing.
func taskUsageQuantity(mode string, usage repository.TaskUsageSummary) (int64, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "per_1k_tokens":
		total := usage.InputTokens + usage.OutputTokens
		if !usage.UsageAvailable || total <= 0 {
			return 0, false
		}
		return int64(math.Ceil(float64(total) / 1000.0)), true
	case "per_second":
		if usage.VideoSeconds <= 0 {
			return 0, false
		}
		return int64(usage.VideoSeconds), true
	case "per_image":
		if usage.MediaCount <= 0 {
			return 0, false
		}
		return int64(usage.MediaCount), true
	default:
		return 0, false
	}
}

// ReleaseTaskCost returns the outstanding freeze for a failed or cancelled task.
func (a *taskCostAdapter) ReleaseTaskCost(request localtask.CostReleaseRequest) error {
	charge, ok, err := a.reservedCharge(request.UserID, request.TaskID)
	if err != nil || !ok {
		return err
	}
	if _, err := a.credits.ReleaseAttempt(charge.UserID, charge.TaskID, charge.Attempt, charge.EstimatedCredits); err != nil {
		return err
	}
	if _, err := a.repo.ReleaseTaskCostCharge(charge.TaskID, charge.Attempt, strings.TrimSpace(request.Reason), a.now()); err != nil {
		return err
	}
	return nil
}

// reservedCharge loads the outstanding reservation and rejects a mismatched
// caller. A missing reservation is a no-op so legacy tasks admitted before
// billing existed keep their old behaviour.
func (a *taskCostAdapter) reservedCharge(userID, taskID string) (*model.TaskCostCharge, bool, error) {
	if a == nil || a.repo == nil || a.credits == nil {
		return nil, false, kernel.NewAppError(kernel.CodeUnavailable, "任务计费服务尚未初始化")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false, nil
	}
	charge, found, err := a.repo.ReservedTaskCostCharge(taskID)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	owner := strings.TrimSpace(userID)
	if owner != "" && owner != charge.UserID {
		return nil, false, kernel.NewAppError(kernel.CodeConflict, "任务计费归属不一致")
	}
	return charge, true, nil
}

type taskCostCandidate struct {
	Mode     string
	Quantity int64
}

// taskCostCandidates lists the billing modes to try, in priority order. The
// fixed-request rule always wins when an operator prices both a flat and a
// usage-based rule.
func taskCostCandidates(capability string, input map[string]any) []taskCostCandidate {
	out := []taskCostCandidate{{Mode: "fixed_request", Quantity: 1}}
	switch capability {
	case "video":
		if seconds, ok := taskCostNumber(input, "duration", "durationSeconds", "videoDuration", "videoSeconds", "seconds"); ok && seconds > 0 {
			out = append(out, taskCostCandidate{Mode: "per_second", Quantity: seconds})
		}
	case "image":
		if count, ok := taskCostNumber(input, "imageCount", "numImages", "batchSize", "count"); ok && count > 0 {
			out = append(out, taskCostCandidate{Mode: "per_image", Quantity: count})
		}
	case "audio":
		if seconds, ok := taskCostNumber(input, "duration", "durationSeconds", "audioDuration", "seconds"); ok && seconds > 0 {
			out = append(out, taskCostCandidate{Mode: "per_second", Quantity: seconds})
		}
		if megabytes, ok := taskCostNumber(input, "sizeMegabytes", "megabytes", "fileSizeMb"); ok && megabytes > 0 {
			out = append(out, taskCostCandidate{Mode: "per_megabyte", Quantity: megabytes})
		}
	case "text":
		if tokens, ok := taskCostNumber(input, "maxTokens", "tokens", "tokenCount"); ok && tokens > 0 {
			out = append(out, taskCostCandidate{Mode: "per_1k_tokens", Quantity: int64(math.Ceil(float64(tokens) / 1000.0))})
		} else if tokens := taskCostEstimatedTextTokens(input); tokens > 0 {
			// Canvas text tasks do not carry a provider token budget. Reserve a
			// conservative prompt + output estimate so per-token pricing remains
			// usable; settlement still re-prices against real upstream usage and is
			// capped at this authorized freeze.
			out = append(out, taskCostCandidate{Mode: "per_1k_tokens", Quantity: int64(math.Ceil(float64(tokens) / 1000.0))})
		}
	}
	return out
}

// taskCostEstimatedTextTokens reserves prompt text plus a conservative output
// allowance. The estimate is only an authorization ceiling; provider usage
// settles the final amount when a receipt is available.
func taskCostEstimatedTextTokens(input map[string]any) int64 {
	prompt := taskCostString(input, "prompt")
	if strings.TrimSpace(prompt) == "" {
		return 0
	}
	var ascii, nonASCII int64
	for _, char := range prompt {
		if char <= 127 {
			ascii++
		} else {
			nonASCII++
		}
	}
	// Rough tokenizer estimate: four ASCII characters or one non-ASCII
	// character per token. A 4096-token output allowance mirrors the common
	// direct-provider default used by the built-in text protocols.
	return int64(math.Ceil(float64(ascii)/4.0)) + nonASCII + 16 + 4096
}

func taskCostString(input map[string]any, key string) string {
	if value, ok := input[key].(string); ok {
		return value
	}
	if config, ok := input["config"].(map[string]any); ok {
		if value, found := config[key].(string); found {
			return value
		}
	}
	return ""
}

func taskCostChannelID(input map[string]any) string {
	return strings.TrimSpace(taskCostString(input, "channelId"))
}

// taskCostCapability maps a task type onto the capability string used by the
// pricing rules ("video", "image", "audio", "text", "local").
func taskCostCapability(taskType string) string {
	switch taskType {
	case model.TaskTypeTimelineTranscription, model.TaskTypeTimelineRender, model.TaskTypeDepthCapture:
		return "local"
	}
	value := strings.ToLower(strings.TrimSpace(taskType))
	switch {
	case strings.Contains(value, "video"):
		return "video"
	case strings.Contains(value, "image"), strings.Contains(value, "portrait"):
		return "image"
	case strings.Contains(value, "audio"), strings.Contains(value, "speech"), strings.Contains(value, "music"):
		return "audio"
	default:
		return "text"
	}
}

func taskCostIsLocalExecutor(taskType string) bool {
	switch taskType {
	case model.TaskTypeTimelineTranscription, model.TaskTypeTimelineRender, model.TaskTypeDepthCapture:
		return true
	default:
		return false
	}
}

// taskCostSelector reads an optional pricing selector from the task input. It
// stays empty unless the caller (or an operator-authored request) supplies one;
// rules without a selector then match everything.
func taskCostSelector(input map[string]any) map[string]string {
	raw := input
	if nested, ok := raw["selector"].(map[string]any); ok {
		if selector := stringifySelector(nested); selector != nil {
			return selector
		}
	}
	if config, ok := raw["config"].(map[string]any); ok {
		if nested, ok := config["selector"].(map[string]any); ok {
			if selector := stringifySelector(nested); selector != nil {
				return selector
			}
		}
	}
	quality := normalizeTaskCostResolution(firstTaskCostString(input, "vquality", "resolution", "quality"))
	switch quality {
	case "", "auto", "standard":
		return nil
	default:
		return map[string]string{"quality": quality}
	}
}

func normalizeTaskCostResolution(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "480", "720", "768", "1080", "1440", "2160":
		return value + "p"
	default:
		return value
	}
}

func firstTaskCostString(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := taskCostString(input, key); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringifySelector(selector map[string]any) map[string]string {
	if len(selector) == 0 {
		return nil
	}
	out := make(map[string]string, len(selector))
	for key, value := range selector {
		text := strings.TrimSpace(selectorString(value))
		if text == "" {
			continue
		}
		out[key] = text
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func selectorString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func decodeTaskCostInput(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil || input == nil {
		return map[string]any{}
	}
	return input
}

// taskCostNumber looks for a numeric hint at the top level and inside config.
func taskCostNumber(input map[string]any, keys ...string) (int64, bool) {
	if value, ok := lookupTaskCostNumber(input, keys...); ok {
		return value, true
	}
	if config, ok := input["config"].(map[string]any); ok {
		if value, found := lookupTaskCostNumber(config, keys...); found {
			return value, true
		}
	}
	return 0, false
}

func lookupTaskCostNumber(input map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		value, ok := input[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return int64(math.Round(typed)), true
			}
		case int:
			if typed > 0 {
				return int64(typed), true
			}
		case int64:
			if typed > 0 {
				return typed, true
			}
		case json.Number:
			if parsed, err := typed.Int64(); err == nil && parsed > 0 {
				return parsed, true
			}
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			if err == nil && parsed > 0 {
				return int64(math.Round(parsed)), true
			}
		}
	}
	return 0, false
}

func isNotFound(err error) bool {
	var appErr *kernel.AppError
	if errors.As(err, &appErr) {
		return appErr.Status == kernel.CodeNotFound
	}
	return false
}
