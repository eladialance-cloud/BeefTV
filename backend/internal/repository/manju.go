package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ManjuProjectAccess struct {
	Project model.Project
	Role    string
}

func (r *Repository) ManjuProjectsForUser(userID string) ([]ManjuProjectAccess, error) {
	var memberships []model.ManjuProjectMember
	if err := r.db.
		Where("user_id = ? AND status = ?", userID, model.ManjuMemberStatusActive).
		Find(&memberships).Error; err != nil {
		return nil, err
	}
	memberRoles := make(map[string]string, len(memberships))
	memberProjectIDs := make([]string, 0, len(memberships))
	for _, membership := range memberships {
		memberRoles[membership.ProjectID] = membership.Role
		memberProjectIDs = append(memberProjectIDs, membership.ProjectID)
	}

	var projects []model.Project
	query := r.db.Where("user_id = ? AND type = ?", userID, "manju")
	if len(memberProjectIDs) > 0 {
		query = r.db.Where("(user_id = ? OR id IN ?) AND type = ?", userID, memberProjectIDs, "manju")
	}
	if err := query.Order("updated_at DESC").Find(&projects).Error; err != nil {
		return nil, err
	}
	result := make([]ManjuProjectAccess, 0, len(projects))
	for _, project := range projects {
		role := memberRoles[project.ID]
		if project.UserID == userID {
			role = model.ManjuMemberRoleOwner
		}
		if role != "" {
			result = append(result, ManjuProjectAccess{Project: project, Role: role})
		}
	}
	return result, nil
}

// ManjuProjectForUser deliberately returns gorm.ErrRecordNotFound for both a
// missing project and a project the user cannot access, so callers cannot use
// the response to enumerate other users' project IDs.
func (r *Repository) ManjuProjectForUser(userID, projectID string) (*ManjuProjectAccess, error) {
	var project model.Project
	if err := r.db.First(&project, "id = ? AND type = ?", projectID, "manju").Error; err != nil {
		return nil, err
	}
	if project.UserID == userID {
		return &ManjuProjectAccess{Project: project, Role: model.ManjuMemberRoleOwner}, nil
	}
	var member model.ManjuProjectMember
	if err := r.db.First(&member, "project_id = ? AND user_id = ? AND status = ?", projectID, userID, model.ManjuMemberStatusActive).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &ManjuProjectAccess{Project: project, Role: member.Role}, nil
}

func (r *Repository) LockManjuProject(projectID string) error {
	var project model.Project
	return r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, "id = ? AND type = ?", projectID, "manju").Error
}

func (r *Repository) HasManjuProjectActiveTasks(projectID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.Task{}).
		Where("project_id = ? AND status IN ?", projectID, []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning}).
		Count(&count).Error
	return count > 0, err
}

