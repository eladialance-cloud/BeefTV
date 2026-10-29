package manju

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ProductionVideoStatusQueued  = model.ManjuProductionVideoStatusQueued
	ProductionVideoStatusRunning = model.ManjuProductionVideoStatusRunning
	ProductionVideoStatusReady   = model.ManjuProductionVideoStatusReady
	ProductionVideoStatusFailed  = model.ManjuProductionVideoStatusFailed
)

var (
	ErrProductionVideosNotReady = errors.New("manju production videos are not ready")
	ErrProductionVideoInvalid   = errors.New("invalid manju production video batch")
	ErrProductionVideoStarted   = errors.New("manju production video batch already started")
)

var productionVideoHeaderPattern = regexp.MustCompile(`^#{2,3}\s*(?:【投喂单元\s*[0-9]+\s*/\s*[0-9]+\s*·\s*(段\s*[0-9]+)】|(段\s*[0-9]+)(?:\s*[｜|:：].*)?)\s*$`)
var productionVideoDurationPattern = regexp.MustCompile(`【时长】\s*(\d+)\s*秒`)
var productionVideoTimePattern = regexp.MustCompile(`(\d{1,2}):(\d{2})(?:[.:](\d{1,2}))?\s*[-~～→]\s*(\d{1,2}):(\d{2})(?:[.:](\d{1,2}))?`)
var productionVideoSecondTimePattern = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*s?\s*[-~～→]\s*(\d+(?:\.\d+)?)\s*s?`)

type ProductionVideoView struct {
	ID              string    `json:"id"`
	Position        int       `json:"position"`
	SegmentCode     string    `json:"segmentCode"`
	DurationSeconds int       `json:"durationSeconds"`
	Attempt         int       `json:"attempt"`
	Status          string    `json:"status"`
	TaskID          string    `json:"taskId,omitempty"`
	ResourceID      string    `json:"resourceId,omitempty"`
	URL             string    `json:"url,omitempty"`
	MimeType        string    `json:"mimeType,omitempty"`
	Error           string    `json:"error,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type productionVideoPlan struct {
	ID              string
	Position        int
	SegmentCode     string
	DurationSeconds int
	Prompt          string
	GlobalStyle     string
	Attempt         int
	ReferenceImage  *model.ManjuProductionImage
}

type productionVideoSegment struct {
	Code            string
	DurationSeconds int
	Prompt          string
	GlobalStyle     string
}

