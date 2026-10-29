package manju

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestP3RejectsMissingPerformanceAndDirectingSections(t *testing.T) {
	content := validP3ArtifactWithout(
		t,
		"## 角色表演档案",
		"## 逐场节拍表",
		"## 导演稿",
	)
	if err := validateP3Delivery(content, validP3StateForTest(t), nil); err == nil {
		t.Fatal("P3 without performance/directing sections must fail")
	}
}

func TestP3CannotLockWithPartialStoryboardImageCoverage(t *testing.T) {
	state := p3StateWithSegmentsForTest(t, "段1", "段2")
	images := []model.ManjuProductionImage{
		readyStoryboardImageForTest("段1"),
	}
	err := validateP3Delivery(validP3ArtifactForState(t, state), state, images)
	if !errors.Is(err, ErrStoryboardImagesNotReady) {
		t.Fatalf("partial image coverage must fail, got %v", err)
	}
}

func TestP3AcceptsCompleteStoryboardImageCoverage(t *testing.T) {
	state := p3StateWithSegmentsForTest(t, "段1", "段2")
	images := []model.ManjuProductionImage{
		readyStoryboardImageForTest("段1"),
		readyStoryboardImageForTest("段2"),
	}
	if err := validateP3Delivery(validP3ArtifactForState(t, state), state, images); err != nil {
		t.Fatalf("complete image coverage must satisfy P3: %v", err)
	}
}

func TestP3AcceptsOnlyRuleValidSingleShotStoryboardExemption(t *testing.T) {
	state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 1})
	content := validP3ArtifactForStateWithExemptions(t, state, map[string]string{
		"段1": "理由：单镜过场，动作相位已在分镜总表完整列出。",
	})
	if err := validateP3Delivery(content, state, nil); err != nil {
		t.Fatalf("rule-valid single-shot transition exemption must pass: %v", err)
	}

	state = p3StateWithShotCountsForTest(t, map[string]int{"段1": 2})
	content = validP3ArtifactForStateWithExemptions(t, state, map[string]string{
		"段1": "无需图片。",
	})
	if err := validateP3Delivery(content, state, nil); err == nil {
		t.Fatal(`normal "无需图片" text must not bypass storyboard delivery`)
	}
}

func TestP3RejectsDuplicateAndUnknownSegmentCodes(t *testing.T) {
	state := p3StateWithSegmentsForTest(t, "段1", "段2")
	images := []model.ManjuProductionImage{
		readyStoryboardImageForTest("段1"),
		readyStoryboardImageForTest("段2"),
	}
	content := validP3ArtifactForState(t, state)

	duplicateHeading := strings.Replace(
		content,
		"### 第1段｜场景1",
		"### 第1段｜场景1\n\n### 第1段｜重复段",
		1,
	)
	if err := validateP3Delivery(duplicateHeading, state, images); err == nil {
		t.Fatal("duplicate storyboard segment heading must fail")
	}

	unknownImage := append([]model.ManjuProductionImage{}, images...)
	unknownImage = append(unknownImage, readyStoryboardImageForTest("段9"))
	if err := validateP3Delivery(content, state, unknownImage); err == nil {
		t.Fatal("unknown storyboard image segment code must fail")
	}

	duplicateImage := append([]model.ManjuProductionImage{}, images...)
	duplicateImage = append(duplicateImage, readyStoryboardImageForTest("段1"))
	if err := validateP3Delivery(content, state, duplicateImage); err == nil {
		t.Fatal("duplicate storyboard image segment code must fail")
	}
}

func TestP3RejectsMissingOrInexactSegmentIDs(t *testing.T) {
	t.Run("missing seg_id has no sequential fallback", func(t *testing.T) {
		state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 1})
		content := validP3ArtifactForState(t, state)
		segment := anyMap(anySlice(state["segments"])[0])
		delete(segment, "seg_id")

		err := validateP3Delivery(content, state, []model.ManjuProductionImage{
			readyStoryboardImageForTest("段1"),
		})
		if err == nil {
			t.Fatal("missing project_state seg_id must fail instead of falling back to 段N")
		}
	})

	t.Run("trailing text is not parsed as segment 1", func(t *testing.T) {
		state := p3StateWithShotCountsForTest(t, map[string]int{"段1-extra": 1})
		content := validP3ArtifactForState(t, state)

		err := validateP3Delivery(content, state, []model.ManjuProductionImage{
			readyStoryboardImageForTest("段1"),
		})
		if err == nil {
			t.Fatal("段1-extra must not be normalized to 段1")
		}
	})
}

