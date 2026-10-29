package model

import "time"

// InviteCode 是托管形态的邀请码。码本身只存明文用于校验与展示，不做密钥用途；
// 使用次数与状态在注册事务里原子扣减，避免超发。
type InviteCode struct {
	ID        string     `json:"id" gorm:"primaryKey;size:36"`
	Code      string     `json:"code" gorm:"size:64;uniqueIndex"`
	Batch     string     `json:"batch" gorm:"size:64;index"`
	MaxUses   int        `json:"maxUses"`
	UsedCount int        `json:"usedCount"`
	Status    string     `json:"status" gorm:"size:24;index"`
	Note      string     `json:"note" gorm:"size:255"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedBy string     `json:"createdBy" gorm:"size:36;index"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func (InviteCode) TableName() string {
	return "invite_codes"
}

// InviteCodeRedemption 记录一张邀请码被哪个账号使用，便于追溯和防重复。
type InviteCodeRedemption struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	InviteCodeID string    `json:"inviteCodeId" gorm:"size:36;index;uniqueIndex:idx_invite_redemption_code_user,priority:1"`
	UserID       string    `json:"userId" gorm:"size:36;index;uniqueIndex:idx_invite_redemption_code_user,priority:2"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (InviteCodeRedemption) TableName() string {
	return "invite_code_redemptions"
}
