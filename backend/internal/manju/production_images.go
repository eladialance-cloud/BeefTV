package manju

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ProductionImageStatusQueued     = model.ManjuProductionImageStatusQueued
	ProductionImageStatusRunning    = model.ManjuProductionImageStatusRunning
	ProductionImageStatusReady      = model.ManjuProductionImageStatusReady
	ProductionImageStatusFailed     = model.ManjuProductionImageStatusFailed
	ProductionImageStatusSuperseded = model.ManjuProductionImageStatusSuperseded
)

var storyboardAssetModifierPattern = regexp.MustCompile(`^((?:CHR|SCN|PRP)-[0-9０-９]+)(?:局部|特写|近景|中景|远景|全景|半身|面部|手部|细节|群像)$`)

var (
	ErrStoryboardImagesNotReady = errors.New("manju storyboard images are not ready")
	ErrStoryboardImageInvalid   = errors.New("invalid manju storyboard image batch")
	ErrStoryboardImageStarted   = errors.New("manju storyboard image batch already started")
)

const storyboardSheetAspectRatio = "16:9"

type ProductionImageView struct {
	ID          string    `json:"id"`
	RunID       string    `json:"runId"`
	ArtifactID  string    `json:"artifactId,omitempty"`
	Position    int       `json:"position"`
	SegmentCode string    `json:"segmentCode"`
	ShotLabel   string    `json:"shotLabel,omitempty"`
	Attempt     int       `json:"attempt"`
	Status      string    `json:"status"`
	TaskID      string    `json:"taskId,omitempty"`
	ResourceID  string    `json:"resourceId,omitempty"`
	URL         string    `json:"url,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type storyboardImagePlan struct {
	ID          string
	Position    int
	SegmentCode string
	ShotLabel   string
	Prompt      string
	Attempt     int
}

type storyboardImageSegment struct {
	Code   string
	Title  string
	Shots  []string
	Phases []string
	Camera []string
	Block  []string
	Assets []string
	Panels []storyboardImagePanel
}

type storyboardImagePanel struct {
	ID           string
	Phase        string
	Camera       string
	Block        string
	Scene        string
	Action       string
	Assets       string
	BoundaryNote string
}

// StartProductionStoryboardImages creates one real canvas_image task per P3
// storyboard segment. It never marks an image ready directly; READY is written
// only by reconciliation after a durable generation output and resource owner
// check succeed.
func (s *Service) StartProductionStoryboardImages(actorID, projectID, runID string) (*ProductionRunDetail, error) {
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

	var plans []storyboardImagePlan
	var providerContext struct {
		ChannelID        string
		ModelKey         string
		AssetMode        string
		Aspect           string
		Style            string
		StyleParameters  string
		StyleImagePrompt string
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
		if run.Status != model.ManjuProductionRunStatusActive || run.CurrentStage != ProductionStageStoryboard {
			return ErrProductionStageNotReady
		}
		stage, err := tx.ManjuProductionStage(runID, ProductionStageStoryboard)
		if err != nil {
			return err
		}
		if stage == nil || stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation || strings.TrimSpace(stage.ArtifactID) == "" {
			return ErrProductionStageNotReady
		}
		binding, err := tx.ManjuProjectModelBinding(projectID)
		if err != nil {
			return err
		}
		if binding == nil || strings.TrimSpace(binding.ImageChannelID) == "" || strings.TrimSpace(binding.ImageModelKey) == "" {
			return ErrInvalidModelBinding
		}
		providerContext.ChannelID = binding.ImageChannelID
		providerContext.ModelKey = binding.ImageModelKey

		recipe, err := tx.LatestManjuLockReceipt(projectID)
		if err != nil {
			return err
		}
		if recipe == nil || recipe.Status != model.ManjuLockStatusLocked || recipe.Revision != run.LockRevision {
			return ErrProductionStageNotReady
		}
		var lockSnapshot struct {
			AssetMode        string   `json:"assetMode"`
			Aspect           string   `json:"aspectRatio"`
			Style            string   `json:"styleBase"`
			StyleCardID      string   `json:"styleCardId"`
			StyleCardName    string   `json:"styleCardName"`
			StyleCustom      string   `json:"styleCustomDescription"`
			StyleParameters  string   `json:"stylePromptParameters"`
			StyleVisual      string   `json:"styleVisualPrompt"`
			StyleMotion      string   `json:"styleMotionParameters"`
			StyleForbidden   []string `json:"styleForbiddenItems"`
			StyleImagePrompt string   `json:"styleImagePrompt"`
		}
		if err := json.Unmarshal([]byte(recipe.PayloadJSON), &lockSnapshot); err != nil {
			return ErrProductionStageNotReady
		}
		lockSnapshotForStyle := LockSnapshot{
			StyleBase:              lockSnapshot.Style,
			StyleCustomDescription: lockSnapshot.StyleCustom,
			StylePromptParameters:  lockSnapshot.StyleParameters,
			StyleVisualPrompt:      lockSnapshot.StyleVisual,
			StyleMotionParameters:  lockSnapshot.StyleMotion,
			StyleForbiddenItems:    lockSnapshot.StyleForbidden,
			StyleImagePrompt:       lockSnapshot.StyleImagePrompt,
			StyleReferenceAnalysis: "",
			StyleCardID:            lockSnapshot.StyleCardID,
			StyleCardName:          lockSnapshot.StyleCardName,
		}
		if err := hydrateLockStylePrompts(&lockSnapshotForStyle); err != nil {
			return ErrProductionStageNotReady
		}
		if strings.TrimSpace(lockSnapshot.StyleImagePrompt) == "" {
			lockSnapshot.StyleImagePrompt = lockSnapshotForStyle.StyleImagePrompt
		}
		assetRun, err := tx.ManjuAssetRun(projectID, run.AssetRunID)
		if err != nil {
			return err
		}
		if assetRun == nil {
			return ErrProductionAssetNotReady
		}
		providerContext.AssetMode = assetRun.AssetMode
		providerContext.Aspect = lockSnapshot.Aspect
		providerContext.Style = lockSnapshot.Style
		providerContext.StyleParameters = sanitizeStyleNames(lockSnapshot.StyleParameters)
		providerContext.StyleImagePrompt = lockSnapshot.StyleImagePrompt

		artifactContent, err := productionArtifactContent(tx, runID, stage.ArtifactID)
		if err != nil {
			return err
		}
		segments, err := parseStoryboardImageSegments(artifactContent)
		if err != nil || len(segments) == 0 {
			return ErrStoryboardImageInvalid
		}
		candidateStateJSON, err := previewManjuProjectStatePatch(
			tx,
			projectID,
			access.Project.Name,
			run.LockRevision,
			"production:director_storyboard",
			artifactContent,
			actorID,
		)
		if err != nil {
			return fmt.Errorf("P3_REGENERATION_REQUIRED: 无法解析故事板交付清单: %w", err)
		}
		var state map[string]any
		if err := json.Unmarshal([]byte(candidateStateJSON), &state); err != nil {
			return fmt.Errorf("P3_REGENERATION_REQUIRED: 无法读取 P3 project_state: %w", err)
		}
		required, err := p3RequiredSegments(state)
		if err != nil {
			return err
		}
		if err := validateP3StoryboardTable(artifactContent, required); err != nil {
			return err
		}
		requirements, err := p3DeliveryRequirements(artifactContent, required)
		if err != nil {
			return err
		}
		segments = slices.DeleteFunc(segments, func(segment storyboardImageSegment) bool {
			requirement := requirements[p3SegmentKey(segment.Code)]
			return requirement.Exempt
		})
		if len(segments) == 0 {
			return nil
		}

		existing, err := reconcileProductionStoryboardImageLineage(
			tx,
			projectID,
			runID,
			*stage,
			required,
			requirements,
			nil,
		)
		if err != nil {
			return err
		}
		now := time.Now()
		eligibleCodes := make(map[string]bool, len(segments))
		for _, segment := range segments {
			eligibleCodes[p3SegmentKey(segment.Code)] = true
		}
		existingCodes := make(map[string]bool)
		nextPosition := 1
		for index := range existing {
			image := &existing[index]
			if image.Position >= nextPosition {
				nextPosition = image.Position + 1
			}
			if image.Status == ProductionImageStatusSuperseded {
				continue
			}
			key := p3SegmentKey(image.SegmentCode)
			if !eligibleCodes[key] {
				continue
			}
			existingCodes[key] = true
			if image.Status == ProductionImageStatusQueued || image.Status == ProductionImageStatusRunning || image.Status == ProductionImageStatusReady {
				continue
			}
			image.Attempt++
			image.Status = ProductionImageStatusQueued
			image.ArtifactID = stage.ArtifactID
			image.TaskID = ""
			image.ResourceID = ""
			image.URL = ""
			image.Error = ""
			image.UpdatedAt = now
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
			plans = append(plans, storyboardImagePlan{
				ID: image.ID, Position: image.Position, SegmentCode: image.SegmentCode,
				ShotLabel: image.ShotLabel, Prompt: image.Prompt, Attempt: image.Attempt,
			})
		}

		images := make([]model.ManjuProductionImage, 0, len(segments))
		for _, segment := range segments {
			if existingCodes[p3SegmentKey(segment.Code)] {
				continue
			}
			prompt := buildStoryboardImagePrompt(segment, providerContext.AssetMode, providerContext.Aspect, providerContext.Style, providerContext.StyleParameters, providerContext.StyleImagePrompt)
			image := model.ManjuProductionImage{
				ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
				ArtifactID: stage.ArtifactID, Position: nextPosition,
				SegmentCode: segment.Code, ShotLabel: storyboardShotLabel(segment),
				Prompt: prompt, Attempt: 1, Status: ProductionImageStatusQueued,
				CreatedBy: actorID, CreatedAt: now, UpdatedAt: now,
			}
			nextPosition++
			images = append(images, image)
			plans = append(plans, storyboardImagePlan{
				ID: image.ID, Position: image.Position, SegmentCode: image.SegmentCode,
				ShotLabel: image.ShotLabel, Prompt: image.Prompt, Attempt: image.Attempt,
			})
		}
		if len(images) == 0 {
			return nil
		}
		return tx.CreateManjuProductionImages(images)
	})
	if err != nil {
		return nil, err
	}

	for _, plan := range plans {
		request := localtask.CreateRequest{
			ProjectID: projectID,
			Type:      "canvas_image",
			Operation: "manju_storyboard_image",
			Prompt:    plan.Prompt,
			Model:     providerContext.ModelKey,
			Input: map[string]any{
				"mode":   "image",
				"prompt": plan.Prompt,
				"config": map[string]any{
					"channelId":   providerContext.ChannelID,
					"model":       providerContext.ModelKey,
					"aspectRatio": storyboardSheetAspectRatio,
				},
				"metadata": map[string]any{
					"clientOperationId":         fmt.Sprintf("manju:production:%s:storyboard-image:%s:%d", runID, plan.ID, plan.Attempt),
					"manjuProjectId":            projectID,
					"manjuProductionRunId":      runID,
					"manjuProductionStageKey":   ProductionStageStoryboard,
					"manjuStoryboardImageJobId": plan.ID,
					"manjuStoryboardSegment":    plan.SegmentCode,
				},
			},
		}
		task, createErr := s.scriptTaskCreate(actorID, request)
		if createErr == nil && (task == nil || strings.TrimSpace(task.ID) == "") {
			createErr = errors.New("图片任务服务没有返回任务 ID")
		}
		if err := s.repo.Transaction(func(tx *repository.Repository) error {
			image, err := tx.ManjuProductionImage(runID, plan.ID)
			if err != nil || image == nil {
				return err
			}
			if createErr != nil {
				image.Status = ProductionImageStatusFailed
				image.Error = strings.TrimSpace(createErr.Error())
			} else {
				image.Status = ProductionImageStatusRunning
				image.TaskID = task.ID
				image.Error = ""
			}
			return tx.SaveManjuProductionImage(image)
		}); err != nil {
			return nil, err
		}
	}
	return s.ProductionRunDetail(actorID, projectID, runID)
}

func buildStoryboardImagePrompt(segment storyboardImageSegment, assetMode, aspect, styleBase, styleParameters string, strongStylePrompt ...string) string {
	panels := segment.Panels
	if len(panels) == 0 {
		count := len(segment.Phases)
		if len(segment.Camera) > count {
			count = len(segment.Camera)
		}
		if len(segment.Block) > count {
			count = len(segment.Block)
		}
		for index := 0; index < count; index++ {
			panels = append(panels, storyboardImagePanel{
				ID:     markdownCell(segment.Shots, index),
				Phase:  markdownCell(segment.Phases, index),
				Camera: markdownCell(segment.Camera, index),
				Block:  markdownCell(segment.Block, index),
			})
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "生成一张 16:9 横版的故事板图（CLEAN_GRID 干净版），整张图分为 %d 个等大的面板（%s），面板之间用细白线分隔，无黑边、无留白、画面填满。\n", len(panels), storyboardSheetLayout(len(panels)))
	fmt.Fprintf(&out, "本图为%s的故事板，人物、场景和道具必须与当前任务连线的 P2 资产图完全一致；各面板资产引用以逐格字段为准，不得从其他段补资产。\n",
		storyboardSegmentLabel(segment))
	for index, panel := range panels {
		phase := fallbackValue(panel.Phase, "动作相位")
		if strings.TrimSpace(panel.ID) != "" {
			fmt.Fprintf(&out, "面板 %d / %s（%s）：", index+1, panel.ID, phase)
		} else {
			fmt.Fprintf(&out, "面板 %d（%s）：", index+1, phase)
		}
		fmt.Fprintf(&out, "%s", fallbackValue(panel.Camera, "镜头信息未提供"))
		if strings.TrimSpace(panel.Scene) != "" {
			fmt.Fprintf(&out, "。%s", strings.TrimSpace(panel.Scene))
		}
		if strings.TrimSpace(panel.Block) != "" {
			fmt.Fprintf(&out, "。%s", panel.Block)
		}
		if strings.TrimSpace(panel.Action) != "" {
			fmt.Fprintf(&out, "。%s", strings.TrimSpace(panel.Action))
		}
		if panelAssets := storyboardPromptAssetText(panel.Assets); panelAssets != "" {
			fmt.Fprintf(&out, "。资产引用：%s", panelAssets)
		}
		if strings.TrimSpace(panel.BoundaryNote) != "" {
			fmt.Fprintf(&out, "。面板边界备注：%s", panel.BoundaryNote)
		}
		out.WriteString("。\n")
	}
	fmt.Fprintf(&out, "统一要求：全体面板统一画风（%s）、统一时段光影、统一色调与可投喂视觉参数（%s）；同一角色跨面板保持同一张脸、同一套服装；场景、站位、视线、手位、道具状态和轴线连续。\n",
		fallbackValue(styleBase, "写实影视级"), fallbackValue(styleParameters, "沿用项目画风锁"))
	out.WriteString("面板只表示动作相位，不是剪辑点；不得用多画一格暗示未授权的反打、插入或切镜。\n")
	out.WriteString("禁止：任何文字、字幕、对白框、序号、箭头、水印、Logo、技术标注；禁止出现未点名的额外人物、道具或伪造资产。")
	stylePrompt := ""
	if len(strongStylePrompt) > 0 {
		stylePrompt = strongStylePrompt[0]
	}
	return prependStylePrompt(stylePrompt, out.String())
}

func storyboardSheetLayout(count int) string {
	switch count {
	case 1:
		return "单面板"
	case 2:
		return "1×2 双格"
	case 3:
		return "单行 3 格"
	case 4:
		return "2×2 四宫格"
	case 5:
		return "单行 5 格"
	case 6:
		return "2×3 六宫格"
	case 7, 8:
		return "2×4 多格"
	case 9:
		return "3×3 九宫格"
	default:
		return "等大网格"
	}
}

func parseStoryboardImageSegments(content string) ([]storyboardImageSegment, error) {
	section, found := p3MarkdownSection(content, "## 故事板表")
	if !found {
		return nil, ErrStoryboardImageInvalid
	}
	lines := strings.Split(strings.ReplaceAll(section, "\r\n", "\n"), "\n")
	type tableBlock struct {
		segmentHint  string
		segmentTitle string
		lines        []string
	}
	blocks := make([]tableBlock, 0)
	table := make([]string, 0)
	segmentHint := ""
	segmentTitle := ""
	flushTable := func() {
		if len(table) == 0 {
			return
		}
		blocks = append(blocks, tableBlock{
			segmentHint:  segmentHint,
			segmentTitle: segmentTitle,
			lines:        append([]string(nil), table...),
		})
		table = table[:0]
	}
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if strings.HasPrefix(line, "### ") {
			flushTable()
			segmentHint, segmentTitle = parseStoryboardSegmentHeading(line)
			continue
		}
		if strings.HasPrefix(line, "|") {
			table = append(table, line)
		} else if len(table) > 0 && line == "" {
			flushTable()
		}
	}
	flushTable()
	if len(blocks) == 0 {
		return nil, ErrStoryboardImageInvalid
	}

	result := make([]storyboardImageSegment, 0)
	byCode := make(map[string]int)
	for _, block := range blocks {
		if len(block.lines) < 3 {
			continue
		}
		headers := markdownCells(block.lines[0])
		segmentIndex := storyboardColumnIndex(headers, "段", "段号", "段落", "段落号", "分镜段")
		phaseIndex := storyboardColumnIndex(headers, "动作相位", "相位", "动作阶段")
		if phaseIndex < 0 {
			return nil, ErrStoryboardImageInvalid
		}
		titleIndex := storyboardColumnIndex(headers, "段落标题", "段标题", "标题")
		shotIndex := storyboardColumnIndex(headers, "面板ID", "面板编号", "面板号", "格号", "格ID", "格编号", "格", "镜格号", "分镜格号")
		cameraIndex := storyboardColumnIndex(headers,
			"镜头信息", "镜头", "景别机位运镜", "景别角度运镜",
			"景别焦段机位运镜", "景别角度机位运镜",
		)
		stanceIndex := storyboardColumnIndex(headers,
			"站位与视线", "站位+视线", "站位/视线", "站位及视线",
			"人物站位与视线", "人物站位视线", "站位视线", "人物站位", "站位", "视线",
		)
		sceneIndex := storyboardColumnIndex(headers, "场景一句话", "场景", "环境")
		actionIndex := storyboardColumnIndex(headers, "动作", "动作描述", "表演动作")
		assetIndex := storyboardColumnIndex(headers, "资产引用", "资产参考", "参考资产", "资产编号", "图号")
		boundaryIndex := storyboardColumnIndex(headers, "面板边界备注", "面板边界", "边界备注")

		for _, row := range block.lines[2:] {
			cells := markdownCells(row)
			code := block.segmentHint
			if segmentIndex >= 0 {
				if value := markdownCell(cells, segmentIndex); value != "" {
					code = value
				}
			}
			if strings.TrimSpace(code) == "" {
				return nil, ErrStoryboardImageInvalid
			}
			key := normalizeStoryboardSegmentCode(code)
			if key == "" {
				return nil, ErrStoryboardImageInvalid
			}
			position, exists := byCode[key]
			if !exists {
				position = len(result)
				byCode[key] = position
				result = append(result, storyboardImageSegment{Code: key, Title: block.segmentTitle})
			}
			segment := &result[position]
			if segment.Title == "" {
				segment.Title = fallbackValue(markdownCell(cells, titleIndex), block.segmentTitle)
			}
			if value := markdownCell(cells, shotIndex); value != "" {
				segment.Shots = appendUnique(segment.Shots, value)
			}
			if value := markdownCell(cells, phaseIndex); value != "" {
				segment.Phases = appendUnique(segment.Phases, value)
			}
			if value := markdownCell(cells, cameraIndex); value != "" {
				segment.Camera = appendUnique(segment.Camera, value)
			}
			if value := markdownCell(cells, stanceIndex); value != "" {
				segment.Block = appendUnique(segment.Block, value)
			}
			if value := markdownCell(cells, assetIndex); value != "" {
				segment.Assets = appendUnique(segment.Assets, value)
			}
			panel := storyboardImagePanel{
				ID:           markdownCell(cells, shotIndex),
				Phase:        markdownCell(cells, phaseIndex),
				Camera:       markdownCell(cells, cameraIndex),
				Block:        markdownCell(cells, stanceIndex),
				Scene:        markdownCell(cells, sceneIndex),
				Action:       markdownCell(cells, actionIndex),
				Assets:       markdownCell(cells, assetIndex),
				BoundaryNote: markdownCell(cells, boundaryIndex),
			}
			if panel.ID != "" || panel.Phase != "" || panel.Camera != "" || panel.Block != "" ||
				panel.Scene != "" || panel.Action != "" || panel.Assets != "" || panel.BoundaryNote != "" {
				segment.Panels = append(segment.Panels, panel)
			}
		}
	}
	if len(result) == 0 {
		return nil, ErrStoryboardImageInvalid
	}
	for index := range result {
		if len(result[index].Phases) == 0 && len(result[index].Camera) == 0 && len(result[index].Block) == 0 {
			return nil, ErrStoryboardImageInvalid
		}
	}
	return result, nil
}

func normalizeStoryboardSegmentCode(value string) string {
	key := p3SegmentKey(value)
	if key == "" {
		return ""
	}
	return "段" + key
}

func parseStoryboardSegmentHeading(line string) (string, string) {
	value := strings.TrimSpace(line)
	location := p3ExactStoryboardHeading.FindStringSubmatchIndex(value)
	if len(location) < 6 {
		return "", ""
	}
	if location[2] < 0 || location[4] < 0 {
		return "", ""
	}
	code := "段" + value[location[2]:location[3]]
	title := strings.TrimSpace(value[location[4]:location[5]])
	return code, title
}

func storyboardColumnIndex(headers []string, aliases ...string) int {
	normalizedAliases := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		normalizedAliases[normalizeStoryboardColumn(alias)] = true
	}
	for index, header := range headers {
		if normalizedAliases[normalizeStoryboardColumn(header)] {
			return index
		}
	}
	for index, header := range headers {
		normalizedHeader := normalizeStoryboardColumn(header)
		for _, alias := range aliases {
			normalizedAlias := normalizeStoryboardColumn(alias)
			if len([]rune(normalizedAlias)) >= 3 && strings.Contains(normalizedHeader, normalizedAlias) {
				return index
			}
		}
	}
	return -1
}

func normalizeStoryboardColumn(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer(
		" ", "", "\t", "", "\u3000", "",
		"*", "", "_", "", "`", "",
		"＋", "", "+", "", "／", "", "/", "",
		"、", "", "·", "", "：", ":",
		"（", "(", "）", ")",
	)
	return replacer.Replace(value)
}

