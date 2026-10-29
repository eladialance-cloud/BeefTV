package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/assets"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

const (
	CanvasPackageStageP2Assets = model.ManjuCanvasStageP2Assets
	CanvasPackageStageP3Images = model.ManjuCanvasStageP3Images
	CanvasPackageStageP4Videos = model.ManjuCanvasStageP4Videos
)

var (
	ErrInvalidCanvasPackage     = errors.New("invalid manju canvas package")
	ErrCanvasPackageUnready     = errors.New("manju canvas package source artifact is not ready")
	ErrCanvasImportNotFound     = errors.New("manju canvas import not found")
	ErrCanvasImportInvalid      = errors.New("invalid manju canvas import")
	ErrCanvasImportStale        = errors.New("manju canvas import package is stale")
	ErrCanvasImportNotReady     = errors.New("manju canvas import tasks are not ready")
	canvasTaskBlockStart        = "<!-- MANJU_CANVAS_TASKS_JSON"
	canvasTaskBlockEnd          = "-->"
	canvasPlotSceneExactPattern = regexp.MustCompile(`^(?:覆盖\s*[:：]\s*)?场([1-9][0-9]*)(?:\s|✅|$).*`)
	canvasMarkdownHeadingLevel  = regexp.MustCompile(`^(#{1,6})\s+`)
	canvasAssetBlockHeading     = regexp.MustCompile(`^(#{2,6})\s+\*{0,2}\s*图\s*([0-9０-９]+)[^\n]*$`)
	canvasAssetHeadingTitle     = regexp.MustCompile(`^#{2,6}\s+\*{0,2}\s*图\s*[0-9０-９]+\s*[·:：-]?\s*(.*?)\s*\*{0,2}\s*$`)
	canvasImageNumber           = regexp.MustCompile(`图\s*([0-9０-９]+)`)
	canvasBlockAssetCode        = regexp.MustCompile(`\b(?:CHR|SCN|PRP|GRP|AUD)-[^\s|,，、:：/／()（）\[\]{}【】<>；;]+`)
	canvasVisualCodePattern     = regexp.MustCompile(`\b(?:CHR|SCN|PRP)-[^\s|,，、:：/／()（）\[\]{}【】<>；;]+`)
	canvasNamespaceCodePattern  = regexp.MustCompile(`@//(?i:(scn|chr|prp))/\s*-\s*([^\s|,，、:：/／()（）\[\]{}【】<>；;]+)`)
	canvasFigureAssetPattern    = regexp.MustCompile(`(?:图|参考图)\s*([0-9０-９]+)\s*(?:=|＝|:|：|->|→)\s*((?:CHR|SCN|PRP)-[^\s|,，、:：)\]}]+)`)
	canvasResourceKeyPattern    = regexp.MustCompile(`(?:resource:|/api/resources/)([A-Za-z0-9._:-]+)`)
)

type CanvasPackageTask struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Code              string   `json:"code"`
	AssetCode         string   `json:"assetCode,omitempty"`
	Title             string   `json:"title"`
	Prompt            string   `json:"prompt"`
	Category          string   `json:"category"`
	Tags              []string `json:"tags,omitempty"`
	AspectRatio       string   `json:"aspectRatio,omitempty"`
	Seconds           int      `json:"seconds,omitempty"`
	ReferenceCodes    []string `json:"referenceCodes,omitempty"`
	GlobalStylePrompt string   `json:"globalStylePrompt,omitempty"`
	SourceKind        string   `json:"sourceKind"`
	Group             string   `json:"group,omitempty"`
}

type CanvasPackage struct {
	Version         int                      `json:"version"`
	ProjectID       string                   `json:"projectId"`
	Stage           string                   `json:"stage"`
	AssetRunID      string                   `json:"assetRunId,omitempty"`
	ProductionRunID string                   `json:"productionRunId,omitempty"`
	SourceArtifact  string                   `json:"sourceArtifact"`
	Title           string                   `json:"title"`
	Hash            string                   `json:"hash"`
	Style           CanvasPackageStyle       `json:"style"`
	Tasks           []CanvasPackageTask      `json:"tasks"`
	Exemptions      []CanvasPackageExemption `json:"exemptions,omitempty"`
}

type CanvasPackageStyle struct {
	CardID              string `json:"cardId,omitempty"`
	CardName            string `json:"cardName,omitempty"`
	StyleBase           string `json:"styleBase,omitempty"`
	AspectRatio         string `json:"aspectRatio,omitempty"`
	PromptParameters    string `json:"promptParameters,omitempty"`
	ImageStylePrompt    string `json:"imageStylePrompt,omitempty"`
	PerformancePrompt   string `json:"performancePrompt,omitempty"`
	GlobalStylePrompt   string `json:"globalStylePrompt,omitempty"`
	ReferenceResourceID string `json:"referenceResourceId,omitempty"`
	ReferenceAssetID    string `json:"referenceAssetId,omitempty"`
	ReferenceStorageKey string `json:"referenceStorageKey,omitempty"`
	ReferenceAnalysis   string `json:"referenceAnalysis,omitempty"`
	ReferencePalette    string `json:"referencePalette,omitempty"`
}

type CanvasPackageExemption struct {
	Kind   string `json:"kind"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type CanvasImportInput struct {
	Stage           string `json:"stage"`
	AssetRunID      string `json:"assetRunId,omitempty"`
	ProductionRunID string `json:"productionRunId,omitempty"`
	CanvasID        string `json:"canvasId"`
	PackageHash     string `json:"packageHash"`
}

type CanvasImportView struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	Stage           string    `json:"stage"`
	AssetRunID      string    `json:"assetRunId,omitempty"`
	ProductionRunID string    `json:"productionRunId,omitempty"`
	CanvasID        string    `json:"canvasId"`
	PackageHash     string    `json:"packageHash"`
	TaskCount       int       `json:"taskCount"`
	Status          string    `json:"status"`
	CreatedBy       string    `json:"createdBy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type CanvasImportReferenceView struct {
	Code       string `json:"code"`
	AssetCode  string `json:"assetCode,omitempty"`
	AssetID    string `json:"assetId,omitempty"`
	TaskID     string `json:"taskId,omitempty"`
	Title      string `json:"title"`
	Prompt     string `json:"prompt,omitempty"`
	Content    string `json:"content,omitempty"`
	StorageKey string `json:"storageKey,omitempty"`
	Category   string `json:"category,omitempty"`
	Kind       string `json:"kind"`
}

type canvasPackageJSON struct {
	Version int `json:"version"`
	Tasks   []struct {
		Code        string   `json:"code"`
		AssetCode   string   `json:"assetCode"`
		Title       string   `json:"title"`
		Category    string   `json:"category"`
		Prompt      string   `json:"prompt"`
		AspectRatio string   `json:"aspectRatio"`
		Tags        []string `json:"tags"`
	} `json:"tasks"`
}