func TestP3RejectsInvalidStoryboardExemptionForms(t *testing.T) {
	testCases := []struct {
		name           string
		storyboardCell string
		reason         string
	}{
		{name: "bare warning mark", storyboardCell: "⛔", reason: "理由：单镜过场，动作相位已列。"},
		{name: "token contains whitespace", storyboardCell: "⛔ 豁免（单镜过场段）", reason: "理由：动作相位已列。"},
		{name: "missing exact token", storyboardCell: "豁免（单镜过场段）", reason: "理由：动作相位已列。"},
		{name: "empty 理由 label", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由："},
		{name: "empty 单镜过场 label", storyboardCell: "⛔豁免（单镜过场段）", reason: "单镜过场："},
		{name: "placeholder reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：无"},
		{name: "pending reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：待补"},
		{name: "composite pending reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：暂无；后续补充"},
		{name: "tbd reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：TBD"},
		{name: "todo reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：todo"},
		{name: "ellipsis reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：略"},
		{name: "composite see above reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：略；见上文"},
		{name: "same as above reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：同上"},
		{name: "punctuation reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：..."},
		{name: "unicode ellipsis reason", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：……"},
		{name: "ellipsis after 略", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：略……"},
		{name: "ellipsis after 同上", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：同上……"},
		{name: "mixed ASCII unicode ellipsis", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：略.…"},
		{name: "mixed double ellipsis", storyboardCell: "⛔豁免（单镜过场段）", reason: "理由：暂无...…"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 1})
			content := p3ArtifactWithSingleChecklistRow(t, state, tc.storyboardCell, tc.reason)
			if err := validateP3Delivery(content, state, nil); err == nil {
				t.Fatalf("invalid exemption must fail: %s / %s", tc.storyboardCell, tc.reason)
			}
		})
	}

	state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 1})
	content := p3ArtifactWithSingleChecklistRow(
		t,
		state,
		"⛔豁免（单镜过场段）",
		"理由：动作相位已按单镜过场完整列入分镜总表。",
	)
	if err := validateP3Delivery(content, state, nil); err != nil {
		t.Fatalf("exact token with a substantive reason must pass: %v", err)
	}
}

func TestP3StoryboardShotCountMustMatchArtifactTruth(t *testing.T) {
	t.Run("artifact has more shots than state", func(t *testing.T) {
		state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 2})
		content := validP3ArtifactForStateWithExemptions(t, state, map[string]string{
			"段1": "理由：单镜过场，动作相位已完整列入分镜总表。",
		})
		segment := anyMap(anySlice(state["segments"])[0])
		segment["shots"] = anySlice(segment["shots"])[:1]

		if err := validateP3Delivery(content, state, nil); err == nil {
			t.Fatal("artifact shot count 2 must not pass against project_state shot count 1")
		}
	})

	t.Run("state has more shots than artifact", func(t *testing.T) {
		state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 1})
		content := validP3ArtifactForStateWithExemptions(t, state, map[string]string{
			"段1": "理由：单镜过场，动作相位已完整列入分镜总表。",
		})
		segment := anyMap(anySlice(state["segments"])[0])
		shots := anySlice(segment["shots"])
		segment["shots"] = append(shots, map[string]any{
			"no": 2, "start": 1, "end": 2, "spec": "近景 · 35mm · 侧面 · 固定",
			"action": "林澈回头", "sound": "雨声", "dialogue": "无",
		})

		if err := validateP3Delivery(content, state, nil); err == nil {
			t.Fatal("project_state shot count 2 must not pass against artifact shot count 1")
		}
	})
}

