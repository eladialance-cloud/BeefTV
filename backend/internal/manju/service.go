package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNotFound             = errors.New("manju project not found")
	ErrForbidden            = errors.New("manju project access denied")
	ErrInvalidLock          = errors.New("invalid manju P0 lock")
	ErrExplicitLockRequired = errors.New("explicit manju P0 lock selection required")
	ErrSeamRequired         = errors.New("manju D segment seam is required")
	ErrInvalidMember        = errors.New("invalid manju project member")
	ErrMemberNotFound       = errors.New("manju project member not found")
	ErrInvalidModelBinding  = errors.New("invalid manju model binding")
	ErrHasActiveTasks       = errors.New("manju project has active tasks")
)

const (
	LockTrack        = "track"
	LockSource       = "source"
	LockVideoModel   = "videoModel"
	LockAspectRatio  = "aspectRatio"
	LockDeliveryMode = "deliveryMode"
	LockStyleCard    = "styleCard"
	LockSeamMode     = "seamMode"

	TrackDomestic  = "domestic"
	TrackOverseas  = "overseas"
	TrackExplainer = "explainer"
	TrackOther     = "other"

	SourceOriginal   = "original"
	SourceAdaptation = "adaptation"
	SourceBenchmark  = "benchmark"
	SourceTrend      = "trend"
	SourcePolish     = "polish"

	VideoModelSeedance25 = "seedance-2.5"
	VideoModelSeedance20 = "seedance-2.0"
	VideoModelMiniMaxH3  = "minimax-h3"

	Aspect16x9 = "16:9"
	Aspect9x16 = "9:16"
	Aspect21x9 = "21:9"

	DeliveryBoard    = "board"
	DeliveryMarkdown = "markdown"
	DeliveryBoth     = "both"

	SeamTailFrameFreeze = "tail-frame-freeze"
	SeamHardCut         = "hard-cut"
	SeamInsertShot      = "insert-shot"
	SeamCreative        = "creative-transition"

	StyleBaseLiveAction  = "live-action"
	StyleBase2DAnimation = "2d-animation"
	StyleBase3DCG        = "3d-cg"
	StyleBaseInk2D       = "ink-2d"

	AssetModeM0 = "M0"
	AssetModeM1 = "M1"
	AssetModeM2 = "M2"
)

var (
	trackValues  = valueSet(TrackDomestic, TrackOverseas, TrackExplainer, TrackOther)
	sourceValues = valueSet(SourceOriginal, SourceAdaptation, SourceBenchmark, SourceTrend, SourcePolish)
	modelLimits  = map[string]int{
		VideoModelSeedance25: 30,
		VideoModelSeedance20: 15,
		VideoModelMiniMaxH3:  15,
	}
	aspectValues    = valueSet(Aspect16x9, Aspect9x16, Aspect21x9)
	deliveryValues  = valueSet(DeliveryBoard, DeliveryMarkdown, DeliveryBoth)
	seamValues      = valueSet(SeamTailFrameFreeze, SeamHardCut, SeamInsertShot, SeamCreative)
	styleBaseValues = valueSet(StyleBaseLiveAction, StyleBase2DAnimation, StyleBase3DCG, StyleBaseInk2D)
	assetModeValues = valueSet(AssetModeM0, AssetModeM1, AssetModeM2)
	lockKeyValues   = valueSet(
		LockTrack,
		LockSource,
		LockVideoModel,
		LockAspectRatio,
		LockDeliveryMode,
		LockStyleCard,
		LockSeamMode,
	)
	memberRoleValues = valueSet(
		model.ManjuMemberRoleProducer,
		model.ManjuMemberRoleWriter,
		model.ManjuMemberRoleDirector,
		model.ManjuMemberRoleReviewer,
		model.ManjuMemberRoleViewer,
	)
)

