package manju

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestP3RequiredStatePatchAcceptsOfficialStoryboardWithoutJSON(t *testing.T) {
	content := officialP3ArtifactForTest()

	if err := validateRequiredManjuProjectStatePatch("production:director_storyboard", content); err != nil {
		t.Fatalf("official V7.4.26 storyboard sections must compile without a duplicated JSON block: %v", err)
	}
}

func TestP3RequiredStatePatchAcceptsSegmentHeadingWithoutPlatformMetadata(t *testing.T) {
	content := strings.Replace(
		officialP3ArtifactForTest(),
		"### 段1｜B01｜旧宅｜15s",
		"### 段1",
		1,
	)

	if err := validateRequiredManjuProjectStatePatch("production:director_storyboard", content); err != nil {
		t.Fatalf("official segment grouping must not require platform-only title or duration metadata: %v", err)
	}
}

func TestP3RequiredStatePatchAcceptsOfficialShotHeadingLevel(t *testing.T) {
	content := strings.ReplaceAll(officialP3ArtifactForTest(), "#### 镜头 ", "### 镜头 ")

	if err := validateRequiredManjuProjectStatePatch("production:director_storyboard", content); err != nil {
		t.Fatalf("official ### 镜头 headings must compile without platform-only heading levels: %v", err)
	}
}

func TestP3RequiredStatePatchAcceptsOfficialShotNumbersWithLeadingZeros(t *testing.T) {
	content := officialP3ArtifactForTest()
	content = strings.Replace(content, "#### 镜头 1 ", "#### 镜头 01 ", 1)
	content = strings.Replace(content, "#### 镜头 2 ", "#### 镜头 02 ", 1)
	content = strings.Replace(content, "#### 镜头 3 ", "#### 镜头 03 ", 1)

	if err := validateRequiredManjuProjectStatePatch("production:director_storyboard", content); err != nil {
		t.Fatalf("official leading-zero shot numbers must compile without platform-only numbering rules: %v", err)
	}
}

func TestP3RequiredStatePatchMissingStructureDoesNotRequestPlatformJSON(t *testing.T) {
	err := validateRequiredManjuProjectStatePatch(
		"production:director_storyboard",
		"# P3\n\n只有说明文字，没有可识别的官方分镜结构。",
	)
	if err == nil {
		t.Fatal("missing P3 storyboard structure must fail")
	}
	if strings.Contains(err.Error(), "MANJU_PROJECT_STATE_PATCH_JSON") {
		t.Fatalf("P3 must not fall back to a platform-only JSON requirement: %v", err)
	}
	if !strings.Contains(err.Error(), "官方分镜结构") {
		t.Fatalf("P3 failure must point to the official storyboard structure: %v", err)
	}
}

func TestP3StateFailureDoesNotRequestPlatformJSON(t *testing.T) {
	message := manjuProjectStateFailure("P3 分镜与故事板", ErrP3StoryboardContract)
	if strings.Contains(message, "MANJU_PROJECT_STATE_PATCH_JSON") {
		t.Fatalf("P3 failure must not request a platform-only JSON block: %s", message)
	}
	if !strings.Contains(message, "按 V7.4.26 官方规则重新生成") {
		t.Fatalf("P3 failure must point back to the official rule-driven regeneration: %s", message)
	}
}

func TestCompileOfficialP3StoryboardBuildsDeterministicTimelineAndAssetRefs(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{
			"门轴": map[string]any{"id": "PRP-01", "fig_no": 3},
		},
	}

	patch, err := compileP3OfficialStoryboardState(officialP3ArtifactForTest(), baseState)
	if err != nil {
		t.Fatal(err)
	}
	segments := anySlice(patch["segments"])
	if len(segments) != 1 {
		t.Fatalf("expected one official segment, got %+v", patch)
	}
	segment := anyMap(segments[0])
	if got := anyString(segment["segment_type"]); got != "对话戏" {
		t.Fatalf("segment_type must be derived from the locked P3 content: got=%q segment=%+v", got, segment)
	}
	shots := anySlice(segment["shots"])
	if len(shots) != 3 {
		t.Fatalf("expected three official shots, got %+v", shots)
	}
	wantTimes := [][2]int{{0, 3}, {3, 7}, {7, 10}}
	for index, raw := range shots {
		shot := anyMap(raw)
		if anyString(shot["start"]) != strconv.Itoa(wantTimes[index][0]) ||
			anyString(shot["end"]) != strconv.Itoa(wantTimes[index][1]) {
			t.Fatalf("shot %d timeline mismatch: %+v", index+1, shot)
		}
	}
	first := anyMap(shots[0])
	if anyString(first["spec"]) != "全景 · 28mm · 侧45度+平视 · 缓推（人物镜头）" {
		t.Fatalf("shot spec was not mapped from the official heading: %+v", first)
	}
	second := anyMap(shots[1])
	if !strings.Contains(anyString(second["dialogue"]), "有人吗") {
		t.Fatalf("dialogue was not preserved: %+v", second)
	}
	refs := anyMap(segment["asset_refs"])
	if got := stringList(refs["scenes"]); len(got) != 1 || got[0] != "旧宅" {
		t.Fatalf("scene refs must use exact P2 keys: %+v", refs)
	}
	if got := stringList(refs["characters"]); len(got) != 1 || got[0] != "林澈" {
		t.Fatalf("character refs must use exact P2 keys: %+v", refs)
	}
	if got := stringList(refs["props"]); len(got) != 1 || got[0] != "门轴" {
		t.Fatalf("props used in any shot must be aggregated at segment level: %+v", refs)
	}
}