func (s *Service) CanvasPackage(actorID, projectID, stage, runID string) (*CanvasPackage, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	stage = strings.TrimSpace(stage)
	runID = strings.TrimSpace(runID)
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if access.Role == "" {
		return nil, ErrForbidden
	}
	snapshot := s.latestCanvasLockSnapshot(projectID)
	var pkg *CanvasPackage
	switch stage {
	case CanvasPackageStageP2Assets:
		pkg, err = s.p2CanvasPackage(projectID, runID, snapshot)
	case CanvasPackageStageP3Images:
		pkg, err = s.p3CanvasPackage(projectID, runID, snapshot)
	case CanvasPackageStageP4Videos:
		pkg, err = s.p4CanvasPackage(projectID, runID, snapshot)
	default:
		return nil, ErrInvalidCanvasPackage
	}
	if err != nil {
		return nil, err
	}
	referenceAssetID := ""
	if strings.TrimSpace(snapshot.StyleReferenceResourceID) != "" {
		referenceAssetID, err = s.ensureStyleReferenceAsset(actorID, snapshot.StyleReferenceResourceID)
		if err != nil {
			return nil, err
		}
	}
	pkg.Style = CanvasPackageStyle{
		CardID: snapshot.StyleCardID, CardName: snapshot.StyleCardName, StyleBase: snapshot.StyleBase,
		AspectRatio: snapshot.AspectRatio, PromptParameters: sanitizeStyleNames(snapshot.StylePromptParameters),
		ImageStylePrompt: snapshot.StyleImagePrompt, PerformancePrompt: snapshot.StylePerformancePrompt,
		ReferenceResourceID: snapshot.StyleReferenceResourceID, ReferenceAssetID: referenceAssetID,
		ReferenceStorageKey: snapshot.StyleReferenceStorageKey,
		ReferenceAnalysis:   snapshot.StyleReferenceAnalysis, ReferencePalette: snapshot.StyleReferencePalette,
	}
	if pkg.Stage == CanvasPackageStageP4Videos {
		for _, task := range pkg.Tasks {
			if strings.TrimSpace(task.GlobalStylePrompt) != "" {
				pkg.Style.GlobalStylePrompt = task.GlobalStylePrompt
				break
			}
		}
	}
	pkg.Hash = canvasPackageHash(*pkg)
	return pkg, nil
}