func TestP3RequiredSectionsMustBeFullMarkdownHeadings(t *testing.T) {
	state := p3StateWithSegmentsForTest(t, "段1")
	content := strings.Replace(
		validP3ArtifactForState(t, state),
		"## 角色表演档案",
		"说明文字提到 ## 角色表演档案，但这不是 Markdown 标题",
		1,
	)
	images := []model.ManjuProductionImage{readyStoryboardImageForTest("段1")}
	if err := validateP3Delivery(content, state, images); err == nil {
		t.Fatal("substring mentions must not satisfy required section headings")
	}
}

func TestP3DeliveryChecklistRequiresSingleConfiguredTable(t *testing.T) {
	state := p3StateWithShotCountsForTest(t, map[string]int{"段1": 1})
	exemptContent := p3ArtifactWithSingleChecklistRow(
		t,
		state,
		"⛔豁免（单镜过场段）",
		"理由：动作相位已按单镜过场完整列入分镜总表。",
	)
	checklist := p3SingleSegmentChecklistTable(
		"⛔豁免（单镜过场段）",
		"理由：动作相位已按单镜过场完整列入分镜总表。",
	)

	t.Run("loose storyboard lines do not satisfy", func(t *testing.T) {
		content := p3ReplaceChecklistTable(
			exemptContent,
			"故事板图：段1 ⛔豁免（单镜过场段） 理由：动作相位已列。",
		)
		if err := validateP3Delivery(content, state, nil); err == nil {
			t.Fatal("loose storyboard exemption line must not satisfy the checklist table")
		}
	})

	t.Run("duplicate checklist tables fail", func(t *testing.T) {
		content := strings.Replace(exemptContent, checklist, checklist+"\n\n"+checklist, 1)
		if err := validateP3Delivery(content, state, nil); err == nil {
			t.Fatal("duplicate checklist tables must fail")
		}
	})

	t.Run("configured header order is exact", func(t *testing.T) {
		content := strings.Replace(
			validP3ArtifactForState(t, state),
			"| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |",
			"| 剧情分镜格/覆盖 | 段 | 故事板图格数 | 具体说明/豁免理由 |",
			1,
		)
		content = strings.Replace(
			content,
			"| 段1 | 覆盖：场1 ✅ 四宫格 | 4格(2×2 四宫格) | 无豁免；等待已就绪故事板图。 |",
			"| 覆盖：场1 ✅ 四宫格 | 段1 | 4格(2×2 四宫格) | 无豁免；等待已就绪故事板图。 |",
			1,
		)
		images := []model.ManjuProductionImage{readyStoryboardImageForTest("段1")}
		if err := validateP3Delivery(content, state, images); err == nil {
			t.Fatal("reordered checklist headers must fail")
		}
	})
}

func TestP3P4ContractsPinOfficialSources(t *testing.T) {
	p3, err := loadP3StoryboardContract()
	if err != nil {
		t.Fatal(err)
	}
	p4, err := loadP4FeedContract()
	if err != nil {
		t.Fatal(err)
	}
	if p3.RulesVersion != manjuSkillVersion || p4.RulesVersion != manjuSkillVersion {
		t.Fatalf(
			"contract rules version mismatch: p3=%s p4=%s runtime=%s",
			p3.RulesVersion,
			p4.RulesVersion,
			manjuSkillVersion,
		)
	}

	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range append(append([]contractSource{}, p3.Sources...), p4.Sources...) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source.Path)))
		if err != nil {
			t.Fatalf("read contract source %s: %v", source.Path, err)
		}
		sum := sha256.Sum256(data)
		if got := fmt.Sprintf("%x", sum); got != source.SHA256 {
			t.Fatalf("contract source is stale: path=%s got=%s want=%s", source.Path, got, source.SHA256)
		}
	}
}