type Service struct {
	repo                   *repository.Repository
	scriptTaskCreate       ScriptTaskCreateFunc
	skillValidationEnabled *bool
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SetSkillValidationEnabledForTest(enabled bool) {
	s.skillValidationEnabled = &enabled
}

func (s *Service) officialSkillValidationEnabled() bool {
	return s.skillValidationEnabled == nil || *s.skillValidationEnabled
}

func (s *Service) StyleCatalog() StyleCatalogView {
	return StyleCatalog()
}

func p0DeliveryPrompt(receipt *model.ManjuLockReceipt) string {
	if receipt == nil {
		return "P0 锁定回执不可用。\n"
	}
	var snapshot LockSnapshot
	if err := json.Unmarshal([]byte(receipt.PayloadJSON), &snapshot); err != nil {
		return "P0 锁定回执不可解析。\n"
	}
	if err := hydrateLockStylePrompts(&snapshot); err != nil {
		return "P0 锁定回执画风字段不可解析。\n"
	}
	styleParameters := sanitizeStyleNames(snapshot.StylePromptParameters)
	if styleParameters == "" {
		styleParameters = "未提供可投喂视觉参数"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "【P0 锁定 revision %d / 交付层参数】\n", receipt.Revision)
	fmt.Fprintf(&out, "赛道：%s\n素材来源：%s\n视频模型：%s\n画幅：%s\n交付形态：%s\n段缝衔接：%s\n", snapshot.Track, snapshot.Source, snapshot.VideoModel, snapshot.AspectRatio, snapshot.DeliveryMode, fallbackValue(snapshot.SeamMode, "单段任务，不触发"))
	fmt.Fprintf(&out, "目标时长：%d秒\n段数：%d\n单段上限：%d秒\n素材形态：%s\n影像基底：%s\n可投喂视觉参数：%s\n", snapshot.TargetDurationSeconds, snapshot.SegmentCount, snapshot.SingleSegmentMaxSeconds, snapshot.AssetMode, snapshot.StyleBase, styleParameters)
	if lensRange, found, err := styleLensRangeForSource(snapshot.StyleCardName); err == nil && found {
		fmt.Fprintf(&out, "P0 画风焦段硬锁：允许区间 [%dmm, %dmm]\n", lensRange.Min, lensRange.Max)
	}
	if motion := strings.TrimSpace(snapshot.StyleMotionParameters); motion != "" {
		fmt.Fprintf(&out, "运镜参数：%s\n", motion)
	}
	return out.String()
}

func sanitizeStyleNames(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	names := make([]string, 0, len(styleCards))
	for _, card := range styleCards {
		if name := strings.TrimSpace(card.Name); name != "" {
			names = append(names, name)
		}
		if id := strings.TrimSpace(card.ID); id != "" {
			names = append(names, id)
		}
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, name := range names {
		value = strings.ReplaceAll(value, name, "[风格名已移除]")
	}
	return strings.TrimSpace(value)
}

type LockInput struct {
	Track                    string   `json:"track"`
	Source                   string   `json:"source"`
	VideoModel               string   `json:"videoModel"`
	AspectRatio              string   `json:"aspectRatio"`
	DeliveryMode             string   `json:"deliveryMode"`
	SeamMode                 string   `json:"seamMode,omitempty"`
	TargetDurationSeconds    int      `json:"targetDurationSeconds"`
	StyleBase                string   `json:"styleBase"`
	StyleCardID              string   `json:"styleCardId"`
	StyleCustomDescription   string   `json:"styleCustomDescription,omitempty"`
	StyleVisualPrompt        string   `json:"styleVisualPrompt,omitempty"`
	StyleMotionParameters    string   `json:"styleMotionParameters,omitempty"`
	StyleForbiddenItems      []string `json:"styleForbiddenItems,omitempty"`
	StyleReferenceResourceID string   `json:"styleReferenceResourceId,omitempty"`
	StyleReferenceAnalysis   string   `json:"styleReferenceAnalysis,omitempty"`
	StyleReferencePalette    string   `json:"styleReferencePalette,omitempty"`
	AssetMode                string   `json:"assetMode,omitempty"`
	ConfirmedLocks           []string `json:"confirmedLocks"`
}

type LockSnapshot struct {
	Track                    string   `json:"track"`
	Source                   string   `json:"source"`
	VideoModel               string   `json:"videoModel"`
	AspectRatio              string   `json:"aspectRatio"`
	DeliveryMode             string   `json:"deliveryMode"`
	SeamMode                 string   `json:"seamMode,omitempty"`
	TargetDurationSeconds    int      `json:"targetDurationSeconds"`
	SegmentCount             int      `json:"segmentCount"`
	SingleSegmentMaxSeconds  int      `json:"singleSegmentMaxSeconds"`
	StyleBase                string   `json:"styleBase"`
	StyleCardID              string   `json:"styleCardId"`
	StyleCustomDescription   string   `json:"styleCustomDescription,omitempty"`
	StyleVisualPrompt        string   `json:"styleVisualPrompt,omitempty"`
	StyleMotionParameters    string   `json:"styleMotionParameters,omitempty"`
	StyleForbiddenItems      []string `json:"styleForbiddenItems,omitempty"`
	StyleCardName            string   `json:"styleCardName"`
	StyleFamily              string   `json:"styleFamily"`
	StyleFightRating         int      `json:"styleFightRating"`
	StyleEffectRating        int      `json:"styleEffectRating"`
	StyleGrade               string   `json:"styleGrade"`
	StyleCatalogVersion      string   `json:"styleCatalogVersion"`
	StylePromptParameters    string   `json:"stylePromptParameters"`
	StyleImagePrompt         string   `json:"styleImagePrompt"`
	StylePerformancePrompt   string   `json:"stylePerformancePrompt,omitempty"`
	StyleReferenceResourceID string   `json:"styleReferenceResourceId,omitempty"`
	StyleReferenceStorageKey string   `json:"styleReferenceStorageKey,omitempty"`
	StyleReferenceAnalysis   string   `json:"styleReferenceAnalysis,omitempty"`
	StyleReferencePalette    string   `json:"styleReferencePalette,omitempty"`
	AssetMode                string   `json:"assetMode"`
	ConfirmedLocks           []string `json:"confirmedLocks,omitempty"`
}

type LockReceipt struct {
	ID           string       `json:"id"`
	ProjectID    string       `json:"projectId"`
	Revision     int64        `json:"revision"`
	Status       string       `json:"status"`
	Snapshot     LockSnapshot `json:"snapshot"`
	SegmentCount int          `json:"segmentCount"`
	ContentHash  string       `json:"contentHash"`
	CreatedBy    string       `json:"createdBy"`
	CreatedAt    time.Time    `json:"createdAt"`
}

type MemberInput struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

type ModelSelection struct {
	ChannelID string `json:"channelId"`
	ModelKey  string `json:"modelKey"`
}

type ModelBindingInput struct {
	Text  ModelSelection `json:"text"`
	Image ModelSelection `json:"image"`
	Video ModelSelection `json:"video"`
}

type ProjectModelBinding struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"projectId"`
	Text      ModelSelection `json:"text"`
	Image     ModelSelection `json:"image"`
	Video     ModelSelection `json:"video"`
	UpdatedBy string         `json:"updatedBy"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type ProjectSummary struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        string              `json:"type"`
	AspectRatio string              `json:"aspectRatio"`
	SourceType  string              `json:"sourceType"`
	Status      model.ProjectStatus `json:"status"`
	AccessRole  string              `json:"accessRole"`
	Revision    int64               `json:"revision"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

type ProjectDetail struct {
	Project    model.Project              `json:"project"`
	AccessRole string                     `json:"accessRole"`
	Members    []model.ManjuProjectMember `json:"members,omitempty"`
	Lock       *LockReceipt               `json:"lock,omitempty"`
	Models     *ProjectModelBinding       `json:"models,omitempty"`
}

func (s *Service) ListProjects(userID string) ([]ProjectSummary, error) {
	projects, err := s.repo.ManjuProjectsForUser(strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	result := make([]ProjectSummary, 0, len(projects))
	for _, item := range projects {
		result = append(result, ProjectSummary{
			ID:          item.Project.ID,
			Name:        item.Project.Name,
			Type:        item.Project.Type,
			AspectRatio: item.Project.AspectRatio,
			SourceType:  item.Project.SourceType,
			Status:      item.Project.Status,
			AccessRole:  item.Role,
			Revision:    item.Project.Revision,
			UpdatedAt:   item.Project.UpdatedAt,
		})
	}
	return result, nil
}

func (s *Service) ProjectDetail(userID, projectID string) (*ProjectDetail, error) {
	access, err := s.repo.ManjuProjectForUser(strings.TrimSpace(userID), strings.TrimSpace(projectID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	detail := &ProjectDetail{Project: access.Project, AccessRole: access.Role}
	modelBinding, err := s.repo.ManjuProjectModelBinding(access.Project.ID)
	if err != nil {
		return nil, err
	}
	if modelBinding != nil {
		detail.Models = projectModelBindingView(*modelBinding)
	}
	if access.Role == model.ManjuMemberRoleOwner {
		detail.Members, err = s.repo.ManjuProjectMembers(access.Project.ID)
		if err != nil {
			return nil, err
		}
	}
	receipt, err := s.repo.LatestManjuLockReceipt(access.Project.ID)
	if err != nil {
		return nil, err
	}
	if receipt != nil {
		detail.Lock, err = receiptView(*receipt)
		if err != nil {
			return nil, err
		}
	}
	return detail, nil
}

func (s *Service) SaveLock(actorID, projectID string, input LockInput) (*LockReceipt, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	var result *LockReceipt
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if access.Role != model.ManjuMemberRoleOwner && access.Role != model.ManjuMemberRoleProducer {
			return ErrForbidden
		}
		if err := tx.LockManjuProject(projectID); err != nil {
			return err
		}

		snapshot, err := normalizeLock(input)
		if err != nil {
			return err
		}
		if err := validateConfirmedLocks(input, snapshot.SegmentCount); err != nil {
			return err
		}
		if snapshot.StyleReferenceResourceID != "" {
			resource, err := tx.ResourceForUser(actorID, snapshot.StyleReferenceResourceID)
			if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil ||
				resource.Kind != "image" || resource.Status != model.ResourceStatusReady {
				return fmt.Errorf("%w: 风格参考图必须是当前用户已上传完成的图片", ErrInvalidLock)
			}
			if err != nil {
				return err
			}
			snapshot.StyleReferenceStorageKey = "resource:" + resource.ID
		}
		payload, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(payload)

		latest, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		revision := int64(1)
		if latest != nil {
			revision = latest.Revision + 1
		}
		if err := tx.SupersedeManjuLockReceipts(projectID); err != nil {
			return err
		}
		createdAt := time.Now()
		receipt := &model.ManjuLockReceipt{
			ID:          uuid.NewString(),
			ProjectID:   projectID,
			Revision:    revision,
			Status:      model.ManjuLockStatusLocked,
			PayloadJSON: string(payload),
			ContentHash: hex.EncodeToString(hash[:]),
			CreatedBy:   actorID,
			CreatedAt:   createdAt,
		}
		if err := tx.CreateManjuLockReceipt(receipt); err != nil {
			return err
		}
		if err := tx.AbandonManjuRunsBeforeLockRevision(projectID, revision); err != nil {
			return err
		}
		if err := initializeManjuProjectState(tx, access.Project.Name, receipt, actorID); err != nil {
			return err
		}
		view, err := receiptView(*receipt)
		if err != nil {
			return err
		}
		result = view
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) InviteMember(actorID, projectID string, input MemberInput) (*model.ManjuProjectMember, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.Role = strings.TrimSpace(input.Role)
	if input.UserID == "" || !memberRoleValues[input.Role] {
		return nil, ErrInvalidMember
	}

	var result *model.ManjuProjectMember
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if access.Role != model.ManjuMemberRoleOwner {
			return ErrForbidden
		}
		if err := tx.LockManjuProject(projectID); err != nil {
			return err
		}
		user, ok, err := tx.UserByID(input.UserID)
		if err != nil {
			return err
		}
		if !ok || user.Status != model.UserStatusActive {
			return ErrInvalidMember
		}
		if input.UserID == access.Project.UserID {
			return ErrInvalidMember
		}
		member := &model.ManjuProjectMember{
			ID:        uuid.NewString(),
			ProjectID: projectID,
			UserID:    input.UserID,
			Role:      input.Role,
			Status:    model.ManjuMemberStatusActive,
			InvitedBy: actorID,
		}
		if err := tx.UpsertManjuProjectMember(member); err != nil {
			return err
		}
		result = member
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) RemoveMember(actorID, projectID, memberID string) error {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	memberID = strings.TrimSpace(memberID)
	if memberID == "" {
		return ErrMemberNotFound
	}
	return s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if access.Role != model.ManjuMemberRoleOwner {
			return ErrForbidden
		}
		if err := tx.LockManjuProject(projectID); err != nil {
			return err
		}
		if err := tx.DeleteManjuProjectMember(projectID, memberID); errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemberNotFound
		} else if err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) DeleteProject(actorID, projectID string) error {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	if actorID == "" || projectID == "" {
		return ErrNotFound
	}
	return s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if access.Role != model.ManjuMemberRoleOwner {
			return ErrForbidden
		}
		if err := tx.LockManjuProject(projectID); err != nil {
			return err
		}
		active, err := tx.HasManjuProjectActiveTasks(projectID)
		if err != nil {
			return err
		}
		if active {
			return ErrHasActiveTasks
		}
		if err := tx.DeleteManjuProjectRecords(projectID); errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) SaveModels(actorID, projectID string, input ModelBindingInput) (*ProjectModelBinding, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	input.Text = normalizeModelSelection(input.Text)
	input.Image = normalizeModelSelection(input.Image)
	input.Video = normalizeModelSelection(input.Video)
	selections := []ModelSelection{input.Text, input.Image, input.Video}
	hasSelection := false
	for _, selection := range selections {
		if selection.ChannelID == "" && selection.ModelKey == "" {
			continue
		}
		if selection.ChannelID == "" || selection.ModelKey == "" {
			return nil, ErrInvalidModelBinding
		}
		hasSelection = true
	}
	if !hasSelection {
		return nil, ErrInvalidModelBinding
	}

	var result *ProjectModelBinding
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if access.Role != model.ManjuMemberRoleOwner && access.Role != model.ManjuMemberRoleProducer {
			return ErrForbidden
		}
		if err := tx.LockManjuProject(projectID); err != nil {
			return err
		}

		checks := []struct {
			selection  ModelSelection
			capability string
		}{
			{input.Text, "text"},
			{input.Image, "image"},
			{input.Video, "video"},
		}
		for _, check := range checks {
			if check.selection.ChannelID == "" && check.selection.ModelKey == "" {
				continue
			}
			enabled, err := tx.ManjuModelEnabled(check.selection.ChannelID, check.selection.ModelKey, check.capability)
			if err != nil {
				return err
			}
			if !enabled {
				return ErrInvalidModelBinding
			}
		}

		binding := &model.ManjuProjectModelBinding{
			ID:             uuid.NewString(),
			ProjectID:      projectID,
			TextChannelID:  input.Text.ChannelID,
			TextModelKey:   input.Text.ModelKey,
			ImageChannelID: input.Image.ChannelID,
			ImageModelKey:  input.Image.ModelKey,
			VideoChannelID: input.Video.ChannelID,
			VideoModelKey:  input.Video.ModelKey,
			UpdatedBy:      actorID,
		}
		if err := tx.UpsertManjuProjectModelBinding(binding); err != nil {
			return err
		}
		result = projectModelBindingView(*binding)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeModelSelection(input ModelSelection) ModelSelection {
	return ModelSelection{
		ChannelID: strings.TrimSpace(input.ChannelID),
		ModelKey:  strings.TrimSpace(input.ModelKey),
	}
}

func projectModelBindingView(binding model.ManjuProjectModelBinding) *ProjectModelBinding {
	return &ProjectModelBinding{
		ID:        binding.ID,
		ProjectID: binding.ProjectID,
		Text: ModelSelection{
			ChannelID: binding.TextChannelID,
			ModelKey:  binding.TextModelKey,
		},
		Image: ModelSelection{
			ChannelID: binding.ImageChannelID,
			ModelKey:  binding.ImageModelKey,
		},
		Video: ModelSelection{
			ChannelID: binding.VideoChannelID,
			ModelKey:  binding.VideoModelKey,
		},
		UpdatedBy: binding.UpdatedBy,
		UpdatedAt: binding.UpdatedAt,
	}
}

func normalizeLock(input LockInput) (LockSnapshot, error) {
	snapshot := LockSnapshot{
		Track:                    strings.TrimSpace(input.Track),
		Source:                   strings.TrimSpace(input.Source),
		VideoModel:               strings.TrimSpace(input.VideoModel),
		AspectRatio:              strings.TrimSpace(input.AspectRatio),
		DeliveryMode:             strings.TrimSpace(input.DeliveryMode),
		SeamMode:                 strings.TrimSpace(input.SeamMode),
		TargetDurationSeconds:    input.TargetDurationSeconds,
		StyleBase:                strings.TrimSpace(input.StyleBase),
		StyleCardID:              strings.TrimSpace(input.StyleCardID),
		StyleCustomDescription:   strings.TrimSpace(input.StyleCustomDescription),
		StyleVisualPrompt:        strings.TrimSpace(input.StyleVisualPrompt),
		StyleMotionParameters:    strings.TrimSpace(input.StyleMotionParameters),
		StyleForbiddenItems:      append([]string(nil), input.StyleForbiddenItems...),
		StyleReferenceResourceID: strings.TrimSpace(input.StyleReferenceResourceID),
		StyleReferenceAnalysis:   strings.TrimSpace(input.StyleReferenceAnalysis),
		StyleReferencePalette:    strings.TrimSpace(input.StyleReferencePalette),
		AssetMode:                strings.ToUpper(strings.TrimSpace(input.AssetMode)),
		ConfirmedLocks:           append([]string(nil), input.ConfirmedLocks...),
	}
	if snapshot.AssetMode == "" {
		snapshot.AssetMode = AssetModeM1
	}
	if !trackValues[snapshot.Track] ||
		!sourceValues[snapshot.Source] ||
		modelLimits[snapshot.VideoModel] == 0 ||
		!aspectValues[snapshot.AspectRatio] ||
		!deliveryValues[snapshot.DeliveryMode] ||
		!styleBaseValues[snapshot.StyleBase] ||
		snapshot.StyleCardID == "" ||
		!assetModeValues[snapshot.AssetMode] ||
		snapshot.TargetDurationSeconds <= 0 {
		return LockSnapshot{}, ErrInvalidLock
	}
	if snapshot.StyleCardID == StyleCustomCardID {
		styleFields := applyCustomStyleFields(&snapshot)
		if styleFields.Visual == "" {
			return LockSnapshot{}, ErrInvalidLock
		}
		snapshot.StyleCardName = "自定义画风"
		snapshot.StyleFamily = "CUSTOM"
		snapshot.StyleGrade = "C"
		snapshot.StyleCatalogVersion = StyleCatalogVersion
	} else {
		card, ok := styleCardByID[snapshot.StyleCardID]
		if !ok || !card.SupportsStyleBase(snapshot.StyleBase) {
			return LockSnapshot{}, ErrInvalidLock
		}
		snapshot.StyleCardName = card.Name
		snapshot.StyleFamily = card.Family
		snapshot.StyleFightRating = card.FightRating
		snapshot.StyleEffectRating = card.EffectRating
		snapshot.StyleGrade = card.Grade
		snapshot.StyleCatalogVersion = StyleCatalogVersion
		snapshot.StylePromptParameters = card.PromptParameters
		snapshot.StyleCustomDescription = ""
	}
	if err := hydrateLockStylePrompts(&snapshot); err != nil {
		return LockSnapshot{}, err
	}

	maxSeconds := modelLimits[snapshot.VideoModel]
	snapshot.SingleSegmentMaxSeconds = maxSeconds
	snapshot.SegmentCount = (snapshot.TargetDurationSeconds + maxSeconds - 1) / maxSeconds
	if snapshot.SegmentCount > 1 {
		if !seamValues[snapshot.SeamMode] {
			return LockSnapshot{}, fmt.Errorf("%w: 目标时长需要 %d 段，请选择段缝衔接方式", ErrSeamRequired, snapshot.SegmentCount)
		}
	} else {
		snapshot.SeamMode = ""
	}
	return snapshot, nil
}

func validateConfirmedLocks(input LockInput, segmentCount int) error {
	for _, key := range input.ConfirmedLocks {
		if !lockKeyValues[key] {
			return fmt.Errorf("%w: unknown confirmation key %q", ErrInvalidLock, key)
		}
	}
	required := []string{
		LockTrack, LockSource, LockVideoModel,
		LockAspectRatio, LockDeliveryMode, LockStyleCard,
	}
	if segmentCount > 1 {
		required = append(required, LockSeamMode)
	}
	confirmed := valueSet(input.ConfirmedLocks...)
	for _, key := range required {
		if !confirmed[key] {
			return fmt.Errorf("%w: %s", ErrExplicitLockRequired, key)
		}
	}
	return nil
}

func receiptView(receipt model.ManjuLockReceipt) (*LockReceipt, error) {
	var snapshot LockSnapshot
	if err := json.Unmarshal([]byte(receipt.PayloadJSON), &snapshot); err != nil {
		return nil, fmt.Errorf("解析 P0 锁回执失败: %w", err)
	}
	if err := hydrateLockStylePrompts(&snapshot); err != nil {
		return nil, fmt.Errorf("迁移 P0 锁画风字段失败: %w", err)
	}
	return &LockReceipt{
		ID:           receipt.ID,
		ProjectID:    receipt.ProjectID,
		Revision:     receipt.Revision,
		Status:       receipt.Status,
		Snapshot:     snapshot,
		SegmentCount: snapshot.SegmentCount,
		ContentHash:  receipt.ContentHash,
		CreatedBy:    receipt.CreatedBy,
		CreatedAt:    receipt.CreatedAt,
	}, nil
}

func valueSet(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