func (s *Service) ensureStyleReferenceAsset(userID, resourceID string) (string, error) {
	userID = strings.TrimSpace(userID)
	resourceID = strings.TrimSpace(resourceID)
	if userID == "" || resourceID == "" {
		return "", nil
	}
	resourceIDs := map[string]struct{}{resourceID: {}}
	existing, err := s.repo.Assets(userID)
	if err != nil {
		return "", err
	}
	for _, asset := range existing {
		if len(assets.DocumentReferencedIDs(asset.PayloadJSON, resourceIDs)) > 0 {
			return asset.ID, nil
		}
	}
	resource, err := s.repo.ResourceForUser(userID, resourceID)
	if err != nil {
		return "", err
	}
	if resource == nil || resource.Kind != "image" || resource.Status != model.ResourceStatusReady {
		return "", fmt.Errorf("%w: 风格参考图资源尚未就绪", ErrCanvasPackageUnready)
	}
	sum := sha256.Sum256([]byte("manju-style-reference\x00" + userID + "\x00" + resourceID))
	assetID := "style_ref_" + hex.EncodeToString(sum[:])[:32]
	payload, err := json.Marshal(map[string]any{
		"id":       assetID,
		"kind":     "image",
		"title":    "风格参考图",
		"category": "material",
		"data": map[string]any{
			"storageKey": "resource:" + resourceID,
			"resourceId": resourceID,
			"url":        resource.PublicURL,
			"width":      resource.Width,
			"height":     resource.Height,
			"bytes":      resource.Size,
			"mimeType":   resource.MimeType,
		},
	})
	if err != nil {
		return "", err
	}
	now := time.Now()
	asset := model.Asset{
		ID: assetID, UserID: userID, Kind: "image", Category: model.AssetCategoryMaterial,
		Status: model.AssetVersionStatusConfirmed, Title: "风格参考图",
		PayloadJSON: string(payload), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.UpsertAsset(&asset); err != nil {
		return "", err
	}
	return assetID, nil
}

func (s *Service) SaveCanvasImport(actorID, projectID string, input CanvasImportInput) (*CanvasImportView, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	input.Stage = strings.TrimSpace(input.Stage)
	input.CanvasID = strings.TrimSpace(input.CanvasID)
	input.PackageHash = strings.TrimSpace(input.PackageHash)
	runID := input.AssetRunID
	if input.Stage != CanvasPackageStageP2Assets {
		runID = input.ProductionRunID
	}
	if input.CanvasID == "" || runID == "" || input.PackageHash == "" {
		return nil, ErrCanvasImportInvalid
	}
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !canEditAsset(access.Role) && !canEditProduction(access.Role) {
		return nil, ErrForbidden
	}
	canvas, err := s.repo.CanvasProjectForUser(actorID, input.CanvasID)
	if errors.Is(err, gorm.ErrRecordNotFound) || canvas == nil || canvas.ProjectID != projectID {
		return nil, ErrCanvasImportInvalid
	}
	if err != nil {
		return nil, err
	}
	pkg, err := s.CanvasPackage(actorID, projectID, input.Stage, runID)
	if err != nil {
		return nil, err
	}
	if pkg.Hash != input.PackageHash {
		return nil, ErrCanvasImportStale
	}
	item := &model.ManjuCanvasImport{
		ID: uuidForCanvasImport(projectID, input.Stage), ProjectID: projectID, Stage: input.Stage,
		AssetRunID: input.AssetRunID, ProductionRunID: input.ProductionRunID,
		CanvasID: input.CanvasID, PackageHash: input.PackageHash, TaskCount: len(pkg.Tasks),
		Status: model.ManjuCanvasImportActive, CreatedBy: actorID,
	}
	if err := s.repo.SaveManjuCanvasImport(item); err != nil {
		return nil, err
	}
	view := canvasImportView(*item)
	return &view, nil
}

func staleP3CanvasImportForRun(tx *repository.Repository, projectID, runID string) error {
	item, err := tx.ManjuCanvasImportForRun(projectID, CanvasPackageStageP3Images, runID)
	if err != nil || item == nil {
		return err
	}
	item.Status = model.ManjuCanvasImportStale
	return tx.SaveManjuCanvasImport(item)
}

func (s *Service) CanvasImports(actorID, projectID string) ([]CanvasImportView, error) {
	access, err := s.repo.ManjuProjectForUser(strings.TrimSpace(actorID), strings.TrimSpace(projectID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if access.Role == "" {
		return nil, ErrForbidden
	}
	items, err := s.repo.ManjuCanvasImports(projectID)
	if err != nil {
		return nil, err
	}
	result := make([]CanvasImportView, 0, len(items))
	for _, item := range items {
		result = append(result, canvasImportView(item))
	}
	return result, nil
}

func (s *Service) CanvasImportReferences(actorID, projectID, stage string) ([]CanvasImportReferenceView, error) {
	access, err := s.repo.ManjuProjectForUser(strings.TrimSpace(actorID), strings.TrimSpace(projectID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if access.Role == "" {
		return nil, ErrForbidden
	}
	item, err := s.repo.ManjuCanvasImportForStage(projectID, stage)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCanvasImportNotFound
	}
	canvas, err := s.repo.CanvasProjectForUser(item.CreatedBy, item.CanvasID)
	if errors.Is(err, gorm.ErrRecordNotFound) || canvas == nil {
		return nil, ErrCanvasImportNotFound
	}
	if err != nil {
		return nil, err
	}
	references := canvasImportReferences(canvas.PayloadJSON, stage)
	if stage == CanvasPackageStageP2Assets && strings.TrimSpace(item.AssetRunID) != "" {
		if pkg, packageErr := s.CanvasPackage(item.CreatedBy, projectID, stage, item.AssetRunID); packageErr == nil {
			references = mergeCanvasImportReferencesWithP2Tasks(references, pkg.Tasks)
		}
	}
	return references, nil
}

func (s *Service) verifyProductionCanvasImport(projectID, runID, stageKey string) (string, error) {
	packageStage := ""
	switch stageKey {
	case ProductionStageStoryboard:
		packageStage = CanvasPackageStageP3Images
	case ProductionStageFeed, ProductionStageQC:
		packageStage = CanvasPackageStageP4Videos
	default:
		return "", nil
	}
	item, err := s.repo.ManjuCanvasImportForRun(projectID, packageStage, runID)
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", nil
	}
	pkg, err := s.CanvasPackage(item.CreatedBy, projectID, packageStage, runID)
	if err != nil {
		return "", err
	}
	if item.PackageHash != pkg.Hash {
		return "", ErrCanvasImportStale
	}
	canvas, err := s.repo.CanvasProjectForUser(item.CreatedBy, item.CanvasID)
	if errors.Is(err, gorm.ErrRecordNotFound) || canvas == nil {
		return "", ErrCanvasImportNotFound
	}
	if err != nil {
		return "", err
	}
	resources, err := canvasImportResources(canvas.PayloadJSON, pkg.Tasks, packageStage)
	if err != nil {
		return "", err
	}
	for _, expected := range resources {
		resource, err := s.repo.ResourceForUser(item.CreatedBy, expected.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil {
			return "", ErrCanvasImportNotReady
		}
		if err != nil {
			return "", err
		}
		if resource.Kind != expected.Kind || resource.Status != model.ResourceStatusReady {
			return "", ErrCanvasImportNotReady
		}
	}
	return packageStage, nil
}

func (s *Service) p3CanvasStoryboardImages(projectID, runID string) ([]model.ManjuProductionImage, error) {
	item, err := s.repo.ManjuCanvasImportForRun(projectID, CanvasPackageStageP3Images, runID)
	if err != nil || item == nil {
		return nil, err
	}
	pkg, err := s.CanvasPackage(item.CreatedBy, projectID, CanvasPackageStageP3Images, runID)
	if err != nil {
		return nil, err
	}
	if item.PackageHash != pkg.Hash {
		return nil, ErrCanvasImportStale
	}
	canvas, err := s.repo.CanvasProjectForUser(item.CreatedBy, item.CanvasID)
	if errors.Is(err, gorm.ErrRecordNotFound) || canvas == nil {
		return nil, ErrCanvasImportNotFound
	}
	if err != nil {
		return nil, err
	}
	resources, err := canvasImportResources(canvas.PayloadJSON, pkg.Tasks, CanvasPackageStageP3Images)
	if err != nil {
		return nil, err
	}
	images := make([]model.ManjuProductionImage, 0, len(pkg.Tasks))
	position := 1
	for index, task := range pkg.Tasks {
		resource := resources[index]
		live, err := s.repo.ResourceForUser(item.CreatedBy, resource.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) || live == nil ||
			live.Kind != resource.Kind || live.Status != model.ResourceStatusReady {
			return nil, ErrCanvasImportNotReady
		}
		if err != nil {
			return nil, err
		}
		if task.Group != "storyboard" {
			continue
		}
		images = append(images, model.ManjuProductionImage{
			ProjectID: projectID, RunID: runID, Position: position,
			SegmentCode: task.Code, ShotLabel: "canvas", Prompt: task.Prompt,
			Status: ProductionImageStatusReady, TaskID: task.ID, ResourceID: resource.ID,
			CreatedBy: item.CreatedBy,
		})
		position++
	}
	return images, nil
}

func (s *Service) p2CanvasPackage(projectID, runID string, snapshot LockSnapshot) (*CanvasPackage, error) {
	run, err := s.repo.ManjuAssetRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrAssetRunNotFound
	}
	if run.AssetMode == AssetModeM0Value {
		return nil, fmt.Errorf("%w: M0 只产出文字外观档案，不产出出图提示词；请使用同一份 P1 新建 M1 资产任务", ErrCanvasPackageUnready)
	}
	stages, err := s.repo.ManjuAssetStages(runID)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.repo.ManjuAssetArtifacts(runID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.ManjuAssetArtifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	var compile *model.ManjuAssetArtifact
	for index := len(stages) - 1; index >= 0; index-- {
		if stages[index].Key != AssetStageCompile || strings.TrimSpace(stages[index].ArtifactID) == "" {
			continue
		}
		artifact, ok := byID[stages[index].ArtifactID]
		if ok {
			compile = &artifact
			break
		}
	}
	if compile == nil {
		return nil, ErrCanvasPackageUnready
	}
	tasks, err := parseP2CanvasTasks(compile.Content, run.AssetMode, snapshot.AspectRatio)
	if err != nil {
		return nil, err
	}
	return &CanvasPackage{
		Version: 1, ProjectID: projectID, Stage: CanvasPackageStageP2Assets, AssetRunID: runID,
		SourceArtifact: compile.Kind, Title: "P2 数字资产", Tasks: tasks,
	}, nil
}

func (s *Service) p3CanvasPackage(projectID, runID string, snapshot LockSnapshot) (*CanvasPackage, error) {
	run, err := s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrProductionRunNotFound
	}
	stage, err := s.repo.ManjuProductionStage(runID, ProductionStageStoryboard)
	if err != nil {
		return nil, err
	}
	if stage == nil || strings.TrimSpace(stage.ArtifactID) == "" {
		return nil, fmt.Errorf("%w: P3 stage is missing an artifact for run %s", ErrCanvasPackageUnready, runID)
	}
	artifact, err := productionArtifactContent(s.repo, runID, stage.ArtifactID)
	if err != nil {
		return nil, err
	}
	segments, err := parseStoryboardImageSegments(artifact)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanvasPackageUnready, err)
	}
	required, err := p3ArtifactRequiredSegments(artifact)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanvasPackageUnready, err)
	}
	requirements, err := p3DeliveryRequirements(artifact, required)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanvasPackageUnready, err)
	}
	checklistRows, err := p3DeliveryChecklistRows(artifact, required)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanvasPackageUnready, err)
	}
	assetCodes := uniqueCanvasCodes(canvasVisualCodePattern.FindAllString(artifact, -1)...)
	figureAssets := s.canvasFigureAssetMap(run.AssetRunID)
	useAssetReferences := true
	assetMode := "M1"
	if assetRun, err := s.repo.ManjuAssetRun(projectID, run.AssetRunID); err == nil && assetRun != nil {
		assetMode = assetRun.AssetMode
		if assetRun.AssetMode == AssetModeM0Value {
			useAssetReferences = false
		}
	}
	exemptions := make([]CanvasPackageExemption, 0)
	tasks := make([]CanvasPackageTask, 0, len(segments)+4)
	for _, segment := range segments {
		requirement := requirements[p3SegmentKey(segment.Code)]
		if requirement.Exempt {
			exemptions = append(exemptions, CanvasPackageExemption{
				Kind: "storyboard", Code: segment.Code, Reason: requirement.Reason,
			})
			continue
		}
		prompt := buildStoryboardImagePrompt(segment, assetMode, snapshot.AspectRatio, snapshot.StyleBase, snapshot.StylePromptParameters)
		prompt = removeCanvasTaskStyleLine(prompt)
		segmentAssetCodes := []string(nil)
		if useAssetReferences {
			segmentAssetCodes = storyboardCanvasReferenceCodes(segment.Assets, figureAssets)
		}
		tasks = append(tasks, CanvasPackageTask{
			ID: canvasTaskID(CanvasPackageStageP3Images, "storyboard:"+segment.Code), Kind: "image",
			Code: segment.Code, Title: "故事板图 · " + segment.Code, Prompt: prompt,
			Category: "material", Tags: []string{"P3", "故事板图"}, AspectRatio: storyboardSheetAspectRatio,
			ReferenceCodes: segmentAssetCodes, SourceKind: "DIRECTOR_STORYBOARD.md", Group: "storyboard",
		})
	}
	plotScenes, plotExemptions, err := parseP3PlotChecklist(checklistRows)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanvasPackageUnready, err)
	}
	exemptions = append(exemptions, plotExemptions...)
	for _, scene := range plotScenes {
		prompt := buildPlotGridCanvasPrompt(artifact, scene.Code, scene.Requirement, scene.Grid, snapshot)
		sceneAssetCodes := []string(nil)
		if useAssetReferences {
			sceneAssetCodes = assetCodes
		}
		tasks = append(tasks, CanvasPackageTask{
			ID: canvasTaskID(CanvasPackageStageP3Images, "plot:"+scene.Code), Kind: "image",
			Code: scene.Code, Title: "剧情分镜格 · " + scene.Code, Prompt: prompt,
			Category: "material", Tags: []string{"P3", "剧情分镜格", scene.Grid}, AspectRatio: snapshot.AspectRatio,
			ReferenceCodes: sceneAssetCodes, SourceKind: "DIRECTOR_STORYBOARD.md", Group: "plot-grid",
		})
	}
	if len(tasks) == 0 && len(exemptions) == 0 {
		return nil, fmt.Errorf(
			"%w: P3 交付清单未产生画布任务或合法豁免 (segments=%d requirements=%d)",
			ErrCanvasPackageUnready,
			len(segments),
			len(requirements),
		)
	}
	return &CanvasPackage{
		Version: 1, ProjectID: projectID, Stage: CanvasPackageStageP3Images, ProductionRunID: runID,
		SourceArtifact: model.ManjuProductionArtifactStoryboard, Title: "P3 故事板与剧情分镜格", Tasks: tasks, Exemptions: exemptions,
	}, nil
}