// DeleteManjuProjectRecords removes the project-owned P0-P5 records while
// leaving Task, Result and Resource history intact for billing and audit.
func (r *Repository) DeleteManjuProjectRecords(projectID string) error {
	productionRunIDs, err := manjuRunIDs(r.db, &model.ManjuProductionRun{}, projectID)
	if err != nil {
		return err
	}
	if len(productionRunIDs) > 0 {
		if err := r.db.Where("run_id IN ?", productionRunIDs).Delete(&model.ManjuProductionStage{}).Error; err != nil {
			return err
		}
	}
	for _, modelValue := range []any{
		&model.ManjuDeliverable{},
		&model.ManjuProductionPrevis{},
		&model.ManjuProductionImage{},
		&model.ManjuProductionVideo{},
		&model.ManjuProductionArtifact{},
		&model.ManjuProductionRun{},
	} {
		if err := r.db.Where("project_id = ?", projectID).Delete(modelValue).Error; err != nil {
			return err
		}
	}

	assetRunIDs, err := manjuRunIDs(r.db, &model.ManjuAssetRun{}, projectID)
	if err != nil {
		return err
	}
	if len(assetRunIDs) > 0 {
		if err := r.db.Where("run_id IN ?", assetRunIDs).Delete(&model.ManjuAssetStage{}).Error; err != nil {
			return err
		}
	}
	for _, modelValue := range []any{
		&model.ManjuAssetReference{},
		&model.ManjuAssetArtifact{},
		&model.ManjuAssetRun{},
	} {
		if err := r.db.Where("project_id = ?", projectID).Delete(modelValue).Error; err != nil {
			return err
		}
	}

	scriptRunIDs, err := manjuRunIDs(r.db, &model.ManjuScriptRun{}, projectID)
	if err != nil {
		return err
	}
	if len(scriptRunIDs) > 0 {
		if err := r.db.Where("run_id IN ?", scriptRunIDs).Delete(&model.ManjuScriptStage{}).Error; err != nil {
			return err
		}
	}
	for _, modelValue := range []any{
		&model.ManjuScriptArtifact{},
		&model.ManjuScriptSource{},
		&model.ManjuScriptRun{},
	} {
		if err := r.db.Where("project_id = ?", projectID).Delete(modelValue).Error; err != nil {
			return err
		}
	}

	for _, modelValue := range []any{
		&model.ManjuArtifactLedger{},
		&model.ManjuModelExchange{},
		&model.ManjuRuleReadReceipt{},
		&model.ManjuRuleSnapshot{},
		&model.ManjuSourceChapter{},
		&model.ManjuSourceDocument{},
		&model.ManjuProjectModelBinding{},
		&model.ManjuLockReceipt{},
		&model.ManjuProjectMember{},
		&model.ManjuProjectStateSnapshot{},
	} {
		if err := r.db.Where("project_id = ?", projectID).Delete(modelValue).Error; err != nil {
			return err
		}
	}

	result := r.db.Delete(&model.Project{}, "id = ? AND type = ?", projectID, "manju")
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func manjuRunIDs(db *gorm.DB, modelValue any, projectID string) ([]string, error) {
	var ids []string
	err := db.Model(modelValue).Where("project_id = ?", projectID).Pluck("id", &ids).Error
	return ids, err
}

func (r *Repository) UpsertManjuProjectMember(member *model.ManjuProjectMember) error {
	now := time.Now()
	member.CreatedAt = now
	member.UpdatedAt = now
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"role":       member.Role,
			"status":     member.Status,
			"invited_by": member.InvitedBy,
			"updated_at": now,
		}),
	}).Create(member).Error; err != nil {
		return err
	}
	stored, err := r.ManjuProjectMemberForUser(member.ProjectID, member.UserID)
	if err != nil {
		return err
	}
	*member = *stored
	return nil
}

func (r *Repository) ManjuProjectMemberForUser(projectID, userID string) (*model.ManjuProjectMember, error) {
	var member model.ManjuProjectMember
	if err := r.db.First(&member, "project_id = ? AND user_id = ?", projectID, userID).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *Repository) ManjuProjectMembers(projectID string) ([]model.ManjuProjectMember, error) {
	var members []model.ManjuProjectMember
	err := r.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&members).Error
	return members, err
}

func (r *Repository) DeleteManjuProjectMember(projectID, memberID string) error {
	result := r.db.Where("id = ? AND project_id = ?", memberID, projectID).Delete(&model.ManjuProjectMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) LatestManjuLockReceipt(projectID string) (*model.ManjuLockReceipt, error) {
	var receipt model.ManjuLockReceipt
	err := r.db.Where("project_id = ?", projectID).Order("revision DESC").First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (r *Repository) SupersedeManjuLockReceipts(projectID string) error {
	return r.db.Model(&model.ManjuLockReceipt{}).
		Where("project_id = ? AND status = ?", projectID, model.ManjuLockStatusLocked).
		Update("status", model.ManjuLockStatusSuperseded).Error
}

func (r *Repository) CreateManjuLockReceipt(receipt *model.ManjuLockReceipt) error {
	return r.db.Create(receipt).Error
}

func (r *Repository) ManjuProjectModelBinding(projectID string) (*model.ManjuProjectModelBinding, error) {
	var binding model.ManjuProjectModelBinding
	err := r.db.First(&binding, "project_id = ?", projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *Repository) UpsertManjuProjectModelBinding(binding *model.ManjuProjectModelBinding) error {
	now := time.Now()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"text_channel_id":  binding.TextChannelID,
			"text_model_key":   binding.TextModelKey,
			"image_channel_id": binding.ImageChannelID,
			"image_model_key":  binding.ImageModelKey,
			"video_channel_id": binding.VideoChannelID,
			"video_model_key":  binding.VideoModelKey,
			"updated_by":       binding.UpdatedBy,
			"updated_at":       now,
		}),
	}).Create(binding).Error; err != nil {
		return err
	}
	stored, err := r.ManjuProjectModelBinding(binding.ProjectID)
	if err != nil {
		return err
	}
	if stored != nil {
		*binding = *stored
	}
	return nil
}

