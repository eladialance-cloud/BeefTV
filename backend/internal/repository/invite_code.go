package repository

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	InviteCodeStatusActive   = "active"
	InviteCodeStatusDisabled = "disabled"
)

var (
	ErrInviteCodeRequired    = errors.New("请输入邀请码")
	ErrInviteCodeNotFound    = errors.New("邀请码不存在")
	ErrInviteCodeDisabled    = errors.New("邀请码已停用")
	ErrInviteCodeExpired     = errors.New("邀请码已过期")
	ErrInviteCodeExhausted   = errors.New("邀请码使用次数已用完")
	ErrInviteCodeAlreadyUsed = errors.New("该邀请码已被当前账号使用")
)

func newInviteRedemptionID() string {
	return kernel.NewID()
}

// AdminInviteCodes 列出邀请码，供后台管理页使用。
func (r *Repository) AdminInviteCodes(keyword string, status string, limit int, offset int) ([]model.InviteCode, int64, error) {
	query := r.db.Model(&model.InviteCode{})
	if value := strings.TrimSpace(keyword); value != "" {
		like := "%" + value + "%"
		query = query.Where("code LIKE ? OR batch LIKE ? OR note LIKE ?", like, like, like)
	}
	switch strings.TrimSpace(status) {
	case "active":
		query = query.Where("status = ?", InviteCodeStatusActive)
	case "disabled":
		query = query.Where("status = ?", InviteCodeStatusDisabled)
	case "expired":
		query = query.Where("expires_at IS NOT NULL AND expires_at < ?", time.Now().UTC())
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.InviteCode
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) InviteCode(id string) (*model.InviteCode, error) {
	var item model.InviteCode
	if err := r.db.First(&item, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateInviteCode(item *model.InviteCode) error {
	return r.db.Create(item).Error
}

// CreateInviteCodes 批量落库，整批失败一起回滚。
func (r *Repository) CreateInviteCodes(items []model.InviteCode) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SaveInviteCode(item *model.InviteCode) error {
	return r.db.Save(item).Error
}

// ConsumeInviteCode 在事务里完成“校验 + 扣次数 + 写核销记录 + 执行注册”。
// 任何一步失败都整体回滚，邀请码不会被白扣。
func (r *Repository) ConsumeInviteCode(code string, user *model.User, identity *model.UserIdentity) error {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ErrInviteCodeRequired
	}
	if user == nil || identity == nil {
		return errors.New("注册账号参数不完整")
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var item model.InviteCode
		if err := tx.First(&item, "code = ?", trimmed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteCodeNotFound
			}
			return err
		}
		if item.Status != InviteCodeStatusActive {
			return ErrInviteCodeDisabled
		}
		now := time.Now().UTC()
		if item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
			return ErrInviteCodeExpired
		}
		var existing int64
		if err := tx.Model(&model.InviteCodeRedemption{}).
			Where("invite_code_id = ? AND user_id = ?", item.ID, user.ID).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrInviteCodeAlreadyUsed
		}
		// 条件更新让并发注册也只能在剩余次数内成功；回滚时余量自动恢复。
		result := tx.Model(&model.InviteCode{}).
			Where("id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?) AND (max_uses <= 0 OR used_count < max_uses)", item.ID, InviteCodeStatusActive, now).
			Update("used_count", gorm.Expr("used_count + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInviteCodeExhausted
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if err := tx.Create(identity).Error; err != nil {
			return err
		}
		redemption := &model.InviteCodeRedemption{
			ID:           newInviteRedemptionID(),
			InviteCodeID: item.ID,
			UserID:       user.ID,
			CreatedAt:    now,
		}
		return tx.Create(redemption).Error
	})
}