func (s *Service) p4CanvasPackage(projectID, runID string, snapshot LockSnapshot) (*CanvasPackage, error) {
	run, err := s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrProductionRunNotFound
	}
	stage, err := s.repo.ManjuProductionStage(runID, ProductionStageFeed)
	if err != nil {
		return nil, err
	}
	if stage == nil || strings.TrimSpace(stage.ArtifactID) == "" {
		return nil, ErrCanvasPackageUnready
	}
	artifact, err := productionArtifactContent(s.repo, runID, stage.ArtifactID)
	if err != nil {
		return nil, err
	}
	segments, err := parseProductionVideoSegments(artifact)
	if err != nil || len(segments) == 0 {
		return nil, fmt.Errorf("%w: %v", ErrCanvasPackageUnready, ErrProductionVideoInvalid)
	}
	p3Stage, err := s.repo.ManjuProductionStage(runID, ProductionStageStoryboard)
	if err != nil {
		return nil, err
	}
	if p3Stage == nil || p3Stage.Status != model.ManjuProductionStageStatusLocked || strings.TrimSpace(p3Stage.ArtifactID) == "" {
		return nil, ErrProductionStageNotReady
	}
	state, stateSnapshot, err := manjuProjectStateMapForArtifact(
		s.repo,
		projectID,
		run.LockRevision,
		"production:director_storyboard",
		p3Stage.ArtifactID,
	)
	if err != nil {
		return nil, err
	}
	if stateSnapshot == nil {
		return nil, ErrProjectStateUnavailable
	}
	stateSegments := make(map[string]map[string]any)
	for _, raw := range anySlice(state["segments"]) {
		segment := anyMap(raw)
		stateSegments[normalizeProductionSegmentCode(anyString(segment["seg_id"]))] = segment
	}
	readyStoryboards, err := s.currentProductionStoryboardCodes(runID, p3Stage.ArtifactID)
	if err != nil {
		return nil, err
	}
	tasks := make([]CanvasPackageTask, 0, len(segments))
	for _, segment := range segments {
		referenceLimit := 9
		_, hasStoryboard := readyStoryboards[normalizeProductionSegmentCode(segment.Code)]
		if hasStoryboard {
			referenceLimit = 8
		}
		referenceCodes := productionSegmentVisualAssetCodes(
			state,
			stateSegments[normalizeProductionSegmentCode(segment.Code)],
			referenceLimit,
		)
		if hasStoryboard {
			referenceCodes = append(referenceCodes, segment.Code)
		}
		globalStyle := segment.GlobalStyle
		if reference := styleReferenceExecutionRule(snapshot.StyleReferenceAnalysis); reference != "" && !strings.Contains(globalStyle, "风格参考图") {
			globalStyle = strings.TrimSpace(globalStyle) + "\n\n" + reference
		}
		tasks = append(tasks, CanvasPackageTask{
			ID: canvasTaskID(CanvasPackageStageP4Videos, segment.Code), Kind: "video",
			Code: segment.Code, Title: "分段视频 · " + segment.Code, Prompt: segment.Prompt,
			Category: "material", Tags: []string{"P4", "分段视频"}, AspectRatio: snapshot.AspectRatio,
			Seconds: segment.DurationSeconds, ReferenceCodes: uniqueCanvasCodes(referenceCodes...),
			GlobalStylePrompt: globalStyle,
			SourceKind:        model.ManjuProductionArtifactFeed, Group: "segment",
		})
	}
	return &CanvasPackage{
		Version: 1, ProjectID: projectID, Stage: CanvasPackageStageP4Videos, ProductionRunID: runID,
		SourceArtifact: model.ManjuProductionArtifactFeed, Title: "P4 分段视频", Tasks: tasks,
	}, nil
}