func TestOfficialP3SegmentTypePreservesExplicitNonDialogueIntent(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
	}
	content := strings.Replace(
		officialP3ArtifactForTest(),
		"### 段1｜B01｜旧宅｜15s",
		"### 段1｜B01｜旧宅｜战斗动作段｜15s",
		1,
	)

	patch, err := compileP3OfficialStoryboardState(content, baseState)
	if err != nil {
		t.Fatal(err)
	}
	segment := anyMap(anySlice(patch["segments"])[0])
	if got := anyString(segment["segment_type"]); got != "战斗动作段" {
		t.Fatalf("explicit non-dialogue intent must not be rewritten as dialogue: got=%q", got)
	}
}

func TestOfficialP3SegmentTypeOnlyInfersEmptyShotFromEnvironmentOnlyContent(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
	}
	content := officialP3ArtifactForTest()
	for old, replacement := range map[string]string{
		`台词：林澈："有人吗？"`:                 "台词：无",
		"画面内容：林澈从左侧走到木门前，右手抬起停在门环旁。":   "画面内容：门环在冷光中轻晃，门板阴影缓慢移动。",
		"画面内容：林澈推门，门缝由窄变宽，门后灰尘在冷光里浮动。": "画面内容：门缝由窄变宽，门后灰尘在冷光里浮动。",
		"画面内容：林澈跨过门槛，视线扫向供桌，肩背保持前倾。":   "画面内容：供桌烛火在侧窗冷光中轻晃，门槛投下硬边阴影。",
		"状态：林澈站在门左侧·空手 → 右手搭门环":        "状态：门板闭合 → 门环轻晃",
		"状态：右手搭门环 → 双手推门":              "状态：门板闭合 → 门板半开",
		"状态：双手推门 → 右手离门垂落":             "状态：门板半开 → 门板完全打开",
	} {
		content = strings.ReplaceAll(content, old, replacement)
	}

	patch, err := compileP3OfficialStoryboardState(content, baseState)
	if err != nil {
		t.Fatal(err)
	}
	segment := anyMap(anySlice(patch["segments"])[0])
	if got := anyString(segment["segment_type"]); got != "空镜段" {
		t.Fatalf("environment-only content must be the only inferred empty-shot case: got=%q", got)
	}
}

func TestOfficialP3SegmentTypeLeavesAmbiguousSilentActionUnmarked(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
	}
	content := strings.ReplaceAll(officialP3ArtifactForTest(), `台词：林澈："有人吗？"`, "台词：无")

	patch, err := compileP3OfficialStoryboardState(content, baseState)
	if err != nil {
		t.Fatal(err)
	}
	segment := anyMap(anySlice(patch["segments"])[0])
	if got := anyString(segment["segment_type"]); got != "" {
		t.Fatalf("unmarked silent action must stay ambiguous for the official C27 conservative path: got=%q", got)
	}
}

func TestP3SegmentAssetRefsAllowExactMultiSceneSequences(t *testing.T) {
	state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 2})
	state["scenes"] = map[string]any{
		"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		"庭院": map[string]any{"id": "SCN-02", "fig_no": 3},
	}
	segment := anyMap(anySlice(state["segments"])[0])
	segment["scene"] = "旧宅"
	refs := anyMap(segment["asset_refs"])
	refs["scenes"] = []any{"旧宅", "庭院"}
	segment["asset_refs"] = refs

	if err := validateP3SegmentAssetRefs(state); err != nil {
		t.Fatalf("P3 must preserve an exact multi-scene sequence without guessing one scene: %v", err)
	}
}

