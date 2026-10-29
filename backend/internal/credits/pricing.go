package credits

import (
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

// PricingRepository is the durable port for editable model pricing. Rules are
// returned as the common model.PricingRule view so the service can resolve a
// quote without knowing which hosted table supplied the row.
type PricingRepository interface {
	PricingRules(kind string) ([]model.PricingRule, error)
	PricingRule(kind string, id string) (*model.PricingRule, bool, error)
	CreatePricingRule(kind string, rule *model.PricingRule) error
	UpdatePricingRule(kind string, rule *model.PricingRule) error
	DeletePricingRule(kind string, id string) error
	AppendAdminAudit(*model.AdminAuditEvent) error
}

// PricingInput is the admin write payload shared by all three pricing layers.
// The kind comes from the route, not from the request body, so a client cannot
// change an existing rule into another table by accident.
type PricingInput struct {
	Name                    string            `json:"name"`
	LogicalModelID          string            `json:"logicalModelId"`
	ChannelID               string            `json:"channelId"`
	ChannelModelID          string            `json:"channelModelId"`
	ChannelModelPriceTierID string            `json:"channelModelPriceTierId"`
	Capability              string            `json:"capability"`
	BillingMode             string            `json:"billingMode"`
	Selector                map[string]string `json:"selector"`
	UnitPriceMicrocredits   int64             `json:"unitPriceMicrocredits"`
	// MinimumChargeMicrocredits is the preferred fixed-point minimum. The legacy
	// MinimumChargeCredits field remains accepted as a whole-credit input.
	MinimumChargeMicrocredits *int64     `json:"minimumChargeMicrocredits"`
	MinimumChargeCredits      *int64     `json:"minimumChargeCredits"`
	Enabled                   *bool      `json:"enabled"`
	SortOrder                 *int       `json:"sortOrder"`
	EffectiveFrom             time.Time  `json:"effectiveFrom"`
	EffectiveTo               *time.Time `json:"effectiveTo"`
	Note                      string     `json:"note"`
}

// QuoteRequest is the runtime-side input to the pricing resolver. Quantity is
// expressed in the billing mode's unit: one request, one second, one image, or
// one thousand tokens.
type QuoteRequest struct {
	LogicalModelID string
	ChannelID      string
	ChannelModelID string
	ModelKey       string
	Capability     string
	BillingMode    string
	Selector       map[string]string
	Quantity       int64
	At             time.Time
}

// Quote is an immutable pricing decision. Task submission must persist the
// selected rule identity and calculated microcredits if settlement happens later.
type Quote struct {
	RuleID                string     `json:"ruleId"`
	RuleKind              string     `json:"ruleKind"`
	RuleName              string     `json:"ruleName,omitempty"`
	BillingMode           string     `json:"billingMode"`
	Quantity              int64      `json:"quantity"`
	UnitPriceMicrocredits int64      `json:"unitPriceMicrocredits"`
	MinimumChargeCredits  int64      `json:"minimumChargeCredits"`
	EstimatedCredits      int64      `json:"estimatedCredits"`
	EffectiveFrom         time.Time  `json:"effectiveFrom"`
	EffectiveTo           *time.Time `json:"effectiveTo,omitempty"`
}

// PricingService owns editable price rules and the quote resolver.
type PricingService struct {
	repo PricingRepository
	now  func() time.Time
}

// NewPricing wires the hosted-only pricing service.
func NewPricing(repo PricingRepository, config Config) *PricingService {
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &PricingService{repo: repo, now: now}
}

// Rules lists one pricing layer for the admin console. All rows are returned,
// including disabled and future-dated ones, so operators can prepare changes.
func (s *PricingService) Rules(kind string) ([]model.PricingRule, error) {
	normalized, err := normalizePricingKind(kind)
	if err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "价格规则服务尚未初始化")
	}
	items, err := s.repo.PricingRules(normalized)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Save creates or updates one rule. id empty means create.