func (s *Service) currentProductionStoryboardCodes(runID, artifactID string) (map[string]bool, error) {
	result := make(map[string]bool)
	images, err := s.repo.ManjuProductionImages(runID)
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		if image.Status == ProductionImageStatusSuperseded ||
			image.Status != ProductionImageStatusReady ||
			image.ArtifactID != artifactID ||
			strings.TrimSpace(image.ResourceID) == "" {
			continue
		}
		key := p3SegmentKey(image.SegmentCode)
		if key == "" {
			continue
		}
		resource, err := s.repo.ResourceForUser(image.CreatedBy, image.ResourceID)
		if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil ||
			resource.Kind != "image" || resource.Status != model.ResourceStatusReady {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[key] = true
	}
	return result, nil
}

func (s *Service) latestCanvasLockSnapshot(projectID string) LockSnapshot {
	receipt, err := s.repo.LatestManjuLockReceipt(projectID)
	if err != nil || receipt == nil {
		return LockSnapshot{AspectRatio: "9:16"}
	}
	var snapshot LockSnapshot
	if json.Unmarshal([]byte(receipt.PayloadJSON), &snapshot) != nil {
		return LockSnapshot{AspectRatio: "9:16"}
	}
	if err := hydrateLockStylePrompts(&snapshot); err != nil {
		return LockSnapshot{AspectRatio: "9:16"}
	}
	return snapshot
}

func parseP2CanvasTasks(content, assetMode, aspect string, stylePrompt ...string) ([]CanvasPackageTask, error) {
	assetBlocks, err := p2CanvasAssetBlocks(content)
	if err != nil {
		return nil, err
	}
	figureAssets := parseCanvasFigureAssetMap(content)
	codes := make([]string, 0, len(assetBlocks))
	for code := range assetBlocks {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool {
		return canvasFigureNumberValue(codes[i]) < canvasFigureNumberValue(codes[j])
	})
	tasks := make([]CanvasPackageTask, 0, len(codes))
	stylePrefix := ""
	if len(stylePrompt) > 0 {
		stylePrefix = stylePrompt[0]
	}
	for _, code := range codes {
		if code == "" {
			return nil, fmt.Errorf("%w: P2 画布任务缺少有效图N编号", ErrCanvasPackageUnready)
		}
		prompt, ok := assetBlocks[code]
		if !ok {
			return nil, fmt.Errorf("%w: P2 任务 %s 缺少对应的 V7.4.26 图N资产块", ErrCanvasPackageUnready, code)
		}
		title := p2CanvasAssetBlockTitle(prompt, code)
		assetCode := normalizeCanvasImportReferenceCode(strings.TrimSpace(figureAssets[code]))
		category := p2CanvasCategoryFromBlock(title)
		if assetCode == "" {
			assetCode = p2CanvasAssetCodeFromBlock(prompt, title, category, code)
		}
		if category == "other" {
			category = normalizeCanvasAssetCategory("", assetCode)
		}
		tasks = append(tasks, CanvasPackageTask{
			ID:   canvasTaskID(CanvasPackageStageP2Assets, code+"|"+assetCode),
			Kind: "image", Code: code, AssetCode: assetCode,
			Title:  fallbackValue(title, fallbackValue(assetCode, code)),
			Prompt: prependStylePrompt(stylePrefix, prompt), Category: category, Tags: uniqueCanvasCodes("P2", category),
			AspectRatio: fallbackValue(aspect, "16:9"),
			SourceKind:  "ASSET_DELIVERY_" + assetMode, Group: category,
		})
	}
	if len(tasks) == 0 {
		return nil, ErrCanvasPackageUnready
	}
	return tasks, nil
}

func p2CanvasAssetBlockTitle(block, fallback string) string {
	firstLine := strings.TrimSpace(strings.SplitN(block, "\n", 2)[0])
	match := canvasAssetHeadingTitle.FindStringSubmatch(firstLine)
	if len(match) == 2 && strings.TrimSpace(match[1]) != "" {
		return strings.TrimSpace(match[1])
	}
	return fallback
}

func p2CanvasCategoryFromBlock(title string) string {
	switch {
	case strings.Contains(title, "群像"):
		return "group"
	case strings.Contains(title, "角色"):
		return "character"
	case strings.Contains(title, "场景"):
		return "environment"
	case strings.Contains(title, "道具"):
		return "prop"
	default:
		return "other"
	}
}

func p2CanvasAssetCodeFromBlock(block, title, category, figure string) string {
	if code := canvasBlockAssetCode.FindString(block); code != "" {
		return code
	}
	name := strings.TrimSpace(strings.SplitN(title, "（", 2)[0])
	if name == "" {
		name = figure
	}
	switch category {
	case "character":
		return "CHR-" + name
	case "environment":
		return "SCN-" + name
	case "prop":
		return "PRP-" + name
	case "group":
		return "GRP-" + name
	default:
		return figure
	}
}

func canvasFigureNumberValue(code string) int {
	match := canvasImageNumber.FindStringSubmatch(code)
	if len(match) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(strings.Map(func(char rune) rune {
		if char >= '０' && char <= '９' {
			return '0' + (char - '０')
		}
		return char
	}, match[1]))
	return value
}