func TestP3PromptDoesNotRequireDuplicateProjectStateJSON(t *testing.T) {
	instruction := projectStatePatchInstruction("P3")
	if strings.Contains(instruction, "MANJU_PROJECT_STATE_PATCH_JSON") {
		t.Fatalf("P3 prompt must not ask the model to copy the storyboard into JSON:\n%s", instruction)
	}
	for _, forbidden := range []string{
		"将总时长均分",
		"平均分配整秒时间轴",
		"平台不按段长÷镜数",
		"逐镜倒推整数秒",
		"单镜超过 6 秒",
		"内容不足时总时长可以诚实变短",
	} {
		if strings.Contains(instruction, forbidden) {
			t.Fatalf("P3 transport must not redefine official timing rules %q:\n%s", forbidden, instruction)
		}
	}
	for _, required := range []string{
		"九个镜头字段",
		"确定性编译",
		"不得写入 locks 或 receipt",
	} {
		if !strings.Contains(instruction, required) {
			t.Fatalf("P3 transport instruction missing %q:\n%s", required, instruction)
		}
	}
	for _, forbidden := range []string{
		"每个“### 段N”标题必须声明整数秒单段上限",
		"#### 镜头 K | {空间类型}",
		"平台会从官方“### 段N”",
	} {
		if strings.Contains(instruction, forbidden) {
			t.Fatalf("P3 transport instruction must not impose platform-only headings %q:\n%s", forbidden, instruction)
		}
	}
}

func TestParseStoryboardImageSegmentsAcceptsOfficialCLEANGridSection(t *testing.T) {
	segments, err := parseStoryboardImageSegments(officialP3ArtifactForTest())
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || segments[0].Code != "段1" {
		t.Fatalf("official CLEAN_GRID prompt must become one storyboard segment: %+v", segments)
	}
	if len(segments[0].Phases) != 4 {
		t.Fatalf("official four-panel prompt must preserve four phases: %+v", segments[0])
	}
}

func TestP3DeliveryAcceptsOfficialStoryboardLayout(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
		"locks": map[string]any{
			"model": "2.0",
		},
	}
	content := officialP3ArtifactForTest()
	patch, err := compileP3OfficialStoryboardState(content, baseState)
	if err != nil {
		t.Fatal(err)
	}
	baseState["segments"] = patch["segments"]

	images := []model.ManjuProductionImage{readyStoryboardImageForTest("段1")}
	if err := validateP3Delivery(content, baseState, images); err != nil {
		t.Fatalf("official V7.4.26 storyboard layout must satisfy P3 delivery: %v", err)
	}
}

func TestP3PlotChecklistAcceptsOfficialGraphicalDeliveryTable(t *testing.T) {
	content := officialP3ArtifactForTest()
	required, err := p3OfficialArtifactRequiredSegments(content)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := p3DeliveryChecklistRows(content, required)
	if err != nil {
		t.Fatal(err)
	}
	scenes, exemptions, err := parseP3PlotChecklist(rows)
	if err != nil {
		t.Fatalf("official graphical delivery table must be understood: %v", err)
	}
	if len(scenes) != 0 || len(exemptions) != 1 || exemptions[0].Code != "场1" {
		t.Fatalf("unexpected official plot delivery: scenes=%+v exemptions=%+v", scenes, exemptions)
	}
}

func TestOfficialP3StoryboardSplitsOnlyExactP2SceneKeys(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
			"庭院": map[string]any{"id": "SCN-02", "fig_no": 3},
		},
		"props": map[string]any{},
	}
	content := strings.Replace(
		officialP3ArtifactForTest(),
		"场景：旧宅\n画面内容：林澈从左侧走到木门前",
		"场景：旧宅 / 庭院\n画面内容：林澈从左侧走到木门前",
		1,
	)

	patch, err := compileP3OfficialStoryboardState(content, baseState)
	if err != nil {
		t.Fatal(err)
	}
	segment := anyMap(anySlice(patch["segments"])[0])
	if got := anyString(segment["scene"]); got != "旧宅" {
		t.Fatalf("segment.scene must keep the first exact P2 scene, got %q", got)
	}
	refs := anyMap(segment["asset_refs"])
	scenes := stringList(refs["scenes"])
	if len(scenes) != 2 || scenes[0] != "旧宅" || scenes[1] != "庭院" {
		t.Fatalf("multi-scene shot must map both exact P2 keys: %+v", refs)
	}
}

