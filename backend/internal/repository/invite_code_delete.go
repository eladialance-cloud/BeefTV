package repository

import (
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// DeleteInviteCode 真删除；核销记录保留用于追溯。
func (r *Repository) DeleteInviteCode(id string) error {
	result := r.db.Delete(&model.InviteCode{}, "id = ?", strings.TrimSpace(id))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
