package repository

import (
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) excludeAdminModerated(query *gorm.DB, targetType string, targetColumn string) *gorm.DB {
	if r == nil || r.db == nil || !r.db.Migrator().HasTable(&model.AdminContentModeration{}) {
		return query
	}
	return query.Where(
		"NOT EXISTS (SELECT 1 FROM admin_content_moderations m WHERE m.target_type = ? AND m.target_id = "+targetColumn+" AND m.status = ?)",
		targetType, model.AdminModerationStatusBlocked,
	)
}

func (r *Repository) UpsertAdminModeration(item *model.AdminContentModeration) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "target_type"}, {Name: "target_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "reason", "operator_id", "updated_at",
		}),
	}).Create(item).Error
}

func (r *Repository) AdminModeration(targetType string, targetID string) (*model.AdminContentModeration, bool, error) {
	var item model.AdminContentModeration
	result := r.db.Where("target_type = ? AND target_id = ?", targetType, targetID).Limit(1).Find(&item)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &item, true, nil
}

func (r *Repository) AdminModerationStatus(targetType string, targetID string) (string, error) {
	if !r.db.Migrator().HasTable(&model.AdminContentModeration{}) {
		return model.AdminModerationStatusActive, nil
	}
	item, found, err := r.AdminModeration(targetType, targetID)
	if err != nil {
		return "", err
	}
	if !found || item.Status == "" {
		return model.AdminModerationStatusActive, nil
	}
	return item.Status, nil
}

func (r *Repository) AdminSetModeration(targetType string, targetID string, status string, reason string, operatorID string, now time.Time) error {
	item := &model.AdminContentModeration{
		ID:         targetType + ":" + targetID,
		TargetType: targetType,
		TargetID:   targetID,
		Status:     status,
		Reason:     reason,
		OperatorID: operatorID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return r.UpsertAdminModeration(item)
}

func (r *Repository) AdminAssetExists(id string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.Asset{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) AdminCanvasExists(id string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.CanvasProject{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