func (r *Repository) ManjuModelEnabled(channelID, modelKey, capability string) (bool, error) {
	var count int64
	err := r.db.Model(&model.ChannelModel{}).
		Joins("JOIN model_channels ON model_channels.id = channel_models.channel_id").
		Where("channel_models.channel_id = ? AND channel_models.model_key = ? AND channel_models.capability = ?", channelID, modelKey, capability).
		Where("channel_models.enabled = ? AND channel_models.deleted_at IS NULL", true).
		Where("model_channels.scope = ? AND model_channels.enabled = ? AND model_channels.deleted_at IS NULL", model.ChannelScopeSystem, true).
		Count(&count).Error
	return count == 1, err
}

func (r *Repository) CreateManjuScriptRun(run *model.ManjuScriptRun) error {
	return r.db.Create(run).Error
}

func (r *Repository) CreateManjuSourceDocument(document *model.ManjuSourceDocument, chapters []model.ManjuSourceChapter) error {
	if err := r.db.Create(document).Error; err != nil {
		return err
	}
	if len(chapters) == 0 {
		return nil
	}
	return r.db.Create(&chapters).Error
}

func (r *Repository) ManjuSourceDocuments(projectID string) ([]model.ManjuSourceDocument, error) {
	var documents []model.ManjuSourceDocument
	err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&documents).Error
	return documents, err
}

func (r *Repository) ManjuSourceDocument(projectID, documentID string) (*model.ManjuSourceDocument, error) {
	var document model.ManjuSourceDocument
	err := r.db.First(&document, "id = ? AND project_id = ?", documentID, projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &document, nil
}

func (r *Repository) ManjuSourceChapters(documentID string) ([]model.ManjuSourceChapter, error) {
	var chapters []model.ManjuSourceChapter
	err := r.db.Where("document_id = ?", documentID).Order("position ASC").Find(&chapters).Error
	return chapters, err
}

func (r *Repository) ManjuSourceChaptersByIDs(projectID, documentID string, chapterIDs []string) ([]model.ManjuSourceChapter, error) {
	if len(chapterIDs) == 0 {
		return nil, nil
	}
	var chapters []model.ManjuSourceChapter
	err := r.db.Where("project_id = ? AND document_id = ? AND id IN ?", projectID, documentID, chapterIDs).
		Order("position ASC").Find(&chapters).Error
	return chapters, err
}

func (r *Repository) ManjuAllSourceChapters(projectID string) ([]model.ManjuSourceChapter, error) {
	var chapters []model.ManjuSourceChapter
	err := r.db.Where("project_id = ?", projectID).Order("document_id ASC, position ASC").Find(&chapters).Error
	return chapters, err
}

func (r *Repository) CreateManjuScriptStages(stages []model.ManjuScriptStage) error {
	if len(stages) == 0 {
		return nil
	}
	return r.db.Create(&stages).Error
}

func (r *Repository) ManjuScriptRun(projectID, runID string) (*model.ManjuScriptRun, error) {
	var run model.ManjuScriptRun
	err := r.db.First(&run, "id = ? AND project_id = ?", runID, projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) ManjuScriptRuns(projectID string) ([]model.ManjuScriptRun, error) {
	var runs []model.ManjuScriptRun
	err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&runs).Error
	return runs, err
}

func (r *Repository) ManjuScriptStages(runID string) ([]model.ManjuScriptStage, error) {
	var stages []model.ManjuScriptStage
	err := r.db.Where("run_id = ?", runID).Order("position ASC").Find(&stages).Error
	return stages, err
}

func (r *Repository) ManjuScriptStage(runID, key string) (*model.ManjuScriptStage, error) {
	var stage model.ManjuScriptStage
	err := r.db.First(&stage, "run_id = ? AND key = ?", runID, key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stage, nil
}

func (r *Repository) SaveManjuScriptStage(stage *model.ManjuScriptStage) error {
	stage.UpdatedAt = time.Now()
	return r.db.Save(stage).Error
}

func (r *Repository) SaveManjuScriptRun(run *model.ManjuScriptRun) error {
	run.UpdatedAt = time.Now()
	return r.db.Save(run).Error
}

func (r *Repository) ManjuScriptSource(runID string) (*model.ManjuScriptSource, error) {
	var source model.ManjuScriptSource
	err := r.db.First(&source, "run_id = ?", runID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *Repository) SaveManjuScriptSource(source *model.ManjuScriptSource) error {
	return r.db.Save(source).Error
}

func (r *Repository) CreateManjuScriptArtifact(artifact *model.ManjuScriptArtifact) error {
	return r.db.Create(artifact).Error
}

func (r *Repository) ManjuScriptArtifacts(runID string) ([]model.ManjuScriptArtifact, error) {
	var artifacts []model.ManjuScriptArtifact
	err := r.db.Where("run_id = ?", runID).Order("created_at ASC").Find(&artifacts).Error
	return artifacts, err
}
