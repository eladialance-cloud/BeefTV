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

const (
	manjuProjectStateSchemaVersion = "7.4.23"
	manjuProjectStatePatchStart    = "<!-- MANJU_PROJECT_STATE_PATCH_JSON"
	manjuProjectStatePatchEnd      = "-->"
)

var (
	ErrProjectStateUnavailable = errors.New("V7.4.26 project_state.json 尚未建立")
	ErrInvalidProjectState     = errors.New("V7.4.26 project_state.json 回写块无效")
)

func manjuProjectStateFailure(stageLabel string, err error) string {
	return fmt.Sprintf(
		"V7.4.26 %s 项目状态台账回写失败：%v。请按 V7.4.26 官方规则重新生成当前阶段工件。",
		stageLabel,
		err,
	)
}

func initializeManjuProjectState(tx *repository.Repository, projectName string, receipt *model.ManjuLockReceipt, createdBy string) error {
	if receipt == nil {
		return ErrProjectStateUnavailable
	}
	state, err := defaultManjuProjectState(projectName, receipt)
	if err != nil {
		return err
	}
	return persistManjuProjectState(tx, receipt.ProjectID, receipt.Revision, "p0:lock", "", state, createdBy)
}

func defaultManjuProjectState(projectName string, receipt *model.ManjuLockReceipt) (map[string]any, error) {
	view, err := receiptView(*receipt)
	if err != nil {
		return nil, err
	}
	styleCard := sanitizeStyleNames(view.Snapshot.StylePromptParameters)
	if strings.TrimSpace(styleCard) == "" {
		styleCard = "电影级写实质感"
	}
	forbiddenElements := make([]any, 0, len(view.Snapshot.StyleForbiddenItems))
	for _, item := range uniqueCustomStyleItems(view.Snapshot.StyleForbiddenItems...) {
		forbiddenElements = append(forbiddenElements, item)
	}
	delivery := map[string]string{
		DeliveryBoard:    "1",
		DeliveryMarkdown: "2",
		DeliveryBoth:     "3",
	}[view.Snapshot.DeliveryMode]
	seam := map[string]string{
		SeamTailFrameFreeze: "1",
		SeamHardCut:         "2",
		SeamInsertShot:      "3",
		SeamCreative:        "4",
	}[view.Snapshot.SeamMode]
	if seam == "" {
		seam = "1"
	}
	return map[string]any{
		"schema_version": manjuProjectStateSchemaVersion,
		"project_name":   fallbackValue(strings.TrimSpace(projectName), "未命名漫剧项目"),
		"locks": map[string]any{
			"track":         manjuSkillTrackCode(view.Snapshot.Track),
			"source":        manjuSkillSourceCode(view.Snapshot.Source),
			"model":         normalizeManjuSkillModel(view.Snapshot.VideoModel),
			"aspect":        view.Snapshot.AspectRatio,
			"delivery":      fallbackValue(delivery, "3"),
			"seam":          seam,
			"style_card":    styleCard,
			"asset_mode":    fallbackValue(view.Snapshot.AssetMode, AssetModeM1Value),
			"combat_tier":   "R1",
			"lock_revision": receipt.Revision,
		},
		"receipt": map[string]any{
			"active_plugins": []string{
				"sd-decision-engine", "sd-script-engine", "sd-asset-engine",
				"sd-storyboard-engine", "sd-feed-engine", "sd-qc-engine",
			},
			"read_refs": []string{
				"track-routing.md", "context-mapping.md", "emotion-anchor-table.md",
				"asset-first-pipeline.md", "asset-delivery-modes.md", "asset-spatial-ledger.md",
				"acting-direction.md", "blocking-direction.md", "storyboard-sheet-template.md",
				"plot-deduction-grid.md", "model-adapters.md", "segment-splicing.md",
			},
			"fingerprints": []string{
				fmt.Sprintf("【P0 锁 v%d · %s · %s · %s】", receipt.Revision, view.Snapshot.Track, view.Snapshot.Source, view.Snapshot.VideoModel),
				fmt.Sprintf("【画幅与素材形态：%s · %s】", view.Snapshot.AspectRatio, view.Snapshot.AssetMode),
				fmt.Sprintf("【P0 可投喂视觉参数：%s】", styleCard),
			},
		},
		"world_anchor": map[string]any{
			"era_anchor":         "",
			"region":             "",
			"tech_level":         "",
			"forbidden_elements": forbiddenElements,
			"required_elements":  []any{},
		},
		"characters": map[string]any{},
		"scenes":     map[string]any{},
		"props":      map[string]any{},
		"segments":   []any{},
	}, nil
}