func TestP3RuntimePromptDoesNotDuplicateSharedBusinessContract(t *testing.T) {
	contract, err := loadP3StoryboardContract()
	if err != nil {
		t.Fatal(err)
	}
	instruction := productionStageInstruction(ProductionStageStoryboard, AssetModeM1Value)
	for _, forbidden := range append(append([]string{}, contract.RequiredSections...), contract.Prompt.RequiredItems...) {
		if strings.Contains(instruction, forbidden) {
			t.Fatalf("P3 runtime prompt duplicated official business contract %q:\n%s", forbidden, instruction)
		}
	}

	stateInstruction := projectStatePatchInstruction("P3")
	for _, required := range []string{
		contract.ProjectState.StageKey,
		"\"segments\"",
		"\"shots\"",
		"\"storyboard_panels\"",
	} {
		if !strings.Contains(stateInstruction, required) {
			t.Fatalf("P3 state patch is missing contract value %q:\n%s", required, stateInstruction)
		}
	}
	if strings.Contains(stateInstruction, contract.ProjectState.Instruction) {
		t.Fatalf("P3 state transport block repeated business guidance:\n%s", stateInstruction)
	}
	if strings.Contains(instruction, "700 字符") || strings.Contains(instruction, "1200 字符") {
		t.Fatalf("P3 contract must not invent fixed per-shot thresholds:\n%s", instruction)
	}
}

func TestP3ContractRequiresSegmentAssetRefs(t *testing.T) {
	contract, err := loadP3StoryboardContract()
	if err != nil {
		t.Fatal(err)
	}
	segments := anySlice(contract.ProjectState.Example["segments"])
	if len(segments) == 0 {
		t.Fatal("P3 project_state example must contain segments")
	}
	example := anyMap(segments[0])
	refs := anyMap(example["asset_refs"])
	if anyString(example["scene"]) == "" ||
		len(anySlice(refs["scenes"])) == 0 ||
		len(anySlice(refs["characters"])) == 0 {
		t.Fatalf("P3 project_state example must declare scene and asset_refs: %+v", example)
	}
	instruction := strings.Join(contract.Prompt.RequiredItems, "\n") + contract.ProjectState.Instruction
	for _, token := range []string{
		"asset_refs",
		"本段实际引用",
		"不得填写项目全量资产",
		"1 个场景",
		"人物数",
		"道具数",
		"≤",
	} {
		if !strings.Contains(instruction, token) {
			t.Fatalf("P3 contract missing %q:\n%s", token, instruction)
		}
	}
	if contract.AssetRefs.LimitByModel["2.0"] != 9 ||
		contract.AssetRefs.LimitByModel["2.5"] != 30 ||
		contract.AssetRefs.LimitByModel["h3"] != 9 {
		t.Fatalf("P3 asset limits must match official gates: %+v", contract.AssetRefs.LimitByModel)
	}
}

func TestP3ContractIncludesOfficialC9HighRiskWords(t *testing.T) {
	contract, err := loadP3StoryboardContract()
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.Safety.HighRiskWords) == 0 {
		t.Fatal("P3 safety contract must include official C9 high-risk words")
	}
	words := strings.Join(contract.Safety.HighRiskWords, "\n")
	for _, required := range []string{"撕裂", "鲜血", "断肢", "自杀", "裸露"} {
		if !strings.Contains(words, required) {
			t.Fatalf("P3 C9 word list missing %q", required)
		}
	}
	prompt := p3StoryboardPrompt()
	for _, required := range []string{"C9 平台高危词硬门", "撕裂", "禁止用否定式"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("P3 prompt missing C9 guidance %q", required)
		}
	}
}

func TestPreviewCompiledFeedSegmentsReportsActualGateBody(t *testing.T) {
	body := "【画幅风格】9:16 竖屏\n【场景资产】@旧宅\n【核心人物】@林澈（参考图1）\n【时间轴分镜】\n00:00-00:04 [镜头1] 中景。林澈推门。声音：木门声。台词：无。\n【音效】木门声\n【强制禁止项】无字幕"
	feed := "## 【投喂单元 1/1 · 段11】\n```text\n" + body + "\n```\n"
	segments := previewCompiledFeedSegments(feed, 2000)
	if len(segments) != 1 {
		t.Fatalf("expected one compiled segment: %+v", segments)
	}
	if segments[0].TotalChars != len([]rune(body)) {
		t.Fatalf("gate body count mismatch: got=%d want=%d", segments[0].TotalChars, len([]rune(body)))
	}
	if segments[0].Characters == 0 || segments[0].SceneProps == 0 || segments[0].Shots == 0 {
		t.Fatalf("breakdown must classify compiled fields: %+v", segments[0])
	}
}