func p2CanvasAssetBlocks(content string) (map[string]string, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	type assetHeading struct {
		line  int
		level int
		code  string
	}
	headings := make([]assetHeading, 0)
	for index, line := range lines {
		match := canvasAssetBlockHeading.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		code := canonicalCanvasFigureCode("图" + match[2])
		if code == "" {
			continue
		}
		headings = append(headings, assetHeading{line: index, level: len(match[1]), code: code})
	}
	if len(headings) == 0 {
		return nil, fmt.Errorf("%w: 未找到 V7.4.26 图N资产块", ErrCanvasPackageUnready)
	}
	blocks := make(map[string]string, len(headings))
	for _, heading := range headings {
		end := len(lines)
		for next := heading.line + 1; next < len(lines); next++ {
			match := canvasMarkdownHeadingLevel.FindStringSubmatch(lines[next])
			if len(match) == 2 && len(match[1]) <= heading.level {
				end = next
				break
			}
		}
		block := strings.TrimSpace(strings.Join(lines[heading.line:end], "\n"))
		if existing, exists := blocks[heading.code]; exists && existing != block {
			return nil, fmt.Errorf("%w: V7.4.26 资产块 %s 重复", ErrCanvasPackageUnready, heading.code)
		}
		blocks[heading.code] = block
	}
	return blocks, nil
}

func canonicalCanvasFigureCode(value string) string {
	match := canvasImageNumber.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 2 {
		return ""
	}
	digits := strings.Map(func(char rune) rune {
		if char >= '０' && char <= '９' {
			return '0' + (char - '０')
		}
		return char
	}, match[1])
	if digits == "" {
		return ""
	}
	return "图" + digits
}

type plotGridScene struct {
	Code        string
	Requirement string
	Grid        string
}

func parseP3PlotChecklist(rows []p3DeliveryChecklistRow) ([]plotGridScene, []CanvasPackageExemption, error) {
	seen := make(map[string]bool)
	scenes := make([]plotGridScene, 0)
	exemptions := make([]CanvasPackageExemption, 0)
	for _, row := range rows {
		match := canvasPlotSceneExactPattern.FindStringSubmatch(strings.TrimSpace(row.PlotCoverage))
		if len(match) != 2 {
			return nil, nil, fmt.Errorf(
				"%w: %s 的剧情分镜格/覆盖必须是精确的场N",
				ErrP3StoryboardContract,
				row.Segment.Code,
			)
		}
		code := "场" + match[1]
		if seen[code] {
			continue
		}
		seen[code] = true
		exempt, reason, err := parseP3PlotExemption(row.PlotCoverage, row.Explanation, code)
		if err != nil {
			return nil, nil, err
		}
		if exempt {
			exemptions = append(exemptions, CanvasPackageExemption{
				Kind: "plot", Code: code, Reason: reason,
			})
			continue
		}
		grid := "4格(2×2)"
		if strings.Contains(row.PlotCoverage, "九") {
			grid = "9格(3×3)"
		} else if strings.Contains(row.PlotCoverage, "六") {
			grid = "6格(2×3)"
		}
		scenes = append(scenes, plotGridScene{
			Code:        code,
			Requirement: strings.TrimSpace(row.PlotCoverage + " | " + row.Explanation),
			Grid:        grid,
		})
	}
	return scenes, exemptions, nil
}

func parseP3PlotExemption(coverage, explanation, code string) (bool, string, error) {
	compact := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(coverage)
	negative := false
	for _, token := range []string{"无豁免", "无需豁免", "未豁免", "不豁免", "没有豁免"} {
		if strings.Contains(compact, token) {
			negative = true
			break
		}
	}
	if !strings.Contains(coverage, "⛔豁免") {
		if !negative && (strings.Contains(compact, "豁免") || strings.Contains(compact, "⛔")) {
			return false, "", fmt.Errorf(
				"%w: %s 的剧情分镜格豁免必须使用完整 token ⛔豁免",
				ErrP3StoryboardContract,
				code,
			)
		}
		return false, "", nil
	}
	if negative {
		return false, "", fmt.Errorf("%w: %s 的剧情分镜格豁免同时包含否定表述", ErrP3StoryboardContract, code)
	}
	combined := coverage + " | " + explanation
	match := p3ExemptionReasonPattern.FindStringSubmatch(combined)
	if len(match) < 2 {
		return false, "", fmt.Errorf(
			"%w: %s 的剧情分镜格豁免必须写明非空理由",
			ErrP3StoryboardContract,
			code,
		)
	}
	reason := p3NormalizeExemptionReason(match[1])
	if p3ExemptionReasonPlaceholder(reason) {
		return false, "", fmt.Errorf(
			"%w: %s 的剧情分镜格豁免理由为空或占位",
			ErrP3StoryboardContract,
			code,
		)
	}
	return true, reason, nil
}

func buildPlotGridCanvasPrompt(content, code, requirement, grid string, snapshot LockSnapshot) string {
	var out strings.Builder
	out.WriteString("基于已锁定的 P3 分镜成果生成一张剧情分镜格（推演宫格）。\n")
	fmt.Fprintf(&out, "【场景】%s\n【格数】%s\n【画幅】%s\n", code, grid, fallbackValue(snapshot.AspectRatio, "9:16"))
	fmt.Fprintf(&out, "【交付清单要求】%s\n", requirement)
	if table := markdownSection(content, "## 分镜总表"); table != "" {
		out.WriteString("【P3 分镜总表】\n")
		out.WriteString(table)
		out.WriteString("\n")
	}
	out.WriteString("【成图要求】固定人物身份、服装、场景、道具与画风；按四/六/九宫格连续叙事，不新增未出现在 P3 中的人物或道具，不生成水印。")
	return out.String()
}