// StartProductionVideos creates one real canvas_video task for every P4 feed
// segment. It never writes READY directly. READY is written only after task,
// generation output, resource ownership, media type, and ready-state checks.
func (s *Service) StartProductionVideos(actorID, projectID, runID string) (*ProductionRunDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !canEditProduction(access.Role) {
		return nil, ErrForbidden
	}
	if s.scriptTaskCreate == nil {
		return nil, ErrScriptTaskUnavailable
	}
	if err := s.reconcileProductionStageTasks(actorID, projectID, runID); err != nil {
		return nil, err
	}

	var plans []productionVideoPlan
	var providerContext struct {
		ChannelID string
		ModelKey  string
		Aspect    string
		AssetMode string
	}
	err = s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditProduction(access.Role) {
			return ErrForbidden
		}
		run, err := tx.ManjuProductionRun(projectID, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return ErrProductionRunNotFound
		}
		if run.Status != model.ManjuProductionRunStatusActive || run.CurrentStage != ProductionStageFeed {
			return ErrProductionStageNotReady
		}
		stage, err := tx.ManjuProductionStage(runID, ProductionStageFeed)
		if err != nil {
			return err
		}
		if stage == nil ||
			stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation ||
			strings.TrimSpace(stage.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		binding, err := tx.ManjuProjectModelBinding(projectID)
		if err != nil {
			return err
		}
		if binding == nil ||
			strings.TrimSpace(binding.VideoChannelID) == "" ||
			strings.TrimSpace(binding.VideoModelKey) == "" {
			return ErrInvalidModelBinding
		}
		providerContext.ChannelID = binding.VideoChannelID
		providerContext.ModelKey = binding.VideoModelKey

		receipt, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		var snapshot LockSnapshot
		if err := json.Unmarshal([]byte(receipt.PayloadJSON), &snapshot); err != nil {
			return ErrProductionVideoInvalid
		}
		providerContext.Aspect = strings.TrimSpace(snapshot.AspectRatio)
		if providerContext.Aspect == "" {
			return ErrProductionVideoInvalid
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil {
			return ErrProductionAssetNotReady
		}
		providerContext.AssetMode = assetRun.AssetMode

		content, err := productionArtifactContent(tx, runID, stage.ArtifactID)
		if err != nil {
			return err
		}
		segments, err := parseProductionVideoSegments(content)
		if err != nil || len(segments) == 0 {
			return ErrProductionVideoInvalid
		}
		images, err := tx.ManjuProductionImages(runID)
		if err != nil {
			return err
		}
		imageBySegment := make(map[string]*model.ManjuProductionImage, len(images))
		for index := range images {
			image := &images[index]
			imageBySegment[normalizeProductionSegmentCode(image.SegmentCode)] = image
		}
		segmentByCode := make(map[string]productionVideoSegment, len(segments))
		for _, segment := range segments {
			segmentByCode[normalizeProductionSegmentCode(segment.Code)] = segment
		}
		for _, segment := range segments {
			if segment.DurationSeconds <= 0 ||
				(snapshot.SingleSegmentMaxSeconds > 0 && segment.DurationSeconds > snapshot.SingleSegmentMaxSeconds) {
				return ErrProductionVideoInvalid
			}
			if assetRun.AssetMode == AssetModeM0Value {
				continue
			}
			segmentCode := normalizeProductionSegmentCode(segment.Code)
			image := imageBySegment[segmentCode]
			if image == nil || image.Status != ProductionImageStatusReady || strings.TrimSpace(image.ResourceID) == "" {
				delete(imageBySegment, segmentCode)
				continue
			}
			resource, err := tx.ResourceForUser(image.CreatedBy, image.ResourceID)
			if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil {
				delete(imageBySegment, segmentCode)
				continue
			}
			if err != nil {
				return err
			}
			if resource.Kind != "image" || resource.Status != model.ResourceStatusReady {
				delete(imageBySegment, segmentCode)
				continue
			}
		}

		existing, err := tx.ManjuProductionVideos(runID)
		if err != nil {
			return err
		}
		now := time.Now()
		if len(existing) > 0 {
			for index := range existing {
				video := &existing[index]
				if video.Status == ProductionVideoStatusQueued ||
					video.Status == ProductionVideoStatusRunning ||
					video.Status == ProductionVideoStatusReady {
					continue
				}
				video.Attempt++
				video.Status = ProductionVideoStatusQueued
				video.TaskID = ""
				video.ResourceID = ""
				video.URL = ""
				video.MimeType = ""
				video.Error = ""
				video.UpdatedAt = now
				if err := tx.SaveManjuProductionVideo(video); err != nil {
					return err
				}
				plans = append(plans, productionVideoPlan{
					ID: video.ID, Position: video.Position, SegmentCode: video.SegmentCode,
					DurationSeconds: video.DurationSeconds, Prompt: video.Prompt,
					GlobalStyle:    segmentByCode[normalizeProductionSegmentCode(video.SegmentCode)].GlobalStyle,
					Attempt:        video.Attempt,
					ReferenceImage: imageBySegment[normalizeProductionSegmentCode(video.SegmentCode)],
				})
			}
			if len(plans) == 0 {
				return ErrProductionVideoStarted
			}
			return nil
		}

		videos := make([]model.ManjuProductionVideo, 0, len(segments))
		for index, segment := range segments {
			video := model.ManjuProductionVideo{
				ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
				Position: index + 1, SegmentCode: segment.Code, DurationSeconds: segment.DurationSeconds,
				Prompt: segment.Prompt, Attempt: 1, Status: ProductionVideoStatusQueued,
				CreatedBy: actorID, CreatedAt: now, UpdatedAt: now,
			}
			videos = append(videos, video)
			plans = append(plans, productionVideoPlan{
				ID: video.ID, Position: video.Position, SegmentCode: video.SegmentCode,
				DurationSeconds: video.DurationSeconds, Prompt: video.Prompt,
				GlobalStyle: segment.GlobalStyle, Attempt: video.Attempt,
				ReferenceImage: imageBySegment[normalizeProductionSegmentCode(video.SegmentCode)],
			})
		}
		return tx.CreateManjuProductionVideos(videos)
	})
	if err != nil {
		return nil, err
	}

	for _, plan := range plans {
		executionPrompt := composeProductionVideoPrompt(plan.GlobalStyle, plan.Prompt)
		input := map[string]any{
			"mode":   "video",
			"prompt": executionPrompt,
			"config": map[string]any{
				"channelId":    providerContext.ChannelID,
				"model":        providerContext.ModelKey,
				"size":         providerContext.Aspect,
				"videoSeconds": strconv.Itoa(plan.DurationSeconds),
			},
			"metadata": map[string]any{
				"clientOperationId":         fmt.Sprintf("manju:production:%s:video:%s:%d", runID, plan.ID, plan.Attempt),
				"manjuProjectId":            projectID,
				"manjuProductionRunId":      runID,
				"manjuProductionStageKey":   ProductionStageFeed,
				"manjuProductionVideoJobId": plan.ID,
				"manjuProductionSegment":    plan.SegmentCode,
			},
		}
		if providerContext.AssetMode != AssetModeM0Value && plan.ReferenceImage != nil && strings.TrimSpace(plan.ReferenceImage.ResourceID) != "" {
			input["referenceImages"] = []any{
				map[string]any{"storageKey": "resource:" + plan.ReferenceImage.ResourceID},
			}
		}
		request := localtask.CreateRequest{
			ProjectID: projectID,
			Type:      "canvas_video",
			Operation: "manju_production_video",
			Prompt:    executionPrompt,
			Model:     providerContext.ModelKey,
			Input:     input,
		}
		task, createErr := s.scriptTaskCreate(actorID, request)
		if createErr == nil && (task == nil || strings.TrimSpace(task.ID) == "") {
			createErr = errors.New("视频任务服务没有返回任务 ID")
		}
		if err := s.repo.Transaction(func(tx *repository.Repository) error {
			video, err := tx.ManjuProductionVideo(runID, plan.ID)
			if err != nil || video == nil {
				return err
			}
			if createErr != nil {
				video.Status = ProductionVideoStatusFailed
				video.Error = strings.TrimSpace(createErr.Error())
			} else {
				video.Status = ProductionVideoStatusRunning
				video.TaskID = task.ID
				video.Error = ""
			}
			return tx.SaveManjuProductionVideo(video)
		}); err != nil {
			return nil, err
		}
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

func parseProductionVideoSegments(content string) ([]productionVideoSegment, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	result := make([]productionVideoSegment, 0)
	seen := make(map[string]bool)
	globalStyle := productionFeedGlobalStyle(strings.Join(lines, "\n"))
	for index := 0; index < len(lines); index++ {
		match := productionVideoHeaderPattern.FindStringSubmatch(strings.TrimSpace(lines[index]))
		if len(match) != 3 {
			continue
		}
		code := strings.TrimSpace(match[1])
		if code == "" {
			code = strings.TrimSpace(match[2])
		}
		key := normalizeProductionSegmentCode(code)
		if seen[key] {
			return nil, ErrProductionVideoInvalid
		}
		seen[key] = true
		start := index + 1
		end := start
		for end < len(lines) {
			next := strings.TrimSpace(lines[end])
			if strings.HasPrefix(next, "## ") {
				break
			}
			end++
		}
		block := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
		prompt := productionVideoPromptBody(block)
		duration := productionVideoDurationSeconds(block)
		if duration <= 0 {
			return nil, ErrProductionVideoInvalid
		}
		result = append(result, productionVideoSegment{Code: code, DurationSeconds: duration, Prompt: prompt, GlobalStyle: globalStyle})
		index = end - 1
	}
	if len(result) == 0 {
		return nil, ErrProductionVideoInvalid
	}
	return result, nil
}

func productionFeedGlobalStyle(content string) string {
	contract := mustLoadP4FeedContract()
	if contract.StyleLock.InjectPosition == "each-unit-text-block" {
		return ""
	}
	styleMarker := contract.StyleLock.StyleMarker
	performanceMarker := contract.StyleLock.PerformanceMarker
	styleStart := strings.Index(content, styleMarker)
	if styleStart < 0 {
		return ""
	}
	searchFrom := styleStart + len(styleMarker)
	performanceOffset := strings.Index(content[searchFrom:], performanceMarker)
	if performanceOffset >= 0 {
		searchFrom += performanceOffset + len(performanceMarker)
	}
	end := len(content)
	for _, marker := range []string{"\n【", "\n```", "\n## "} {
		if offset := strings.Index(content[searchFrom:], marker); offset >= 0 {
			candidate := searchFrom + offset
			if candidate < end {
				end = candidate
			}
		}
	}
	return strings.TrimSpace(content[styleStart:end])
}

func composeProductionVideoPrompt(globalStyle, segmentPrompt string) string {
	globalStyle = strings.TrimSpace(globalStyle)
	segmentPrompt = strings.TrimSpace(segmentPrompt)
	if globalStyle == "" {
		return segmentPrompt
	}
	if segmentPrompt == "" {
		return globalStyle
	}
	return globalStyle + "\n\n" + segmentPrompt
}

func productionVideoDurationSeconds(block string) int {
	if match := productionVideoDurationPattern.FindStringSubmatch(block); len(match) == 2 {
		if value, err := strconv.Atoi(match[1]); err == nil && value > 0 {
			return value
		}
	}
	start, end := math.MaxFloat64, 0.0
	for _, match := range productionVideoTimePattern.FindAllStringSubmatch(block, -1) {
		startSecond := float64(mustAtoi(match[1])*60+mustAtoi(match[2])) + productionVideoFractionSeconds(match[3])
		endSecond := float64(mustAtoi(match[4])*60+mustAtoi(match[5])) + productionVideoFractionSeconds(match[6])
		if startSecond < start {
			start = startSecond
		}
		if endSecond > end {
			end = endSecond
		}
	}
	if start != math.MaxFloat64 && end > start {
		return int(math.Round(end - start))
	}
	for _, match := range productionVideoSecondTimePattern.FindAllStringSubmatch(block, -1) {
		startSecond, startErr := strconv.ParseFloat(match[1], 64)
		endSecond, endErr := strconv.ParseFloat(match[2], 64)
		if startErr == nil && endErr == nil && endSecond > startSecond {
			return int(math.Round(endSecond - startSecond))
		}
	}
	return 0
}

func productionVideoFractionSeconds(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseFloat("0."+value, 64)
	return parsed
}

func mustAtoi(value string) int {
	result, _ := strconv.Atoi(value)
	return result
}

func productionVideoPromptBody(block string) string {
	for _, marker := range []string{"```text", "```prompt", "```"} {
		if start := strings.Index(block, marker); start >= 0 {
			body := block[start+len(marker):]
			if newline := strings.IndexByte(body, '\n'); newline >= 0 {
				body = body[newline+1:]
			}
			if end := strings.Index(body, "```"); end >= 0 {
				return strings.TrimSpace(body[:end])
			}
		}
	}
	return strings.TrimSpace(block)
}

func normalizeProductionSegmentCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.TrimPrefix(value, "段")
	value = strings.TrimPrefix(value, "SEG")
	value = strings.TrimPrefix(value, "#")
	value = strings.TrimLeft(value, "0")
	if value == "" {
		return "0"
	}
	return value
}

func reconcileProductionVideos(tx *repository.Repository, runID string) error {
	videos, err := tx.ManjuProductionVideos(runID)
	if err != nil {
		return err
	}
	for index := range videos {
		video := &videos[index]
		if video.Status == ProductionVideoStatusReady ||
			video.Status == ProductionVideoStatusFailed ||
			strings.TrimSpace(video.TaskID) == "" {
			continue
		}
		task, err := tx.TaskForUser(video.CreatedBy, video.TaskID)
		if errors.Is(err, gorm.ErrRecordNotFound) || task == nil {
			video.Status = ProductionVideoStatusFailed
			video.Error = "视频任务不存在"
			if err := tx.SaveManjuProductionVideo(video); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if task.ProjectID != video.ProjectID {
			video.Status = ProductionVideoStatusFailed
			video.Error = "视频任务不属于当前项目"
			if err := tx.SaveManjuProductionVideo(video); err != nil {
				return err
			}
			continue
		}
		if task.Type != "canvas_video" {
			video.Status = ProductionVideoStatusFailed
			video.Error = "视频任务类型不是 canvas_video"
			if err := tx.SaveManjuProductionVideo(video); err != nil {
				return err
			}
			continue
		}
		switch task.Status {
		case model.TaskStatusQueued, model.TaskStatusRunning:
			video.Status = ProductionVideoStatusRunning
			video.Error = ""
			if err := tx.SaveManjuProductionVideo(video); err != nil {
				return err
			}
		case model.TaskStatusFailed, model.TaskStatusCancelled:
			video.Status = ProductionVideoStatusFailed
			video.Error = strings.TrimSpace(task.Error)
			if video.Error == "" {
				video.Error = "视频任务未成功完成"
			}
			if err := tx.SaveManjuProductionVideo(video); err != nil {
				return err
			}
		case model.TaskStatusSucceeded:
			results, err := tx.GenerationOutputResults(task.ID)
			if err != nil {
				return err
			}
			ownedResults := make([]model.Result, 0, len(results))
			for _, result := range results {
				if result.UserID == video.CreatedBy {
					ownedResults = append(ownedResults, result)
				}
			}
			outputs, err := localtask.DecodeOutputResults(ownedResults)
			if err != nil {
				video.Status = ProductionVideoStatusFailed
				video.Error = "视频交付记录无法读取"
				if err := tx.SaveManjuProductionVideo(video); err != nil {
					return err
				}
				continue
			}
			var selected *localtask.CanonicalOutput
			for outputIndex := range outputs {
				if outputs[outputIndex].MediaType == "video" && strings.TrimSpace(outputs[outputIndex].ResourceID) != "" {
					selected = &outputs[outputIndex]
					break
				}
			}
			if selected == nil {
				video.Status = ProductionVideoStatusFailed
				video.Error = "视频任务成功但没有可交付的真实视频资源"
				if err := tx.SaveManjuProductionVideo(video); err != nil {
					return err
				}
				continue
			}
			resource, err := tx.ResourceForUser(task.UserID, selected.ResourceID)
			if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil {
				video.Status = ProductionVideoStatusFailed
				video.Error = "视频资源不存在或不属于任务账号"
				if err := tx.SaveManjuProductionVideo(video); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if resource.Kind != "video" || resource.Status != model.ResourceStatusReady {
				video.Status = ProductionVideoStatusRunning
				video.Error = "视频资源尚未就绪"
				if err := tx.SaveManjuProductionVideo(video); err != nil {
					return err
				}
				continue
			}
			video.Status = ProductionVideoStatusReady
			video.ResourceID = resource.ID
			video.URL = resource.PublicURL
			video.MimeType = resource.MimeType
			video.Error = ""
			if err := tx.SaveManjuProductionVideo(video); err != nil {
				return err
			}
		}
	}
	return nil
}

func productionVideosReady(tx *repository.Repository, runID string) error {
	videos, err := tx.ManjuProductionVideos(runID)
	if err != nil {
		return err
	}
	if len(videos) == 0 {
		return ErrProductionVideosNotReady
	}
	for _, video := range videos {
		if video.Status != ProductionVideoStatusReady || strings.TrimSpace(video.ResourceID) == "" {
			return ErrProductionVideosNotReady
		}
		resource, err := tx.ResourceForUser(video.CreatedBy, video.ResourceID)
		if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil ||
			resource.Status != model.ResourceStatusReady || resource.Kind != "video" {
			return ErrProductionVideosNotReady
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func productionVideoView(video model.ManjuProductionVideo) ProductionVideoView {
	return ProductionVideoView{
		ID: video.ID, Position: video.Position, SegmentCode: video.SegmentCode,
		DurationSeconds: video.DurationSeconds, Attempt: video.Attempt, Status: video.Status,
		TaskID: video.TaskID, ResourceID: video.ResourceID, URL: video.URL,
		MimeType: video.MimeType, Error: video.Error, CreatedAt: video.CreatedAt, UpdatedAt: video.UpdatedAt,
	}
}