func TestManjuAssetRefKeyCatalogUsesExactProjectStateKeys(t *testing.T) {
	state := map[string]any{
		"characters": map[string]any{
			"守城兵":    map[string]any{"id": "CHR-01", "fig_no": 8},
			"城门守兵群像": map[string]any{"id": "CHR-02", "fig_no": 9},
		},
		"scenes": map[string]any{
			"太原城门": map[string]any{"id": "SCN-01", "fig_no": 10},
		},
		"props": map[string]any{
			"城门门闩": map[string]any{"id": "PRP-01", "fig_no": 11},
		},
	}
	catalog := manjuAssetRefKeyCatalog(state)
	for _, required := range []string{
		"【asset_refs 精确键目录】",
		"`守城兵`",
		"`城门守兵群像`",
		"`太原城门`",
		"`城门门闩`",
		"必须逐字复制",
	} {
		if !strings.Contains(catalog, required) {
			t.Fatalf("精确键目录缺少 %q:\n%s", required, catalog)
		}
	}
	artifactCatalog := manjuAssetRefKeyCatalogFromAssetArtifacts([]model.ManjuAssetArtifact{{
		Content: "<!-- MANJU_PROJECT_STATE_PATCH_JSON\n" +
			`{"characters":{"守城兵":{"id":"CHR-01","fig_no":8}},"scenes":{"太原城门":{"id":"SCN-01","fig_no":10}},"props":{}}` +
			"\n-->",
	}})
	if !strings.Contains(artifactCatalog, "`守城兵`") || !strings.Contains(artifactCatalog, "`太原城门`") {
		t.Fatalf("缺失 state 时必须从 P2 工件恢复精确键目录:\n%s", artifactCatalog)
	}
}

func TestCompileStatePreservesC9WordsForOfficialValidation(t *testing.T) {
	state := defaultProjectStateMapForTest(t)
	segments := anySlice(state["segments"])
	segment := anyMap(segments[0])
	shots := anySlice(segment["shots"])
	shot := anyMap(shots[0])
	shot["action"] = strings.ReplaceAll(
		anyString(shot["action"]),
		"画面主体",
		"衣甲表面有血污，画面主体",
	)
	shots[0] = shot
	segment["shots"] = shots
	segments[0] = segment
	state["segments"] = segments
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	prepared, err := defaultManjuSkillRuntime.prepareOfficialCompileState(string(raw), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1",
		Aspect: "16:9", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prepared, "血污") {
		t.Fatalf("BeefTV rewrote C9 content before official validation:\n%s", prepared)
	}
}