func persistManjuProjectState(tx *repository.Repository, projectID string, lockRevision int64, stageKey, artifactID string, state map[string]any, createdBy string) error {
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 V7.4.26 project_state.json: %w", err)
	}
	sum := sha256.Sum256(payload)
	return tx.CreateManjuProjectState(&model.ManjuProjectStateSnapshot{
		ID:            uuid.NewString(),
		ProjectID:     projectID,
		LockRevision:  lockRevision,
		StageKey:      stageKey,
		ArtifactID:    artifactID,
		SchemaVersion: manjuProjectStateSchemaVersion,
		StateJSON:     string(payload),
		ContentHash:   hex.EncodeToString(sum[:]),
		CreatedBy:     createdBy,
		CreatedAt:     time.Now(),
	})
}

func latestManjuProjectStateMap(tx *repository.Repository, projectID string, lockRevision int64) (map[string]any, *model.ManjuProjectStateSnapshot, error) {
	snapshot, err := tx.LatestManjuProjectState(projectID, lockRevision)
	if err != nil {
		return nil, nil, err
	}
	if snapshot == nil {
		return nil, nil, nil
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(snapshot.StateJSON), &state); err != nil {
		return nil, nil, fmt.Errorf("解析 V7.4.26 project_state.json 快照失败: %w", err)
	}
	return state, snapshot, nil
}

func manjuProjectStateMapForArtifact(
	tx *repository.Repository,
	projectID string,
	lockRevision int64,
	stageKey, artifactID string,
) (map[string]any, *model.ManjuProjectStateSnapshot, error) {
	snapshot, err := tx.ManjuProjectStateByArtifact(projectID, lockRevision, stageKey, artifactID)
	if err != nil {
		return nil, nil, err
	}
	if snapshot == nil {
		return nil, nil, nil
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(snapshot.StateJSON), &state); err != nil {
		return nil, nil, fmt.Errorf("解析 V7.4.26 project_state.json 快照失败: %w", err)
	}
	return state, snapshot, nil
}

func ensureManjuProjectStateLegacy(tx *repository.Repository, projectID string, lockRevision int64, createdBy string) (map[string]any, error) {
	state, _, err := latestManjuProjectStateMap(tx, projectID, lockRevision)
	if err != nil {
		return nil, err
	}
	if state != nil {
		return state, nil
	}
	receipt, err := tx.LatestManjuLockReceipt(projectID)
	if err != nil {
		return nil, err
	}
	if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != lockRevision {
		return nil, ErrProjectStateUnavailable
	}
	access, err := tx.ProjectByID(projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProjectStateUnavailable
	}
	if err != nil {
		return nil, err
	}
	state, err = defaultManjuProjectState(access.Name, receipt)
	if err != nil {
		return nil, err
	}
	if err := persistManjuProjectState(tx, projectID, lockRevision, "p0:lock", "", state, createdBy); err != nil {
		return nil, err
	}
	return state, nil
}

func ensureManjuProjectState(tx *repository.Repository, projectID, projectName string, lockRevision int64, createdBy string) (map[string]any, error) {
	_ = projectName
	return ensureManjuProjectStateLegacy(tx, projectID, lockRevision, createdBy)
}