func TestP3DeliveryRejectsOfficialArtifactWithC9HighRiskWord(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
		"locks": map[string]any{
			"model": "2.0",
		},
	}
	content := strings.Replace(officialP3ArtifactForTest(), "衣料摩擦", "血迹", 1)
	patch, err := compileP3OfficialStoryboardState(content, baseState)
	if err != nil {
		t.Fatal(err)
	}
	baseState["segments"] = patch["segments"]

	images := []model.ManjuProductionImage{readyStoryboardImageForTest("段1")}
	err = validateP3Delivery(content, baseState, images)
	if err == nil || !strings.Contains(err.Error(), "C9") || !strings.Contains(err.Error(), "血迹") {
		t.Fatalf("P3 must reject official C9 high-risk words before locking: %v", err)
	}
}

func TestDialogueNamesIgnoreOfficialP3TechnicalColonLines(t *testing.T) {
	content := strings.Join([]string{
		"- 画风执行：焦段硬锁 [28mm, 50mm]；可投喂视觉参数",
		"- 项目：222；阶段：P3 分镜与故事板",
		`台词：林澈（压低）："有人吗？" / 梁父（低声）："进来。"`,
		`台词：无`,
		"光影：冷灰天光自东方斜入",
	}, "\n")

	names := dialogueNamesFromScript(content)
	want := []string{"林澈", "梁父"}
	if !slices.Equal(names, want) {
		t.Fatalf("dialogue names must come only from actual dialogue lines: got=%v want=%v", names, want)
	}
}

func TestOfficialP3StoryboardDerivesNonUniformRuleBasedTimeline(t *testing.T) {
	content := officialP3ArtifactForTest()
	content = strings.ReplaceAll(
		content,
		`林澈："有人吗？"`,
		`林澈（急促）："这封信是谁送来的？你为什么现在才说？"`,
	)
	content = strings.Replace(
		content,
		"林澈跨过门槛，视线扫向供桌，肩背保持前倾。",
		"林澈跨过门槛，视线扫向供桌。他再向前一步。烛火轻晃。他停住。",
		1,
	)
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
	}

	patch, err := compileP3OfficialStoryboardState(content, baseState)
	if err != nil {
		t.Fatal(err)
	}
	shots := anySlice(anyMap(anySlice(patch["segments"])[0])["shots"])
	durations := make([]int, 0, len(shots))
	for _, raw := range shots {
		shot := anyMap(raw)
		start := int(anyNumber(shot["start"]))
		end := int(anyNumber(shot["end"]))
		durations = append(durations, end-start)
	}
	if len(slices.Compact(append([]int(nil), durations...))) <= 1 {
		t.Fatalf("shot durations must follow dialogue/action content rather than equal division: %v", durations)
	}
	for index, duration := range durations {
		if duration < 2 || duration > 6 {
			t.Fatalf("shot %d duration %ds is outside the official rule-derived range", index+1, duration)
		}
	}
}

func TestOfficialP3StoryboardKeepsCompiledShotOnOneTimelineLine(t *testing.T) {
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
	}
	patch, err := compileP3OfficialStoryboardState(officialP3ArtifactForTest(), baseState)
	if err != nil {
		t.Fatal(err)
	}
	shots := anySlice(anyMap(anySlice(patch["segments"])[0])["shots"])
	second := anyMap(shots[1])
	action := anyString(second["action"])
	if strings.Contains(action, "\n") {
		t.Fatalf("compiled action must stay on the timecode line: %q", action)
	}
	if strings.Contains(action, "状态：") || strings.Contains(action, "重音：") {
		t.Fatalf("P3 storyboard-only status/emphasis fields must be stripped before P4: %q", action)
	}
	if !strings.Contains(anyString(second["sound"]), "林澈短促发问后的吸气") {
		t.Fatalf("human voice must be merged into the P4 sound field: %+v", second)
	}
	if dialogue := anyString(second["dialogue"]); !strings.Contains(dialogue, "【林澈（") ||
		!strings.Contains(dialogue, "：\"有人吗？\"】") {
		t.Fatalf("dialogue must be normalized to the official tone-position format: %q", dialogue)
	}
}

