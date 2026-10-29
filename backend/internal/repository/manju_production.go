package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) CreateManjuProductionRun(run *model.ManjuProductionRun) error {
	return r.db.Create(run).Error
}

func (r *Repository) CreateManjuProductionStages(stages []model.ManjuProductionStage) error {
	if len(stages) == 0 {
		return nil
	}
	return r.db.Create(&stages).Error
}

func (r *Repository) ManjuProductionRun(projectID, runID string) (*model.ManjuProductionRun, error) {
	var run model.ManjuProductionRun
	err := r.db.First(&run, "id = ? AND project_id = ?", runID, projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) ManjuProductionRuns(projectID string) ([]model.ManjuProductionRun, error) {
	var runs []model.ManjuProductionRun
	err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&runs).Error
	return runs, err
}

func (r *Repository) ManjuProductionStages(runID string) ([]model.ManjuProductionStage, error) {
	var stages []model.ManjuProductionStage
	err := r.db.Where("run_id = ?", runID).Order("position ASC").Find(&stages).Error
	return stages, err
}

func (r *Repository) ManjuProductionStage(runID, key string) (*model.ManjuProductionStage, error) {
	var stage model.ManjuProductionStage
	err := r.db.First(&stage, "run_id = ? AND key = ?", runID, key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stage, nil
}

func (r *Repository) ManjuProductionStageByPosition(runID string, position int) (*model.ManjuProductionStage, error) {
	var stage model.ManjuProductionStage
	err := r.db.First(&stage, "run_id = ? AND position = ?", runID, position).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stage, nil
}

func (r *Repository) SaveManjuProductionStage(stage *model.ManjuProductionStage) error {
	stage.UpdatedAt = time.Now()
	return r.db.Save(stage).Error
}

func (r *Repository) SaveManjuProductionRun(run *model.ManjuProductionRun) error {
	run.UpdatedAt = time.Now()
	return r.db.Save(run).Error
}

// ActivateManjuProductionRun makes the selected run the project's only ACTIVE
// production run. Older active siblings remain in storage as SUPERSEDED history.
func (r *Repository) ActivateManjuProductionRun(run *model.ManjuProductionRun) error {
	now := time.Now()
	if err := r.db.Model(&model.ManjuProductionRun{}).
		Where("project_id = ? AND id <> ? AND status = ?", run.ProjectID, run.ID, model.ManjuProductionRunStatusActive).
		Updates(map[string]any{
			"status":     model.ManjuProductionRunStatusSuperseded,
			"updated_at": now,
		}).Error; err != nil {
		return err
	}
	run.Status = model.ManjuProductionRunStatusActive
	run.UpdatedAt = now
	return r.db.Save(run).Error
}

func (r *Repository) CreateManjuProductionArtifact(artifact *model.ManjuProductionArtifact) error {
	return r.db.Create(artifact).Error
}

func (r *Repository) ManjuProductionArtifacts(runID string) ([]model.ManjuProductionArtifact, error) {
	var artifacts []model.ManjuProductionArtifact
	err := r.db.Where("run_id = ?", runID).Order("created_at ASC").Find(&artifacts).Error
	return artifacts, err
}

func (r *Repository) CreateManjuProductionImages(images []model.ManjuProductionImage) error {
	if len(images) == 0 {
		return nil
	}
	return r.db.Create(&images).Error
}

func (r *Repository) ManjuProductionImages(runID string) ([]model.ManjuProductionImage, error) {
	var images []model.ManjuProductionImage
	err := r.db.Where("run_id = ?", runID).Order("position ASC").Find(&images).Error
	return images, err
}

func (r *Repository) ManjuProductionImage(runID, imageID string) (*model.ManjuProductionImage, error) {
	var image model.ManjuProductionImage
	err := r.db.First(&image, "run_id = ? AND id = ?", runID, imageID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &image, nil
}

func (r *Repository) SaveManjuProductionImage(image *model.ManjuProductionImage) error {
	image.UpdatedAt = time.Now()
	return r.db.Save(image).Error
}

func (r *Repository) CreateManjuProductionVideos(videos []model.ManjuProductionVideo) error {
	if len(videos) == 0 {
		return nil
	}
	return r.db.Create(&videos).Error
}

func (r *Repository) ManjuProductionVideos(runID string) ([]model.ManjuProductionVideo, error) {
	var videos []model.ManjuProductionVideo
	err := r.db.Where("run_id = ?", runID).Order("position ASC").Find(&videos).Error
	return videos, err
}

func (r *Repository) ManjuProductionVideo(runID, videoID string) (*model.ManjuProductionVideo, error) {
	var video model.ManjuProductionVideo
	err := r.db.First(&video, "run_id = ? AND id = ?", runID, videoID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *Repository) SaveManjuProductionVideo(video *model.ManjuProductionVideo) error {
	video.UpdatedAt = time.Now()
	return r.db.Save(video).Error
}