func commitManjuProjectStatePatchLegacy(tx *repository.Repository, projectID string, lockRevision int64, stageKey, artifactID, content, createdBy string, required bool) error {
	state, err := ensureManjuProjectStateLegacy(tx, projectID, lockRevision, createdBy)
	if err != nil {
		return err
	}
	patch, found, err := resolveManjuProjectStatePatch(stageKey, content, state)
	if err != nil {
		return err
	}
	if !found {
		if required {
			return fmt.Errorf("%w: %s 未输出 %s 回写块", ErrInvalidProjectState, stageKey, manjuProjectStatePatchStart)
		}
		return nil
	}
	mergeManjuProjectState(state, patch, stageKey)
	if err := validateCommittedManjuProjectState(stageKey, state); err != nil {
		return err
	}
	return persistManjuProjectState(tx, projectID, lockRevision, stageKey, artifactID, state, createdBy)
}

func previewManjuProjectStatePatch(
	tx *repository.Repository,
	projectID, projectName string,
	lockRevision int64,
	stageKey, content, createdBy string,
) (string, error) {
	if _, err := ensureManjuProjectState(tx, projectID, projectName, lockRevision, createdBy); err != nil {
		return "", err
	}
	state, snapshot, err := latestManjuProjectStateMap(tx, projectID, lockRevision)
	if err != nil {
		return "", err
	}
	if snapshot == nil {
		return "", ErrProjectStateUnavailable
	}
	patch, found, err := resolveManjuProjectStatePatch(stageKey, content, state)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s 未输出 %s 回写块", ErrInvalidProjectState, stageKey, manjuProjectStatePatchStart)
	}
	mergeManjuProjectState(state, patch, stageKey)
	if err := validateCommittedManjuProjectState(stageKey, state); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 P3 预检 project_state: %w", err)
	}
	return string(data), nil
}

func commitManjuProjectStatePatch(tx *repository.Repository, projectID, projectName string, lockRevision int64, stageKey, artifactID, content, createdBy string, required bool) error {
	_ = projectName
	return commitManjuProjectStatePatchLegacy(tx, projectID, lockRevision, stageKey, artifactID, content, createdBy, required)
}

func extractManjuProjectStatePatch(content string) (map[string]any, bool, error) {
	start := strings.Index(content, manjuProjectStatePatchStart)
	if start < 0 {
		return nil, false, nil
	}
	rest := content[start+len(manjuProjectStatePatchStart):]
	end := strings.Index(rest, manjuProjectStatePatchEnd)
	if end < 0 {
		return nil, false, fmt.Errorf("%w: 状态回写块缺少结束标记 -->", ErrInvalidProjectState)
	}
	raw := strings.TrimSpace(rest[:end])
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, fmt.Errorf("%w: 状态回写块为空", ErrInvalidProjectState)
	}
	var patch map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&patch); err != nil {
		return nil, true, fmt.Errorf("%w: JSON 解析失败: %v", ErrInvalidProjectState, err)
	}
	return patch, true, nil
}

func resolveManjuProjectStatePatch(stageKey, content string, state map[string]any) (map[string]any, bool, error) {
	patch, found, err := extractManjuProjectStatePatch(content)
	if err != nil {
		return nil, false, err
	}
	if !found && stageKey == "production:director_storyboard" && p3OfficialSegmentHeading.MatchString(content) {
		patch, err = compileP3OfficialStoryboardState(content, state)
		if err != nil {
			return nil, false, err
		}
		found = len(anySlice(patch["segments"])) > 0
	}
	if !found && strings.HasPrefix(stageKey, "script:") {
		patch, found, err = compileP1OfficialAnchorState(content, state)
		if err != nil {
			return nil, false, err
		}
	}
	if !found && stageKey == "asset:asset_compile" {
		patch, found, err = compileP2OfficialAssetState(content, state)
		if err != nil {
			return nil, false, err
		}
	}
	if !found {
		return nil, false, nil
	}
	if err := validateManjuProjectStatePatch(stageKey, patch); err != nil {
		return nil, false, err
	}
	return patch, true, nil
}