func TestOfficialP3RejectsDialogueThatCannotFitOneShot(t *testing.T) {
	content := strings.ReplaceAll(
		officialP3ArtifactForTest(),
		`林澈："有人吗？"`,
		`林澈（低声）："这封信是谁送来的？你为什么现在才说？门外还有谁在等着？"`,
	)
	baseState := map[string]any{
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-01", "fig_no": 1},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2},
		},
		"props": map[string]any{},
	}

	_, err := compileP3OfficialStoryboardState(content, baseState)
	if err == nil || !strings.Contains(err.Error(), "拆") {
		t.Fatalf("dialogue requiring more than six seconds must return to P3 for a shot split: %v", err)
	}
}

func TestProductionMachineCheckWarningsPreserveOfficialValidatorResults(t *testing.T) {
	warnings := productionMachineCheckWarnings(ManjuSkillValidationResult{
		Warnings:         []string{"C13 镜长雷同", "C15 无台词镜拉伸", "C13 镜长雷同"},
		DialogueWarnings: []string{"C27b 台词密度不足"},
	})
	want := []string{
		"C13 镜长雷同",
		"C15 无台词镜拉伸",
		"C27b 台词密度不足",
	}
	if !slices.Equal(warnings, want) {
		t.Fatalf("P4 machine-check warnings mismatch: got=%v want=%v", warnings, want)
	}
}

func officialP3ArtifactForTest() string {
	return strings.TrimSpace(`
# P3 分镜与故事板

## 一、导演稿（P3 调度决策层）

文字平面图：门在画面右侧，林澈从左侧入画，轴线保持左林澈右门。

## 二、角色表演档案

角色表演档案 · 林澈。克制的青年，肩背常紧，紧张时声音更轻。

## 三、逐场节拍表（B01）

| 场 | 段 | 节拍 1 |
|---|---|---|
| B01 | 段1 | 林澈推门，确认门内无人 |

## 四、逐段分镜稿（段1）

### 段1｜B01｜旧宅｜15s

段首站位：林澈在门左侧。段缝（段1→2）：动作桥——推门动势承接。

#### 镜头 1 | O1 室外开阔 | 全景 | 28mm | 侧45度+平视 | 缓推 | 人物镜头

场景：旧宅
画面内容：林澈从左侧走到木门前，右手抬起停在门环旁。
状态：林澈站在门左侧·空手 → 右手搭门环
光影：清晨冷光从左前方斜入，门板暗部保留木纹。
色调：寒木青灰调，高光#A8B0B8，阴影#1B2026，色温5600K。palette=冷青灰/saturation=低/film_stock=Kodak 500T/grain=细/halation=无
台词：无
重音：无
人声：林澈压低的呼吸声。
音效：衣料摩擦、门环轻碰。

#### 镜头 2 | S3 室内大型 | 中景 | 35mm | 侧45度+微俯 | 固定 | 人物镜头

场景：旧宅
画面内容：林澈推门，门缝由窄变宽，门后灰尘在冷光里浮动。
状态：右手搭门环 → 双手推门
光影：门缝冷光切入室内，门槛形成硬边阴影。
色调：同段寒木青灰调。
台词：林澈："有人吗？"
重音：人
人声：林澈短促发问后的吸气。
音效：旧门轴吱呀、脚步压过门槛。

#### 镜头 3 | S3 室内大型 | 近景 | 50mm | 正面+平视 | 缓推 | 人物镜头

场景：旧宅
画面内容：林澈跨过门槛，视线扫向供桌，肩背保持前倾。
状态：双手推门 → 右手离门垂落
光影：侧窗冷光勾出脸侧，供桌烛台留暖色反光。
色调：同段寒木青灰调。
台词：无
重音：无
人声：林澈放慢的呼吸。
音效：门槛轻响、远处风声。

## 五、故事板图 CLEAN_GRID 提示词（逐段）

1. **段1 CLEAN_GRID｜4格（2×2）**：16:9 横版四格故事板，无文字。面板1（起始·镜1）林澈站门左侧抬手；面板2（过程·镜2）双手推门；面板3（反应·镜3）跨门槛扫视供桌；面板4（出口·镜3出口）右手离门，出口状态连续。统一旧宅结构与冷光。

## 六、剧情分镜格提示词（条件必出）

本场为单线过门动作，剧情分镜格 ⛔豁免：动作已由故事板四格完整覆盖。

**P3 图形化交付清单**

| 段 | 故事板图 | 剧情分镜格 |
|---|---|---|
| 段1 | ✅ 4格（2×2）提示词已列·待生成 | ⛔豁免：动作已由故事板四格完整覆盖 |

## 七、台词精简建议清单

本稿全量承接上游台词，无精简建议。
`)
}