func (s *PricingService) Save(actor *model.User, kind string, id string, input PricingInput) (*model.PricingRule, error) {
	normalized, err := normalizePricingKind(kind)
	if err != nil {
		return nil, err
	}
	if err := requirePricingAdmin(actor); err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "价格规则服务尚未初始化")
	}

	var existing *model.PricingRule
	if strings.TrimSpace(id) != "" {
		loaded, found, loadErr := s.repo.PricingRule(normalized, strings.TrimSpace(id))
		if loadErr != nil {
			return nil, loadErr
		}
		if !found {
			return nil, kernel.NotFound("价格规则不存在")
		}
		existing = loaded
	}

	now := s.now().UTC()
	rule, err := buildPricingRule(normalized, input, existing, actor.ID, now)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		if err := s.repo.CreatePricingRule(normalized, rule); err != nil {
			return nil, err
		}
	} else if err := s.repo.UpdatePricingRule(normalized, rule); err != nil {
		return nil, err
	}
	action := "pricing_rule.create"
	if existing != nil {
		action = "pricing_rule.update"
	}
	if err := s.repo.AppendAdminAudit(&model.AdminAuditEvent{
		ID: kernel.NewID(), ActorUserID: actor.ID, Action: action,
		TargetType: "pricing_rule:" + normalized, TargetID: rule.ID,
		Summary: rule.Name, CreatedAt: now,
	}); err != nil {
		return nil, err
	}
	return rule, nil
}

// Delete removes one rule. Historical task quotes must carry their own
// snapshot, so deleting a rule does not rewrite old bills.
func (s *PricingService) Delete(actor *model.User, kind string, id string) error {
	normalized, err := normalizePricingKind(kind)
	if err != nil {
		return err
	}
	if err := requirePricingAdmin(actor); err != nil {
		return err
	}
	if s == nil || s.repo == nil {
		return kernel.NewAppError(kernel.CodeUnavailable, "价格规则服务尚未初始化")
	}
	existing, found, err := s.repo.PricingRule(normalized, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if !found {
		return kernel.NotFound("价格规则不存在")
	}
	if err := s.repo.DeletePricingRule(normalized, existing.ID); err != nil {
		return err
	}
	return s.repo.AppendAdminAudit(&model.AdminAuditEvent{
		ID: kernel.NewID(), ActorUserID: actor.ID, Action: "pricing_rule.delete",
		TargetType: "pricing_rule:" + normalized, TargetID: existing.ID,
		Summary: existing.Name, CreatedAt: s.now().UTC(),
	})
}

// Quote resolves the effective rule for one request. It prefers a published
// logical-model SKU, then an executable channel-model tier, then the logical
// model default. Task submission should persist the returned decision.
func (s *PricingService) Quote(request QuoteRequest) (*Quote, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "价格规则服务尚未初始化")
	}
	request.LogicalModelID = strings.TrimSpace(request.LogicalModelID)
	request.ChannelID = strings.TrimSpace(request.ChannelID)
	request.ChannelModelID = strings.TrimSpace(request.ChannelModelID)
	request.Capability = strings.TrimSpace(request.Capability)
	request.BillingMode = strings.ToLower(strings.TrimSpace(request.BillingMode))
	if request.Capability == "" {
		return nil, kernel.BadAuthRequest("缺少模型能力")
	}
	if err := validateBillingMode(request.BillingMode); err != nil {
		return nil, err
	}
	if request.Quantity <= 0 {
		return nil, kernel.BadAuthRequest("计费用量必须大于零")
	}
	if request.At.IsZero() {
		request.At = s.now()
	} else {
		request.At = request.At.UTC()
	}
	selector, _, err := model.CanonicalSKUSelector(request.Selector)
	if err != nil {
		return nil, kernel.BadAuthRequest("价格选择器格式错误")
	}

	for _, kind := range []string{model.PricingKindLogicalSKU, model.PricingKindChannelTier, model.PricingKindModel} {
		rules, err := s.repo.PricingRules(kind)
		if err != nil {
			return nil, err
		}
		selected := selectEffectiveRule(rules, kind, request, selector)
		if selected == nil {
			continue
		}
		credits, err := calculateCredits(selected.UnitPriceMicrocredits, request.Quantity, selected.MinimumChargeCredits)
		if err != nil {
			return nil, err
		}
		return &Quote{
			RuleID: selected.ID, RuleKind: selected.Kind, RuleName: selected.Name,
			BillingMode: selected.BillingMode, Quantity: request.Quantity,
			UnitPriceMicrocredits: selected.UnitPriceMicrocredits,
			MinimumChargeCredits:  selected.MinimumChargeCredits,
			EstimatedCredits:      credits,
			EffectiveFrom:         selected.EffectiveFrom,
			EffectiveTo:           selected.EffectiveTo,
		}, nil
	}
	return nil, kernel.NotFound("未找到有效的模型价格规则")
}