func validateRequiredManjuProjectStatePatch(stageKey, content string) error {
	patch, found, err := resolveManjuProjectStatePatch(stageKey, content, nil)
	if err != nil {
		return err
	}
	if !found {
		if stageKey == "production:director_storyboard" {
			return fmt.Errorf(
				"%w: 未识别到可编译的官方分镜结构；请按 V7.4.26 官方分镜与故事板规则重新生成当前 P3 工件",
				ErrP3StoryboardContract,
			)
		}
		return fmt.Errorf(
			"%w: 未识别到可编译的 V7.4.26 官方阶段结构；请按官方规则重新生成当前阶段工件",
			ErrInvalidProjectState,
		)
	}
	if err := validateManjuProjectStatePatch(stageKey, patch); err != nil {
		return err
	}
	switch stageKey {
	case "asset:asset_compile":
		if len(anyMap(patch["characters"])) == 0 || len(anyMap(patch["scenes"])) == 0 {
			return fmt.Errorf("%w: P2 状态回写必须包含非空 characters 与 scenes", ErrInvalidProjectState)
		}
	case "production:director_storyboard":
		segments := anySlice(patch["segments"])
		if len(segments) == 0 {
			return fmt.Errorf("%w: P3 状态回写必须包含非空 segments", ErrInvalidProjectState)
		}
	}
	return nil
}

func validateManjuProjectStatePatch(stageKey string, patch map[string]any) error {
	if _, forbidden := patch["locks"]; forbidden {
		return fmt.Errorf("%w: %s 不得改写 P0 locks", ErrInvalidProjectState, stageKey)
	}
	if _, forbidden := patch["receipt"]; forbidden {
		return fmt.Errorf("%w: %s 不得改写机检回执", ErrInvalidProjectState, stageKey)
	}
	allowed := projectStateAllowedPatchFields(stageKey)
	if len(allowed) == 0 {
		return fmt.Errorf("%w: %s 不是允许回写台账的阶段", ErrInvalidProjectState, stageKey)
	}
	for field := range patch {
		switch field {
		case "schema_version", "project_name":
			continue
		}
		if !allowed[field] {
			return fmt.Errorf("%w: %s 不允许回写字段 %s", ErrInvalidProjectState, stageKey, field)
		}
	}
	if allowed["characters"] {
		label := "P1"
		if stageKey == "asset:asset_compile" {
			label = "P2"
		}
		if err := validateManjuCharacterStates(label, anyMap(patch["characters"])); err != nil {
			return err
		}
	}
	return nil
}

func projectStateAllowedPatchFields(stageKey string) map[string]bool {
	switch stageKey {
	case "script:ip_development", "script:series_bible", "script:script_draft":
		return valueSet("world_anchor", "characters")
	case "asset:asset_compile":
		return valueSet("world_anchor", "characters", "scenes", "props")
	case "production:director_storyboard":
		return valueSet("segments")
	default:
		return nil
	}
}

func mergeManjuProjectState(state, patch map[string]any, stageKey string) {
	for field := range projectStateAllowedPatchFields(stageKey) {
		value, ok := patch[field]
		if !ok || value == nil {
			continue
		}
		if field == "segments" {
			state[field] = value
			continue
		}
		dst, dstOK := state[field].(map[string]any)
		src, srcOK := value.(map[string]any)
		if dstOK && srcOK {
			mergeManjuStateMaps(dst, src)
			state[field] = dst
			continue
		}
		state[field] = value
	}
}

