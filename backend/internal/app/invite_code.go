package app

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	RegistrationPolicyOpen   = "open"
	RegistrationPolicyInvite = "invite"
	RegistrationPolicyClosed = "closed"

	inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// RegistrationPolicy 是站点级注册策略，存 system_settings，管理员可随时改。
type RegistrationPolicy struct {
	Mode                  string `json:"mode"`
	DefaultBonusCredits   int64  `json:"defaultBonusCredits"`
	InviteDefaultMaxUses  int    `json:"inviteDefaultMaxUses"`
	InviteDefaultValidDay int    `json:"inviteDefaultValidDays"`
}

func defaultRegistrationPolicy() RegistrationPolicy {
	return RegistrationPolicy{
		Mode:                  RegistrationPolicyOpen,
		DefaultBonusCredits:   0,
		InviteDefaultMaxUses:  1,
		InviteDefaultValidDay: 30,
	}
}

type InviteCodePage struct {
	Items []model.InviteCode `json:"items"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"pageSize"`
}

type InviteCodeBatchRequest struct {
	Count       int    `json:"count"`
	MaxUses     int    `json:"maxUses"`
	ValidDays   int    `json:"validDays"`
	Batch       string `json:"batch"`
	Note        string `json:"note"`
	Prefix      string `json:"prefix"`
	AutoEnabled bool   `json:"autoEnabled"`
}

// RegistrationPolicy 读取站点注册策略；未配置时返回默认值。
func (s *Service) RegistrationPolicy() (*RegistrationPolicy, error) {
	setting, err := s.repo.SystemSetting("registration_policy")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			policy := defaultRegistrationPolicy()
			return &policy, nil
		}
		return nil, err
	}
	policy := defaultRegistrationPolicy()
	if strings.TrimSpace(setting.ValueJSON) != "" {
		if err := json.Unmarshal([]byte(setting.ValueJSON), &policy); err != nil {
			policy = defaultRegistrationPolicy()
		}
	}
	policy.Mode = normalizeRegistrationMode(policy.Mode)
	if policy.InviteDefaultMaxUses <= 0 {
		policy.InviteDefaultMaxUses = 1
	}
	if policy.InviteDefaultValidDay < 0 {
		policy.InviteDefaultValidDay = 30
	}
	return &policy, nil
}

// RegistrationPolicyMode 是 auth 服务只需的最小策略视图。
func (s *Service) RegistrationPolicyMode() (string, int64, error) {
	policy, err := s.RegistrationPolicy()
	if err != nil {
		return RegistrationPolicyOpen, 0, err
	}
	return policy.Mode, policy.DefaultBonusCredits, nil
}

func (s *Service) UpdateRegistrationPolicy(actor *model.User, policy RegistrationPolicy) (*RegistrationPolicy, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	policy.Mode = normalizeRegistrationMode(policy.Mode)
	if policy.DefaultBonusCredits < 0 {
		return nil, BadAuthRequest("默认赠送积分不能为负数")
	}
	if policy.InviteDefaultMaxUses <= 0 {
		policy.InviteDefaultMaxUses = 1
	}
	if policy.InviteDefaultValidDay < 0 {
		return nil, BadAuthRequest("邀请码默认有效期不能为负数")
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveSystemSetting(&model.SystemSetting{
		Key:       "registration_policy",
		ValueJSON: string(encoded),
		UpdatedBy: actor.ID,
	}); err != nil {
		return nil, err
	}
	return &policy, nil
}

func (s *Service) AdminInviteCodePage(actor *model.User, query AdminListQuery) (*InviteCodePage, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	page, limit := normalizeAdminPage(query.Page, query.Limit)
	items, total, err := s.repo.AdminInviteCodes(query.Keyword, query.Status, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.InviteCode{}
	}
	return &InviteCodePage{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func (s *Service) GenerateInviteCodes(actor *model.User, req InviteCodeBatchRequest) ([]model.InviteCode, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 500 {
		return nil, BadAuthRequest("单次最多生成 500 个邀请码")
	}
	if req.MaxUses < 0 {
		return nil, BadAuthRequest("可用次数不能为负数")
	}
	if req.ValidDays < 0 {
		return nil, BadAuthRequest("有效期不能为负数")
	}
	maxUses := req.MaxUses
	if maxUses == 0 {
		maxUses = 1
	}
	now := time.Now().UTC()
	batch := strings.TrimSpace(req.Batch)
	if batch == "" {
		batch = "B" + now.Format("20060102-150405")
	}
	prefix := strings.ToUpper(strings.TrimSpace(req.Prefix))
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	status := "disabled"
	if req.AutoEnabled {
		status = "active"
	}
	var expiresAt *time.Time
	if req.ValidDays > 0 {
		value := now.AddDate(0, 0, req.ValidDays)
		expiresAt = &value
	}
	items := make([]model.InviteCode, 0, count)
	seen := map[string]struct{}{}
	for len(items) < count {
		code := prefix + randomInviteCode(10)
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		items = append(items, model.InviteCode{
			ID:        newID(),
			Code:      code,
			Batch:     batch,
			MaxUses:   maxUses,
			Status:    status,
			Note:      strings.TrimSpace(req.Note),
			ExpiresAt: expiresAt,
			CreatedBy: actor.ID,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if err := s.repo.CreateInviteCodes(items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) SetInviteCodeStatus(actor *model.User, id string, enabled bool) (*model.InviteCode, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	item, err := s.repo.InviteCode(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, BadAuthRequest("邀请码不存在或已删除")
		}
		return nil, err
	}
	if enabled {
		item.Status = "active"
	} else {
		item.Status = "disabled"
	}
	item.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveInviteCode(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteInviteCode(actor *model.User, id string) error {
	if err := s.RequireAdmin(actor); err != nil {
		return err
	}
	item, err := s.repo.InviteCode(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BadAuthRequest("邀请码不存在或已删除")
		}
		return err
	}
	return s.repo.DeleteInviteCode(item.ID)
}

func normalizeRegistrationMode(value string) string {
	switch strings.TrimSpace(value) {
	case RegistrationPolicyInvite:
		return RegistrationPolicyInvite
	case RegistrationPolicyClosed:
		return RegistrationPolicyClosed
	default:
		return RegistrationPolicyOpen
	}
}

func randomInviteCode(length int) string {
	if length <= 0 {
		length = 10
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		for i := range buf {
			buf[i] = byte(time.Now().UnixNano() >> (i % 8))
		}
	}
	out := make([]byte, length)
	for i, value := range buf {
		out[i] = inviteCodeAlphabet[int(value)%len(inviteCodeAlphabet)]
	}
	return string(out)
}