func storyboardSegmentLabel(segment storyboardImageSegment) string {
	number := strings.TrimPrefix(segment.Code, "段")
	if segment.Title != "" {
		return fmt.Sprintf("第 %s 段「%s」", number, segment.Title)
	}
	return fmt.Sprintf("第 %s 段", number)
}

func storyboardPromptAssetText(values ...string) string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return strings.Join(result, "；")
}

func normalizeStoryboardVisualAssetCode(value string) string {
	if match := storyboardAssetModifierPattern.FindStringSubmatch(value); len(match) > 1 {
		return match[1]
	}
	return value
}

func markdownCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func markdownCell(cells []string, index int) string {
	if index < 0 || index >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[index])
}

func isMarkdownSeparator(value string) bool {
	value = strings.ReplaceAll(strings.TrimSpace(value), " ", "")
	return value != "" && strings.Trim(value, "-:") == ""
}

func storyboardShotLabel(segment storyboardImageSegment) string {
	if len(segment.Shots) == 0 {
		return ""
	}
	if len(segment.Shots) == 1 {
		return "格" + segment.Shots[0]
	}
	return "格" + strings.Join(segment.Shots, "/")
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func fallbackValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func reconcileProductionStoryboardImageLineage(
	tx *repository.Repository,
	projectID, runID string,
	stage model.ManjuProductionStage,
	required map[string]p3RequiredSegment,
	requirements map[string]p3DeliveryRequirement,
	canvasImages []model.ManjuProductionImage,
) ([]model.ManjuProductionImage, error) {
	images, err := tx.ManjuProductionImages(runID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	nextPosition := 1
	for index := range images {
		image := &images[index]
		if image.Position >= nextPosition {
			nextPosition = image.Position + 1
		}
		if image.Status == ProductionImageStatusSuperseded {
			continue
		}
		key := p3SegmentKey(image.SegmentCode)
		requirement := requirements[key]
		staleReason := ""
		switch {
		case key == "" || required[key].Key == "":
			staleReason = "segment is no longer present in current P3"
		case requirement.Exempt:
			staleReason = "segment is exempt in current P3"
		case strings.TrimSpace(image.ArtifactID) == "":
			if stage.Version > 1 {
				staleReason = "legacy image has no current P3 artifact lineage"
			} else {
				image.ArtifactID = stage.ArtifactID
				image.UpdatedAt = now
				if err := tx.SaveManjuProductionImage(image); err != nil {
					return nil, err
				}
			}
		case image.ArtifactID != stage.ArtifactID:
			staleReason = "P3 artifact lineage changed"
		}
		if staleReason == "" {
			continue
		}
		image.Status = ProductionImageStatusSuperseded
		image.Error = "SUPERSEDED: " + staleReason
		image.UpdatedAt = now
		if err := tx.SaveManjuProductionImage(image); err != nil {
			return nil, err
		}
	}

	for _, fresh := range canvasImages {
		key := p3SegmentKey(fresh.SegmentCode)
		segment, exists := required[key]
		if !exists || requirements[key].Exempt || stage.ArtifactID == "" {
			return nil, ErrStoryboardImageInvalid
		}
		exactMatch := -1
		for index := range images {
			image := &images[index]
			if image.Status == ProductionImageStatusSuperseded ||
				p3SegmentKey(image.SegmentCode) != key ||
				image.ArtifactID != stage.ArtifactID {
				continue
			}
			if exactMatch < 0 &&
				image.Status == ProductionImageStatusReady &&
				image.ResourceID == fresh.ResourceID &&
				image.TaskID == fresh.TaskID &&
				image.CreatedBy == fresh.CreatedBy {
				exactMatch = index
				continue
			}
			image.Status = ProductionImageStatusSuperseded
			image.Error = "SUPERSEDED: replaced by current canvas task/resource"
			image.UpdatedAt = now
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return nil, err
			}
		}
		if exactMatch >= 0 {
			continue
		}
		image := model.ManjuProductionImage{
			ID: uuid.NewString(), ProjectID: projectID, RunID: runID, StageID: stage.ID,
			ArtifactID: stage.ArtifactID, Position: nextPosition,
			SegmentCode: segment.Code, ShotLabel: "canvas",
			Prompt: fresh.Prompt, Attempt: 1, Status: ProductionImageStatusReady,
			TaskID: fresh.TaskID, ResourceID: fresh.ResourceID, URL: fresh.URL,
			CreatedBy: fresh.CreatedBy, CreatedAt: now, UpdatedAt: now,
		}
		if strings.TrimSpace(image.CreatedBy) == "" {
			return nil, ErrStoryboardImageInvalid
		}
		if err := tx.CreateManjuProductionImages([]model.ManjuProductionImage{image}); err != nil {
			return nil, err
		}
		nextPosition++
		images = append(images, image)
	}
	return images, nil
}

func reconcileProductionStoryboardImages(tx *repository.Repository, runID string) error {
	images, err := tx.ManjuProductionImages(runID)
	if err != nil {
		return err
	}
	for index := range images {
		image := &images[index]
		if image.Status == ProductionImageStatusSuperseded {
			continue
		}
		if image.Status == ProductionImageStatusReady || image.Status == ProductionImageStatusFailed || strings.TrimSpace(image.TaskID) == "" {
			continue
		}
		task, err := tx.TaskForUser(image.CreatedBy, image.TaskID)
		if errors.Is(err, gorm.ErrRecordNotFound) || task == nil {
			image.Status = ProductionImageStatusFailed
			image.Error = "图片任务不存在"
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if task.ProjectID != image.ProjectID {
			image.Status = ProductionImageStatusFailed
			image.Error = "图片任务不属于当前项目"
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
			continue
		}
		if task.Type != "canvas_image" {
			image.Status = ProductionImageStatusFailed
			image.Error = "图片任务类型不是 canvas_image"
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
			continue
		}
		switch task.Status {
		case model.TaskStatusQueued:
			image.Status = ProductionImageStatusRunning
			image.Error = ""
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
		case model.TaskStatusRunning:
			image.Status = ProductionImageStatusRunning
			image.Error = ""
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
		case model.TaskStatusFailed, model.TaskStatusCancelled:
			image.Status = ProductionImageStatusFailed
			image.Error = strings.TrimSpace(task.Error)
			if image.Error == "" {
				image.Error = "图片任务未成功完成"
			}
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
		case model.TaskStatusSucceeded:
			results, err := tx.GenerationOutputResults(task.ID)
			if err != nil {
				return err
			}
			ownedResults := make([]model.Result, 0, len(results))
			for _, result := range results {
				if result.UserID == image.CreatedBy {
					ownedResults = append(ownedResults, result)
				}
			}
			results = ownedResults
			outputs, err := localtask.DecodeOutputResults(results)
			if err != nil {
				image.Status = ProductionImageStatusFailed
				image.Error = "图片交付记录无法读取"
				if err := tx.SaveManjuProductionImage(image); err != nil {
					return err
				}
				continue
			}
			var selected *localtask.CanonicalOutput
			for outputIndex := range outputs {
				if outputs[outputIndex].MediaType == "image" && strings.TrimSpace(outputs[outputIndex].ResourceID) != "" {
					selected = &outputs[outputIndex]
					break
				}
			}
			if selected == nil {
				image.Status = ProductionImageStatusFailed
				image.Error = "图片任务成功但没有可交付的真实图片资源"
				if err := tx.SaveManjuProductionImage(image); err != nil {
					return err
				}
				continue
			}
			resource, err := tx.ResourceForUser(task.UserID, selected.ResourceID)
			if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil {
				image.Status = ProductionImageStatusFailed
				image.Error = "图片资源不存在或不属于任务账号"
				if err := tx.SaveManjuProductionImage(image); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if resource.Kind != "image" || resource.Status != model.ResourceStatusReady {
				image.Status = ProductionImageStatusRunning
				image.Error = "图片资源尚未就绪"
				if err := tx.SaveManjuProductionImage(image); err != nil {
					return err
				}
				continue
			}
			image.Status = ProductionImageStatusReady
			image.ResourceID = resource.ID
			image.URL = resource.PublicURL
			image.Error = ""
			if err := tx.SaveManjuProductionImage(image); err != nil {
				return err
			}
		}
	}
	return nil
}

func storyboardImagesReady(tx *repository.Repository, runID string) error {
	images, err := tx.ManjuProductionImages(runID)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return nil
	}
	active := 0
	for _, image := range images {
		if image.Status == ProductionImageStatusSuperseded {
			continue
		}
		active++
		if image.Status != ProductionImageStatusReady || strings.TrimSpace(image.ResourceID) == "" {
			return ErrStoryboardImagesNotReady
		}
		resource, err := tx.ResourceForUser(image.CreatedBy, image.ResourceID)
		if errors.Is(err, gorm.ErrRecordNotFound) || resource == nil || resource.Status != model.ResourceStatusReady || resource.Kind != "image" {
			return ErrStoryboardImagesNotReady
		}
		if err != nil {
			return err
		}
	}
	if active == 0 {
		return nil
	}
	return nil
}

func productionImageView(image model.ManjuProductionImage) ProductionImageView {
	return ProductionImageView{
		ID: image.ID, RunID: image.RunID, ArtifactID: image.ArtifactID, Position: image.Position,
		SegmentCode: image.SegmentCode, ShotLabel: image.ShotLabel,
		Attempt: image.Attempt, Status: image.Status, TaskID: image.TaskID, ResourceID: image.ResourceID,
		URL: image.URL, Error: image.Error, CreatedAt: image.CreatedAt, UpdatedAt: image.UpdatedAt,
	}
}
