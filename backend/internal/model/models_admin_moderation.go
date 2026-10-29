package model

import "time"

// AdminContentModeration is a hosted-only moderation decision. Keeping it in a
// separate table avoids overloading user-editable project/asset status fields
// and preserves the audit reason when an item is blocked or restored.
type AdminContentModeration struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	TargetType string    `json:"targetType" gorm:"size:24;uniqueIndex:idx_admin_moderation_target,priority:1"`
	TargetID   string    `json:"targetId" gorm:"size:80;uniqueIndex:idx_admin_moderation_target,priority:2"`
	Status     string    `json:"status" gorm:"size:24;index"`
	Reason     string    `json:"reason" gorm:"type:text"`
	OperatorID string    `json:"operatorId" gorm:"size:36;index"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (AdminContentModeration) TableName() string { return "admin_content_moderations" }

const (
	AdminModerationTargetCanvas = "canvas"
	AdminModerationTargetAsset  = "asset"

	AdminModerationStatusActive  = "active"
	AdminModerationStatusBlocked = "blocked"
)