func buildPricingRule(kind string, input PricingInput, existing *model.PricingRule, actorID string, now time.Time) (*model.PricingRule, error) {
	rule := &model.PricingRule{
		ID:        kernel.NewID(),
		Kind:      kind,
		Enabled:   true,
		CreatedBy: actorID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing != nil {
		rule.ID = existing.ID
		rule.Kind = existing.Kind
		rule.Enabled = existing.Enabled
		rule.CreatedBy = existing.CreatedBy
		rule.CreatedAt = existing.CreatedAt
	}

	rule.Name = strings.TrimSpace(input.Name)
	if rule.Name == "" && existing != nil {
		rule.Name = existing.Name
	}
	rule.LogicalModelID = strings.TrimSpace(input.LogicalModelID)
	if rule.LogicalModelID == "" && existing != nil {
		rule.LogicalModelID = existing.LogicalModelID
	}
	rule.ChannelID = strings.TrimSpace(input.ChannelID)
	if rule.ChannelID == "" && existing != nil {
		rule.ChannelID = existing.ChannelID
	}
	rule.ChannelModelID = strings.TrimSpace(input.ChannelModelID)
	if rule.ChannelModelID == "" && existing != nil {
		rule.ChannelModelID = existing.ChannelModelID
	}
	rule.ChannelModelPriceTierID = strings.TrimSpace(input.ChannelModelPriceTierID)
	if rule.ChannelModelPriceTierID == "" && existing != nil {
		rule.ChannelModelPriceTierID = existing.ChannelModelPriceTierID
	}
	rule.Capability = strings.ToLower(strings.TrimSpace(input.Capability))
	if rule.Capability == "" && existing != nil {
		rule.Capability = existing.Capability
	}
	rule.BillingMode = strings.ToLower(strings.TrimSpace(input.BillingMode))
	if rule.BillingMode == "" && existing != nil {
		rule.BillingMode = existing.BillingMode
	}
	if input.Selector != nil {
		selector, _, err := model.CanonicalSKUSelector(input.Selector)
		if err != nil {
			return nil, kernel.BadAuthRequest("价格选择器格式错误")
		}
		rule.Selector = selector
	} else if existing != nil {
		rule.Selector = cloneSelector(existing.Selector)
	}
	rule.UnitPriceMicrocredits = input.UnitPriceMicrocredits
	if rule.UnitPriceMicrocredits == 0 && existing != nil {
		rule.UnitPriceMicrocredits = existing.UnitPriceMicrocredits
	}
	if input.MinimumChargeMicrocredits != nil {
		rule.MinimumChargeCredits = *input.MinimumChargeMicrocredits
	} else if input.MinimumChargeCredits != nil {
		rule.MinimumChargeCredits = FromWholeCredits(*input.MinimumChargeCredits)
	} else if existing != nil {
		rule.MinimumChargeCredits = existing.MinimumChargeCredits
	}
	if input.Enabled != nil {
		rule.Enabled = *input.Enabled
	}
	rule.SortOrder = 0
	if existing != nil {
		rule.SortOrder = existing.SortOrder
	}
	if input.SortOrder != nil {
		rule.SortOrder = *input.SortOrder
	}
	rule.EffectiveFrom = input.EffectiveFrom.UTC()
	if rule.EffectiveFrom.IsZero() {
		if existing != nil {
			rule.EffectiveFrom = existing.EffectiveFrom
		} else {
			rule.EffectiveFrom = now
		}
	}
	if input.EffectiveTo != nil {
		value := input.EffectiveTo.UTC()
		rule.EffectiveTo = &value
	} else if existing != nil {
		rule.EffectiveTo = existing.EffectiveTo
	}
	rule.Note = strings.TrimSpace(input.Note)
	if rule.Note == "" && existing != nil {
		rule.Note = existing.Note
	}

	if err := validatePricingRule(rule); err != nil {
		return nil, err
	}
	if rule.Name == "" {
		switch kind {
		case model.PricingKindModel:
			rule.Name = rule.LogicalModelID + " 默认价"
		case model.PricingKindChannelTier:
			rule.Name = rule.ChannelModelID + " 渠道档"
		case model.PricingKindLogicalSKU:
			rule.Name = rule.LogicalModelID + " SKU"
		}
	}
	return rule, nil
}

func validatePricingRule(rule *model.PricingRule) error {
	if rule == nil {
		return kernel.BadAuthRequest("价格规则不能为空")
	}
	if rule.Capability == "" {
		return kernel.BadAuthRequest("请选择模型能力")
	}
	if err := validateBillingMode(rule.BillingMode); err != nil {
		return err
	}
	if rule.UnitPriceMicrocredits <= 0 {
		return kernel.BadAuthRequest("单价必须大于零")
	}
	if rule.MinimumChargeCredits < 0 {
		return kernel.BadAuthRequest("最低扣费不能为负数")
	}
	if rule.EffectiveTo != nil && !rule.EffectiveTo.After(rule.EffectiveFrom) {
		return kernel.BadAuthRequest("失效时间必须晚于生效时间")
	}
	switch rule.Kind {
	case model.PricingKindModel:
		if rule.LogicalModelID == "" {
			return kernel.BadAuthRequest("模型默认价必须填写逻辑模型")
		}
	case model.PricingKindChannelTier:
		if rule.ChannelID == "" {
			return kernel.BadAuthRequest("渠道价格档必须填写渠道")
		}
		if rule.ChannelModelID == "" {
			return kernel.BadAuthRequest("渠道价格档必须填写渠道模型")
		}
	case model.PricingKindLogicalSKU:
		if rule.LogicalModelID == "" {
			return kernel.BadAuthRequest("逻辑模型 SKU 必须填写逻辑模型")
		}
		if len(rule.Selector) == 0 {
			return kernel.BadAuthRequest("逻辑模型 SKU 必须填写选择器")
		}
	default:
		return kernel.BadAuthRequest("未知价格规则类型")
	}
	return nil
}

func selectEffectiveRule(rules []model.PricingRule, kind string, request QuoteRequest, selector map[string]string) *model.PricingRule {
	var selected *model.PricingRule
	for index := range rules {
		rule := rules[index]
		if !rule.Enabled || rule.Kind != kind || rule.Capability != request.Capability || rule.BillingMode != request.BillingMode {
			continue
		}
		if !ruleEffectiveAt(rule, request.At) {
			continue
		}
		if !selectorMatches(rule.Selector, selector) {
			continue
		}
		switch kind {
		case model.PricingKindModel:
			if request.LogicalModelID == "" || rule.LogicalModelID != request.LogicalModelID {
				continue
			}
		case model.PricingKindChannelTier:
			if request.ChannelModelID == "" || rule.ChannelModelID != request.ChannelModelID {
				continue
			}
			if request.ChannelID != "" && rule.ChannelID != request.ChannelID {
				continue
			}
		case model.PricingKindLogicalSKU:
			if request.LogicalModelID == "" || rule.LogicalModelID != request.LogicalModelID {
				continue
			}
		}
		if selected == nil ||
			len(rule.Selector) > len(selected.Selector) ||
			(len(rule.Selector) == len(selected.Selector) && pricingRuleNewer(rule, *selected)) {
			copy := rule
			selected = &copy
		}
	}
	return selected
}

func pricingRuleNewer(candidate model.PricingRule, current model.PricingRule) bool {
	if !candidate.EffectiveFrom.Equal(current.EffectiveFrom) {
		return candidate.EffectiveFrom.After(current.EffectiveFrom)
	}
	if candidate.SortOrder != current.SortOrder {
		return candidate.SortOrder < current.SortOrder
	}
	if !candidate.CreatedAt.Equal(current.CreatedAt) {
		return candidate.CreatedAt.After(current.CreatedAt)
	}
	return candidate.ID > current.ID
}

func ruleEffectiveAt(rule model.PricingRule, at time.Time) bool {
	if rule.EffectiveFrom.After(at) {
		return false
	}
	return rule.EffectiveTo == nil || at.Before(*rule.EffectiveTo)
}

func selectorMatches(ruleSelector map[string]string, requestSelector map[string]string) bool {
	if len(ruleSelector) == 0 {
		return true
	}
	if len(ruleSelector) != len(requestSelector) {
		return false
	}
	for key, value := range ruleSelector {
		if !pricingSelectorValueEqual(key, value, requestSelector[key]) {
			return false
		}
	}
	return true
}

func pricingSelectorValueEqual(key string, left string, right string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "quality", "resolution", "vquality":
		return normalizePricingResolution(left) == normalizePricingResolution(right)
	default:
		return left == right
	}
}