func removeCanvasTaskStyleLine(prompt string) string {
	lines := strings.Split(prompt, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "统一要求：全体面板统一画风") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func styleReferenceExecutionRule(analysis string) string {
	analysis = strings.TrimSpace(sanitizeStyleNames(analysis))
	if analysis == "" {
		return ""
	}
	return "【风格参考图 · 全局执行规则】\n" +
		"参考分析：" + analysis + "\n" +
		"只迁移光线、主辅色、材质、镜头、颗粒与后期质感；禁止复刻构图、人物身份、服装款式、文字和具体场景。"
}

func canvasTaskID(stage, code string) string {
	sum := sha256.Sum256([]byte(stage + "\x00" + code))
	return stage + "-" + hex.EncodeToString(sum[:8])
}

func canvasPackageHash(pkg CanvasPackage) string {
	pkg.Hash = ""
	raw, _ := json.Marshal(pkg)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func uuidForCanvasImport(string, string) string { return uuid.NewString() }

func canvasImportView(item model.ManjuCanvasImport) CanvasImportView {
	return CanvasImportView{
		ID: item.ID, ProjectID: item.ProjectID, Stage: item.Stage, AssetRunID: item.AssetRunID,
		ProductionRunID: item.ProductionRunID, CanvasID: item.CanvasID, PackageHash: item.PackageHash,
		TaskCount: item.TaskCount, Status: item.Status, CreatedBy: item.CreatedBy,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (s *Service) canvasFigureAssetMap(assetRunID string) map[string]string {
	result := make(map[string]string)
	assetRunID = strings.TrimSpace(assetRunID)
	if assetRunID == "" {
		return result
	}
	stages, err := s.repo.ManjuAssetStages(assetRunID)
	if err != nil {
		return result
	}
	artifacts, err := s.repo.ManjuAssetArtifacts(assetRunID)
	if err != nil {
		return result
	}
	byID := make(map[string]model.ManjuAssetArtifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	for _, stage := range stages {
		if stage.Key != AssetStageCompile || strings.TrimSpace(stage.ArtifactID) == "" {
			continue
		}
		if artifact, ok := byID[stage.ArtifactID]; ok {
			for figure, code := range parseCanvasFigureAssetMap(artifact.Content) {
				result[figure] = code
			}
		}
	}
	if len(result) > 0 {
		return result
	}
	for _, artifact := range artifacts {
		for figure, code := range parseCanvasFigureAssetMap(artifact.Content) {
			result[figure] = code
		}
	}
	return result
}

func parseCanvasFigureAssetMap(content string) map[string]string {
	result := make(map[string]string)
	add := func(figure, code string) {
		figure = normalizeCanvasFigureKey(figure)
		code = strings.TrimSpace(code)
		if figure == "" || code == "" || !canvasVisualCodePattern.MatchString(code) {
			return
		}
		result[figure] = code
	}
	if start := strings.Index(content, canvasTaskBlockStart); start >= 0 {
		raw := content[start+len(canvasTaskBlockStart):]
		if end := strings.Index(raw, canvasTaskBlockEnd); end >= 0 {
			var manifest canvasPackageJSON
			if json.Unmarshal([]byte(strings.TrimSpace(raw[:end])), &manifest) == nil {
				for _, task := range manifest.Tasks {
					add(task.Code, task.AssetCode)
				}
			}
		}
	}
	for _, match := range canvasFigureAssetPattern.FindAllStringSubmatch(content, -1) {
		if len(match) >= 3 {
			add("图"+match[1], match[2])
		}
	}
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := markdownCells(line)
		figure := ""
		code := ""
		for _, cell := range cells {
			if match := canvasImageNumber.FindStringSubmatch(strings.TrimSpace(cell)); len(match) > 1 && figure == "" {
				figure = "图" + match[1]
			}
			if match := canvasVisualCodePattern.FindString(cell); match != "" && code == "" {
				code = match
			}
		}
		add(figure, code)
	}
	return result
}

func normalizeCanvasFigureKey(value string) string {
	value = strings.TrimSpace(value)
	match := canvasImageNumber.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	number := strings.NewReplacer(
		"０", "0", "１", "1", "２", "2", "３", "3", "４", "4",
		"５", "5", "６", "6", "７", "7", "８", "8", "９", "9",
	).Replace(match[1])
	return "图" + number
}

func storyboardCanvasReferenceCodes(values []string, figureAssets map[string]string) []string {
	result := make([]string, 0)
	seen := make(map[string]bool)
	usedFigureAsset := false
	add := func(code string) {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			return
		}
		seen[code] = true
		result = append(result, code)
	}
	for _, value := range values {
		for _, code := range canvasVisualCodePattern.FindAllString(value, -1) {
			add(normalizeStoryboardVisualAssetCode(code))
		}
		for _, figure := range canvasImageNumber.FindAllString(value, -1) {
			if code := figureAssets[normalizeCanvasFigureKey(figure)]; code != "" {
				add(code)
				usedFigureAsset = true
			}
		}
	}
	if !usedFigureAsset {
		sort.Strings(result)
	}
	return result
}

func gridAssetCodes(values, fallback []string) []string {
	result := visualCanvasCodes(values...)
	if len(result) == 0 {
		return visualCanvasCodes(fallback...)
	}
	return result
}

func visualCanvasCodes(values ...string) []string {
	matches := make([]string, 0)
	for _, value := range values {
		matches = append(matches, canvasVisualCodePattern.FindAllString(value, -1)...)
		for _, match := range canvasNamespaceCodePattern.FindAllStringSubmatch(value, -1) {
			if len(match) < 3 {
				continue
			}
			matches = append(matches, strings.ToUpper(match[1])+"-"+strings.TrimSpace(match[2]))
		}
	}
	return uniqueCanvasCodes(matches...)
}

func uniqueCanvasCodes(values ...string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeCanvasAssetCategory(category, assetCode string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "character", "environment", "prop", "material", "group", "other":
		return strings.ToLower(strings.TrimSpace(category))
	}
	switch {
	case strings.HasPrefix(assetCode, "CHR-"):
		return "character"
	case strings.HasPrefix(assetCode, "SCN-"):
		return "environment"
	case strings.HasPrefix(assetCode, "PRP-"):
		return "prop"
	case strings.HasPrefix(assetCode, "AUD-"):
		return "material"
	case strings.HasPrefix(assetCode, "GRP-"):
		return "group"
	default:
		return "other"
	}
}

type canvasPayloadNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Metadata map[string]any `json:"metadata"`
}

type canvasPayload struct {
	Nodes []canvasPayloadNode `json:"nodes"`
}

func canvasImportReferences(payload, stage string) []CanvasImportReferenceView {
	var document canvasPayload
	if json.Unmarshal([]byte(payload), &document) != nil {
		return nil
	}
	result := make([]CanvasImportReferenceView, 0, len(document.Nodes))
	indexByCode := make(map[string]int)
	upsert := func(reference CanvasImportReferenceView) {
		reference.Code = strings.TrimSpace(reference.Code)
		reference.Title = strings.TrimSpace(reference.Title)
		if reference.Code == "" {
			return
		}
		key := normalizeCanvasImportReferenceCode(reference.Code)
		if index, found := indexByCode[key]; found {
			current := &result[index]
			if reference.Content != "" {
				current.Content = reference.Content
			}
			if reference.StorageKey != "" {
				current.StorageKey = reference.StorageKey
			}
			if reference.AssetCode != "" {
				current.AssetCode = reference.AssetCode
			}
			if reference.AssetID != "" {
				current.AssetID = reference.AssetID
			}
			if reference.TaskID != "" {
				current.TaskID = reference.TaskID
			}
			if reference.Prompt != "" {
				current.Prompt = reference.Prompt
			}
			if reference.Category != "" {
				current.Category = reference.Category
			}
			if reference.Kind != "" {
				current.Kind = reference.Kind
			}
			if reference.Title != "" && reference.Title != reference.Code &&
				(current.Title == "" || current.Title == current.Code || strings.Contains(current.Title, "待补参考")) {
				current.Title = reference.Title
			}
			return
		}
		indexByCode[key] = len(result)
		if reference.Title == "" {
			reference.Title = reference.Code
		}
		result = append(result, reference)
	}
	for _, node := range document.Nodes {
		nodeStage, _ := node.Metadata["manjuStage"].(string)
		taskID, _ := node.Metadata["manjuTaskId"].(string)
		if nodeStage != stage || strings.TrimSpace(taskID) == "" || (node.Type != "image" && node.Type != "video") {
			continue
		}
		content, _ := node.Metadata["content"].(string)
		storageKey, _ := node.Metadata["storageKey"].(string)
		prompt, _ := node.Metadata["prompt"].(string)
		if strings.TrimSpace(prompt) == "" {
			prompt, _ = node.Metadata["composerContent"].(string)
		}
		if strings.TrimSpace(content) == "" && strings.TrimSpace(storageKey) == "" {
			continue
		}
		assetCode, _ := node.Metadata["manjuAssetCode"].(string)
		assetID, _ := node.Metadata["assetId"].(string)
		taskCode, _ := node.Metadata["manjuTaskCode"].(string)
		code := fallbackValue(assetCode, taskCode)
		if strings.TrimSpace(code) == "" {
			continue
		}
		category, _ := node.Metadata["assetCategory"].(string)
		upsert(CanvasImportReferenceView{
			Code: code, AssetCode: assetCode, AssetID: assetID, TaskID: taskID, Title: fallbackValue(node.Title, code),
			Prompt: strings.TrimSpace(prompt), Content: content, StorageKey: storageKey, Category: category, Kind: node.Type,
		})
	}
	for _, node := range document.Nodes {
		nodeStage, _ := node.Metadata["manjuStage"].(string)
		batchTable, _ := node.Metadata["batchTable"].(map[string]any)
		if batchTable == nil {
			continue
		}
		assetTable, _ := batchTable["manjuAssetTable"].(map[string]any)
		storyboardTable, _ := batchTable["manjuStoryboardTable"].(map[string]any)
		if assetTable == nil && storyboardTable == nil {
			continue
		}
		sourceStage := fallbackValue(stringMapValue(assetTable, "sourceStage"), stringMapValue(storyboardTable, "sourceStage"))
		if nodeStage != stage && sourceStage != stage {
			continue
		}
		rows, _ := batchTable["rows"].([]any)
		for _, rawRow := range rows {
			row, _ := rawRow.(map[string]any)
			if asset, _ := row["manjuAsset"].(map[string]any); asset != nil {
				code := stringMapValue(asset, "code")
				if code != "" {
					kind := stringMapValue(asset, "kind")
					upsert(CanvasImportReferenceView{
						Code: code, AssetCode: code, TaskID: stringMapValue(asset, "taskId"),
						Title:    fallbackValue(stringMapValue(asset, "name"), code),
						Prompt:   stringMapValue(row, "prompt"),
						Category: canvasAssetCategoryFromKind(kind), Kind: "image",
					})
				}
			}
			if storyboard, _ := row["manjuStoryboard"].(map[string]any); storyboard != nil {
				code := stringMapValue(storyboard, "code")
				if code != "" {
					upsert(CanvasImportReferenceView{
						Code: code, TaskID: stringMapValue(storyboard, "taskId"),
						Title:    fallbackValue(stringMapValue(storyboard, "title"), code),
						Category: "material", Kind: "image",
					})
				}
			}
		}
	}
	return result
}

func mergeCanvasImportReferencesWithP2Tasks(references []CanvasImportReferenceView, tasks []CanvasPackageTask) []CanvasImportReferenceView {
	result := append([]CanvasImportReferenceView(nil), references...)
	indexByCode := make(map[string]int, len(result)+len(tasks))
	for index, reference := range result {
		indexByCode[normalizeCanvasImportReferenceCode(reference.Code)] = index
	}
	for _, task := range tasks {
		code := strings.TrimSpace(task.AssetCode)
		if code == "" {
			code = strings.TrimSpace(task.Code)
		}
		if code == "" || !canvasVisualCodePattern.MatchString(code) {
			continue
		}
		key := normalizeCanvasImportReferenceCode(code)
		index, found := indexByCode[key]
		if !found {
			index = len(result)
			indexByCode[key] = index
			result = append(result, CanvasImportReferenceView{
				Code: code, AssetCode: code, TaskID: task.ID, Title: fallbackValue(task.Title, code),
				Prompt: task.Prompt, Category: task.Category, Kind: "image",
			})
			continue
		}
		current := &result[index]
		if current.AssetCode == "" {
			current.AssetCode = code
		}
		if task.ID != "" {
			current.TaskID = task.ID
		}
		if task.Title != "" {
			current.Title = task.Title
		}
		if strings.TrimSpace(task.Prompt) != "" {
			current.Prompt = strings.TrimSpace(task.Prompt)
		}
		if task.Category != "" {
			current.Category = task.Category
		}
		current.Kind = "image"
	}
	sort.SliceStable(result, func(left, right int) bool {
		return result[left].Code < result[right].Code
	})
	return result
}

func normalizeCanvasImportReferenceCode(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(strings.ToUpper(code), "CNH-") {
		return "CHR-" + code[len("CNH-"):]
	}
	return code
}

func stringMapValue(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return strings.TrimSpace(value)
}

func canvasAssetCategoryFromKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "character":
		return "character"
	case "scene", "environment":
		return "environment"
	case "prop":
		return "prop"
	default:
		return ""
	}
}