func mergeManjuStateMaps(dst, src map[string]any) {
	for key, value := range src {
		srcMap, srcOK := value.(map[string]any)
		dstMap, dstOK := dst[key].(map[string]any)
		if srcOK && dstOK {
			mergeManjuStateMaps(dstMap, srcMap)
			continue
		}
		dst[key] = value
	}
}

func anyString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func validateCommittedManjuProjectState(stageKey string, state map[string]any) error {
	if stageKey != "asset:asset_compile" && stageKey != "production:director_storyboard" {
		return nil
	}
	switch stageKey {
	case "asset:asset_compile":
		if len(anyMap(state["characters"])) == 0 {
			return fmt.Errorf("%w: P2 资产台账 characters 不能为空", ErrInvalidProjectState)
		}
		if len(anyMap(state["scenes"])) == 0 {
			return fmt.Errorf("%w: P2 资产台账 scenes 不能为空", ErrInvalidProjectState)
		}
		if err := validateManjuCharacterStates("P2", anyMap(state["characters"])); err != nil {
			return err
		}
	case "production:director_storyboard":
		segments := anySlice(state["segments"])
		if len(segments) == 0 {
			return fmt.Errorf("%w: P3 台账 segments 不能为空", ErrInvalidProjectState)
		}
		for index, item := range segments {
			segment := anyMap(item)
			if len(anySlice(segment["shots"])) == 0 {
				return fmt.Errorf("%w: P3 台账第 %d 段缺少 shots", ErrInvalidProjectState, index+1)
			}
		}
	}
	return nil
}

func validateManjuCharacterStates(label string, characters map[string]any) error {
	names := make([]string, 0, len(characters))
	for name := range characters {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		states := anyMap(anyMap(characters[name])["char_states"])
		stateKeys := make([]string, 0, len(states))
		for stateKey := range states {
			stateKeys = append(stateKeys, stateKey)
		}
		sort.Strings(stateKeys)
		for _, stateKey := range stateKeys {
			if _, ok := states[stateKey].(map[string]any); !ok {
				return fmt.Errorf(
					"%w: %s 角色 %s 的 char_states.%s 必须是对象（item/action/consequence），不得写成字符串摘要",
					ErrInvalidProjectState,
					label,
					name,
					stateKey,
				)
			}
		}
	}
	return nil
}

func anyMap(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func anySlice(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	return nil
}

func scriptProjectStateStageKey(stageKey string) string {
	switch stageKey {
	case ScriptStageIPDevelopment, ScriptStageSeriesBible, ScriptStageScriptDraft:
		return "script:" + stageKey
	default:
		return ""
	}
}

func scriptArtifactContentByID(tx *repository.Repository, runID, artifactID string) (string, error) {
	artifacts, err := tx.ManjuScriptArtifacts(runID)
	if err != nil {
		return "", err
	}
	for _, artifact := range artifacts {
		if artifact.ID == artifactID {
			return artifact.Content, nil
		}
	}
	return "", ErrProjectStateUnavailable
}

func assetArtifactContentByID(tx *repository.Repository, runID, artifactID string) (string, error) {
	artifacts, err := tx.ManjuAssetArtifacts(runID)
	if err != nil {
		return "", err
	}
	for _, artifact := range artifacts {
		if artifact.ID == artifactID {
			return artifact.Content, nil
		}
	}
	return "", ErrProjectStateUnavailable
}

func projectStatePatchInstruction(stage string) string {
	switch stage {
	case "P1":
		return `

【VisiDeep AI 项目状态传输块】
平台会从当前阶段官方《世界锚卡》和《人物锚卡》原文自动提取内部台账。不要为本平台新增 JSON 块、额外字段或改标题。`
	case "P2":
		return `

【VisiDeep AI 项目状态传输块】
平台会从当前阶段官方“图N”资产块自动提取内部台账。不要为本平台新增 JSON 块、额外资产编码或改标题。`
	case "P3":
		return p3ProjectStateInstruction()
	default:
		return ""
	}
}