func normalizePricingResolution(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480", "480p":
		return "480p"
	case "720", "720p", "768", "768p":
		return "720p"
	case "1080", "1080p":
		return "1080p"
	case "1440", "1440p", "2k":
		return "2k"
	case "2160", "2160p", "4k":
		return "4k"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func cloneSelector(selector map[string]string) map[string]string {
	if len(selector) == 0 {
		return nil
	}
	copy := make(map[string]string, len(selector))
	for key, value := range selector {
		copy[key] = value
	}
	return copy
}

func calculateCredits(unitPriceMicrocredits int64, quantity int64, minimumMicrocredits int64) (int64, error) {
	if unitPriceMicrocredits <= 0 || quantity <= 0 {
		return 0, kernel.BadAuthRequest("计费用量和单价必须大于零")
	}
	if unitPriceMicrocredits > (1<<63-1)/quantity {
		return 0, kernel.BadAuthRequest("计费用量过大")
	}
	totalMicrocredits := unitPriceMicrocredits * quantity
	if totalMicrocredits < minimumMicrocredits {
		totalMicrocredits = minimumMicrocredits
	}
	if totalMicrocredits <= 0 {
		totalMicrocredits = 1
	}
	return totalMicrocredits, nil
}

// CalculateCredits exposes the quote math to settlement. A frozen pricing
// snapshot can therefore re-price real usage without reloading the rule, so
// an operator editing prices later never rewrites an in-flight task's bill.
func CalculateCredits(unitPriceMicrocredits, quantity, minimum int64) (int64, error) {
	return calculateCredits(unitPriceMicrocredits, quantity, minimum)
}

func normalizePricingKind(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case model.PricingKindModel:
		return model.PricingKindModel, nil
	case model.PricingKindChannelTier:
		return model.PricingKindChannelTier, nil
	case model.PricingKindLogicalSKU:
		return model.PricingKindLogicalSKU, nil
	default:
		return "", kernel.BadAuthRequest("未知价格规则类型")
	}
}

func validateBillingMode(raw string) error {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fixed_request", "per_second", "per_image", "per_1k_tokens", "per_megabyte":
		return nil
	default:
		return kernel.BadAuthRequest("暂不支持该计费方式")
	}
}

func requirePricingAdmin(actor *model.User) error {
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		return kernel.Unauthorized("请先登录")
	}
	if actor.Role != model.UserRoleAdmin {
		return kernel.Forbidden("需要管理员权限")
	}
	return nil
}