type canvasImportedResource struct {
	ID   string
	Kind string
}

func canvasImportResources(payload string, tasks []CanvasPackageTask, stage string) ([]canvasImportedResource, error) {
	var document canvasPayload
	if err := json.Unmarshal([]byte(payload), &document); err != nil {
		return nil, ErrCanvasImportNotReady
	}
	byTask := make(map[string]struct {
		metadata map[string]any
		nodeType string
	})
	for _, node := range document.Nodes {
		nodeStage, _ := node.Metadata["manjuStage"].(string)
		taskID, _ := node.Metadata["manjuTaskId"].(string)
		if nodeStage != stage || strings.TrimSpace(taskID) == "" || (node.Type != "image" && node.Type != "video") {
			continue
		}
		byTask[taskID] = struct {
			metadata map[string]any
			nodeType string
		}{metadata: node.Metadata, nodeType: node.Type}
	}
	result := make([]canvasImportedResource, 0, len(tasks))
	for _, task := range tasks {
		entry, ok := byTask[task.ID]
		if !ok || entry.nodeType != task.Kind {
			return nil, ErrCanvasImportNotReady
		}
		resourceID := canvasNodeResourceID(entry.metadata)
		if resourceID == "" {
			return nil, ErrCanvasImportNotReady
		}
		result = append(result, canvasImportedResource{ID: resourceID, Kind: task.Kind})
	}
	return result, nil
}

func canvasNodeResourceID(metadata map[string]any) string {
	storageKey, _ := metadata["storageKey"].(string)
	content, _ := metadata["content"].(string)
	for _, value := range []string{storageKey, content} {
		match := canvasResourceKeyPattern.FindStringSubmatch(value)
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			return strings.TrimSuffix(strings.TrimSpace(match[1]), "/file")
		}
	}
	return ""
}