func defaultProjectStateMapForTest(t *testing.T) map[string]any {
	t.Helper()
	raw, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestP4ValidationAndStylePlacementUseSharedContract(t *testing.T) {
	contract, err := loadP4FeedContract()
	if err != nil {
		t.Fatal(err)
	}
	args := p4ValidateArgs("feed.md", "state.json", ManjuSkillValidationOptions{
		Model: "seedance-2.0", Track: "1", Combat: "R2", Aspect: "9:16", StyleSource: "路阳",
	})
	for _, required := range contract.Validator.RequiredArgs {
		if !containsString(args, required) {
			t.Fatalf("P4 validator args are missing contract arg %q: %v", required, args)
		}
	}
	if containsString(args, "--whole") {
		t.Fatalf("P4 contract must validate by segment, got args: %v", args)
	}

	styleMarker, performanceMarker, position, err := p4StyleLockMarkers()
	if err != nil {
		t.Fatal(err)
	}
	if position != contract.StyleLock.InjectPosition {
		t.Fatalf("P4 style position mismatch: got=%s want=%s", position, contract.StyleLock.InjectPosition)
	}
	section := strings.Join([]string{
		"## 【投喂单元 1/1 · 段1】",
		"```text",
		"【画幅风格】16:9 横屏",
		"【时间轴分镜】",
		"00:00-00:04 [镜头1] 中景 · 35mm · 平视 · 固定（定场）。动作。声音：环境音。台词：无。",
		"```",
	}, "\n")
	withStyle := injectStyleLockBlocksIntoUnit(section, styleMarker+"\n光比 [1:4, 1:9]\n镜头 [24mm, 85mm]\n\n"+performanceMarker)
	codeStart := strings.Index(withStyle, "```text")
	codeEnd := -1
	if codeStart >= 0 {
		if offset := strings.Index(withStyle[codeStart+len("```text"):], "\n```"); offset >= 0 {
			codeEnd = codeStart + len("```text") + offset
		}
	}
	if codeStart < 0 || codeEnd < 0 {
		t.Fatalf("style witness lost text fence:\n%s", withStyle)
	}
	styleIndex := strings.Index(withStyle, styleMarker)
	if styleIndex <= codeStart || styleIndex >= codeEnd {
		t.Fatalf("style block must be inside each unit text block:\n%s", withStyle)
	}
}

func TestP4RuntimePromptDoesNotDuplicateSharedFeedContract(t *testing.T) {
	contract, err := loadP4FeedContract()
	if err != nil {
		t.Fatal(err)
	}
	instruction := productionStageInstruction(ProductionStageFeed, AssetModeM1Value)
	for _, forbidden := range contract.Sections {
		if strings.Contains(instruction, forbidden) {
			t.Fatalf("P4 runtime prompt duplicated official feed section %q:\n%s", forbidden, instruction)
		}
	}
	for _, required := range []string{
		contract.StyleLock.StyleMarker,
		contract.StyleLock.PerformanceMarker,
		contract.StyleLock.InjectPosition,
		contract.Validator.Script,
		contract.Gate.Script,
	} {
		if strings.Contains(instruction, required) {
			t.Fatalf("P4 runtime prompt duplicated official feed contract %q:\n%s", required, instruction)
		}
	}
}

func TestP4RepairRoutesComeFromSharedContract(t *testing.T) {
	contract, err := loadP4FeedContract()
	if err != nil {
		t.Fatal(err)
	}
	for code, stage := range map[string]string{
		"C23a":    "beef_tv_adapter",
		"C23b":    "P3",
		"C23d2":   "beef_tv_adapter",
		"C23f":    "P3",
		"CT5-P45": "P4.5",
	} {
		if got := contract.RepairRoutes[code]; got != stage {
			t.Fatalf("repair route contract mismatch: code=%s got=%s want=%s", code, got, stage)
		}
		if got := p4RepairStageForIssue("[段1] " + code + " 示例失败"); got != stage {
			t.Fatalf("repair route resolve mismatch: code=%s got=%s want=%s", code, got, stage)
		}
	}
}

func TestOfficialStyleLensRangeIsAppliedToP3Prompt(t *testing.T) {
	lensRange, found, err := styleLensRangeForSource("徐克")
	if err != nil {
		t.Fatal(err)
	}
	if !found || lensRange.Min != 21 || lensRange.Max != 28 {
		t.Fatalf("official style lens range mismatch: found=%v range=%+v", found, lensRange)
	}
	instruction := p3StoryboardPromptWithLensRange(&lensRange)
	for _, required := range []string{
		"P0 画风焦段硬锁",
		"21mm",
		"28mm",
		"所有 shot.spec 的焦段必须落在该区间",
		"P0B 核心机位组合与本硬锁冲突时，以本硬锁为准",
		"保留景别、机位角度和运镜语义，重新选择区间内焦段",
		"越界时必须回 P3",
	} {
		if !strings.Contains(instruction, required) {
			t.Fatalf("P3 prompt is missing locked lens constraint %q:\n%s", required, instruction)
		}
	}
}

func TestP4ValidationDefersP45ContractGateToDedicatedStage(t *testing.T) {
	contract, err := loadP4FeedContract()
	if err != nil {
		t.Fatal(err)
	}
	active, deferred := splitP4DeferredContractIssues([]string{
		"[段1] C23b 镜头1 镜头 35mm 越界",
		"C31 CT5-P45 缺《预演故事板》（触发：跨 10 个场景）",
	})
	if len(active) != 1 || !strings.Contains(active[0], "C23b") {
		t.Fatalf("C23b must remain an active P4 error: %+v", active)
	}
	if len(deferred) != 1 || !strings.Contains(deferred[0], "CT5-P45") {
		t.Fatalf("CT5-P45 must be deferred to P4.5: %+v", deferred)
	}
	if len(contract.DeferredContractIssues.Codes) != 1 ||
		contract.DeferredContractIssues.Codes[0] != "CT5-P45" ||
		contract.DeferredContractIssues.Owner != "P4.5" {
		t.Fatalf("P4 contract does not declare the deferred P4.5 gate: %+v", contract.DeferredContractIssues)
	}
}

func TestDecodeP4ValidationOutputWithInformationalPrefix(t *testing.T) {
	output := `[i] 分单元校验：识别到 16 个投喂段
[i] 段1：时长15.0s 镜头3 字符1264 | FAIL=2 WARN=24
{
  "issues": [
    "C31 CT5-P45 缺《预演故事板》（触发：跨 10 个场景）"
  ],
  "warnings": [
    "C19c 示例告警"
  ],
  "stats": {
    "segments": 16
  }
}`
	parsed, ok := decodeP4ValidationOutput(output)
	if !ok {
		t.Fatal("P4 validator output with informational prefix must still expose its JSON result")
	}
	if len(parsed.Issues) != 1 || !strings.Contains(parsed.Issues[0], "CT5-P45") {
		t.Fatalf("P4 validator issues were not decoded: %+v", parsed)
	}
	if len(parsed.Warnings) != 1 || !strings.Contains(parsed.Warnings[0], "C19c") {
		t.Fatalf("P4 validator warnings were not decoded: %+v", parsed)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validP3StateForTest(t *testing.T) map[string]any {
	t.Helper()
	return p3StateWithShotCountsForTest(t, map[string]int{"段1": 2})
}

func p3StateWithSegmentsForTest(t *testing.T, codes ...string) map[string]any {
	t.Helper()
	shotCounts := make(map[string]int, len(codes))
	for _, code := range codes {
		shotCounts[code] = 2
	}
	return p3StateWithShotCountsForTest(t, shotCounts)
}

func p3StateWithShotCountsForTest(t *testing.T, shotCounts map[string]int) map[string]any {
	t.Helper()
	segments := make([]any, 0, len(shotCounts))
	for code, shotCount := range shotCounts {
		shots := make([]any, 0, shotCount)
		for index := 0; index < shotCount; index++ {
			shots = append(shots, map[string]any{
				"no":       index + 1,
				"start":    index * 3,
				"end":      (index + 1) * 3,
				"spec":     "中景 · 35mm · 侧45度 · 固定",
				"action":   "林澈推门",
				"sound":    "木门吱呀",
				"dialogue": "无",
			})
		}
		segments = append(segments, map[string]any{
			"seg_id": code,
			"scene":  "旧宅",
			"asset_refs": map[string]any{
				"scenes":     []any{"旧宅"},
				"characters": []any{"林澈"},
				"props":      []any{},
			},
			"shots": shots,
		})
	}
	return map[string]any{
		"characters": map[string]any{"林澈": map[string]any{"id": "CHR-01", "fig_no": 1}},
		"scenes":     map[string]any{"旧宅": map[string]any{"id": "SCN-01", "fig_no": 2}},
		"props":      map[string]any{},
		"segments":   segments,
	}
}

func validP3ArtifactWithout(t *testing.T, headings ...string) string {
	t.Helper()
	content := validP3ArtifactForState(t, validP3StateForTest(t))
	for _, heading := range headings {
		content = strings.ReplaceAll(content, heading, "## 已移除不合规章节")
	}
	return content
}

func validP3ArtifactForState(t *testing.T, state map[string]any) string {
	t.Helper()
	return validP3ArtifactForStateWithExemptions(t, state, nil)
}

func validP3ArtifactForStateWithExemptions(
	t *testing.T,
	state map[string]any,
	exemptions map[string]string,
) string {
	t.Helper()
	segments := anySlice(state["segments"])
	var out strings.Builder
	out.WriteString(`# P3 分镜与故事板

## 角色表演档案

角色表演档案 · 林澈。克制的青年，肩背常紧。声音偏低，紧张时更轻。

## 逐场节拍表

| 场 | 目标 | 障碍 | 战术 | 转折 |
|---|---|---|---|---|
| 场1 | 进门 | 门锁 | 推门 | 门开 |

## 导演稿

文字平面图：门在画面右侧，林澈从左侧入画，轴线保持左林澈右门。

## 分镜总表

| 段号 | 镜号 | 时长 | 画面内容 | 台词 | 音效 | 资产引用 |
|---|---|---|---|---|---|---|
`)
	for _, raw := range segments {
		segment := anyMap(raw)
		code := anyString(segment["seg_id"])
		for index := range anySlice(segment["shots"]) {
			fmt.Fprintf(&out, "| %s | %02d | 3s | 林澈推门 | 无 | 木门吱呀 | CHR-林澈 |\n", code, index+1)
		}
	}
	out.WriteString("\n## 故事板表\n")
	for index, raw := range segments {
		segment := anyMap(raw)
		code := fallbackValue(anyString(segment["seg_id"]), fmt.Sprintf("段%d", index+1))
		fmt.Fprintf(&out, `
### 第%d段｜场景%d

| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 | 面板边界备注 |
|---|---|---|---|---|---|---|
| %s | %s-P1 | 起始 | 中景 · 侧45度 | 林澈在左前景看门 | CHR-林澈 | 入口帧 |
| %s | %s-P2 | 过程 | 近景 · 固定 | 右手推门 | CHR-林澈 | 动作连续 |
| %s | %s-P3 | 反应 | 中近景 · 固定 | 视线看向门内 | CHR-林澈 | 不剪 |
| %s | %s-P4 | 出口 | 中景 · 固定 | 人物停在门内 | CHR-林澈 | 出口帧 |
`, index+1, index+1, code, code, code, code, code, code, code, code)
	}
	out.WriteString(`
## 台词精简建议清单

本稿全量承接上游台词，无精简建议。

## P3 交付清单

| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |
|---|---|---|---|
`)
	for index, raw := range segments {
		segment := anyMap(raw)
		code := fallbackValue(anyString(segment["seg_id"]), fmt.Sprintf("段%d", index+1))
		storyboardCell := "4格(2×2 四宫格)"
		reason := "无豁免；等待已就绪故事板图。"
		if exemptionReason, ok := exemptions[code]; ok {
			storyboardCell = "⛔豁免（单镜过场段）"
			reason = exemptionReason
		}
		fmt.Fprintf(&out, "| %s | 覆盖：场%d ✅ 四宫格 | %s | %s |\n", code, index+1, storyboardCell, reason)
	}
	return out.String()
}

func p3ArtifactWithSingleChecklistRow(
	t *testing.T,
	state map[string]any,
	storyboardCell string,
	reason string,
) string {
	t.Helper()
	content := validP3ArtifactForState(t, state)
	return p3ReplaceChecklistTable(
		content,
		p3SingleSegmentChecklistTable(storyboardCell, reason),
	)
}

func p3SingleSegmentChecklistTable(storyboardCell, reason string) string {
	return "| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |\n" +
		"|---|---|---|---|\n" +
		"| 段1 | 覆盖：场1 ✅ 四宫格 | " + storyboardCell + " | " + reason + " |"
}

func p3ReplaceChecklistTable(content, replacement string) string {
	start := strings.Index(content, "| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |")
	if start < 0 {
		return content
	}
	end := strings.Index(content[start:], "\n\n<!-- MANJU_PROJECT_STATE_PATCH_JSON")
	if end < 0 {
		end = len(content) - start
	}
	return content[:start] + replacement + content[start+end:]
}

func readyStoryboardImageForTest(code string) model.ManjuProductionImage {
	return model.ManjuProductionImage{
		ID:          "storyboard-" + code,
		SegmentCode: code,
		Status:      ProductionImageStatusReady,
		ResourceID:  "resource-" + code,
		Prompt:      "生成一张 16:9 横版的故事板图（CLEAN_GRID 干净版）",
	}
}
