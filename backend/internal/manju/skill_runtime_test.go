package manju

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const testP3NoDialogueBaseline = "台词：无\n"

func compileFeedWithTestBaseline(stateJSON string, options ManjuSkillValidationOptions) (string, ManjuSkillValidationResult, error) {
	options.UpstreamScript = testP3NoDialogueBaseline
	return defaultManjuSkillRuntime.CompileFeed(stateJSON, options)
}

func TestOfficialV7426CompileFeedFromProjectState(t *testing.T) {
	state, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	feed, result, err := compileFeedWithTestBaseline(state, ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "R1", Mode: "M1", Aspect: "16:9", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("官方 compile_feed 产物未过机检: %+v\n%s", result, feed)
	}
	for _, required := range []string{
		"> 素材形态：M1",
		"【画幅风格】",
		"【场景资产】",
		"【核心人物】",
		"【站位声明】",
		"【时间轴分镜】",
		"【音效】",
		"【强制禁止项】",
		"## 素材接线单",
	} {
		if !strings.Contains(feed, required) {
			t.Fatalf("官方编译投喂稿缺少 %q:\n%s", required, feed)
		}
	}
}

func TestCompileFeedUsesOnlySegmentAssetRefs(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	characters := anyMap(state["characters"])
	characters["林澈"] = map[string]any{
		"id": "CHR-01", "fig_no": 11, "voice_lock": "清亮青年声",
	}
	characters["陈枭"] = map[string]any{
		"id": "CHR-02", "fig_no": 12, "voice_lock": "低沉中年声",
	}
	state["characters"] = characters
	props := anyMap(state["props"])
	props["旧怀表"] = map[string]any{
		"id": "PRP-01", "fig_no": 13, "material_desc": "黄铜旧表",
	}
	state["props"] = props
	segments := anySlice(state["segments"])
	if len(segments) == 0 {
		t.Fatal("默认 project_state 缺少 segments")
	}
	first := anyMap(segments[0])
	scene := anyString(first["scene"])
	if scene == "" {
		t.Fatal("默认段缺少 scene")
	}
	first["asset_refs"] = map[string]any{
		"scenes":     []any{scene},
		"characters": []any{"林澈"},
		"props":      []any{"旧怀表"},
	}
	firstBytes, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var second map[string]any
	if err := json.Unmarshal(firstBytes, &second); err != nil {
		t.Fatal(err)
	}
	second["seg_id"] = "段2"
	second["asset_refs"] = map[string]any{
		"scenes":     []any{scene},
		"characters": []any{"陈枭"},
		"props":      []any{},
	}
	segments = []any{first, second}
	state["segments"] = segments
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9", Assets: 99, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("编译失败: %v\n%s", err, feed)
	}
	if !result.Passed {
		t.Fatalf("分段资产编译结果未过机检: %+v\n%s", result, feed)
	}
	unitOne := strings.Index(feed, "## 【投喂单元 1/2")
	unitTwo := strings.Index(feed, "## 【投喂单元 2/2")
	if unitOne < 0 || unitTwo < 0 {
		t.Fatalf("缺少两个投喂单元:\n%s", feed)
	}
	firstUnit := feed[unitOne:unitTwo]
	secondUnit := feed[unitTwo:]
	firstCore := feedFieldLine(firstUnit, "【核心人物】")
	secondCore := feedFieldLine(secondUnit, "【核心人物】")
	firstScene := feedFieldLine(firstUnit, "【场景资产】")
	secondScene := feedFieldLine(secondUnit, "【场景资产】")
	if !strings.Contains(firstCore, "@林澈") || strings.Contains(firstCore, "@陈枭") {
		t.Fatalf("段1核心人物只能注入自身资产:\n%s", firstUnit)
	}
	if !strings.Contains(firstScene, "@PRP-旧怀表") {
		t.Fatalf("段1必须注入旧怀表:\n%s", firstUnit)
	}
	if !strings.Contains(secondCore, "@陈枭") || strings.Contains(secondCore, "@林澈") {
		t.Fatalf("段2核心人物只能注入自身资产:\n%s", secondUnit)
	}
	if strings.Contains(secondScene, "@PRP-旧怀表") {
		t.Fatalf("段2不得继承旧怀表:\n%s", secondUnit)
	}
}

func TestCompileFeedLargeProjectUsesPerSegmentAssetRefs(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	characters := map[string]any{}
	for index := 1; index <= 20; index++ {
		characters[fmt.Sprintf("角色%d", index)] = map[string]any{
			"fig_no": index, "voice_lock": "清晰稳定角色声",
		}
	}
	props := map[string]any{}
	for index := 1; index <= 15; index++ {
		props[fmt.Sprintf("道具%d", index)] = map[string]any{
			"fig_no": 20 + index, "material_desc": "哑光实体材质",
		}
	}
	scenes := map[string]any{
		"场景1": map[string]any{
			"fig_no": 40, "coords": "北侧门，南侧窗",
			"set_states":  map[string]any{"default": "清晨"},
			"media_layer": "薄尘",
			"lighting":    "清晨冷光从窗外照入",
		},
	}
	segments := make([]any, 0, 16)
	for index := 0; index < 16; index++ {
		segments = append(segments, map[string]any{
			"seg_id": fmt.Sprintf("段%d", index+1),
			"scene":  "场景1",
			"asset_refs": map[string]any{
				"scenes":     []any{"场景1"},
				"characters": []any{"角色1", "角色2", "角色3", "角色4"},
				"props":      []any{"道具1", "道具2", "道具3"},
			},
			"segment_type": "对话戏",
			"standing":     []any{"角色1位于画面左侧"},
			"shots": []any{
				map[string]any{
					"no": 1, "start": 0, "end": 4,
					"spec":     "中景 · 35mm · 侧45度 · 固定",
					"action":   "角色1抬手示意，角色2点头回应",
					"sound":    "衣料摩擦",
					"dialogue": "无",
				},
			},
			"sfx_summary": "衣料摩擦",
		})
	}
	state["characters"] = characters
	state["props"] = props
	state["scenes"] = scenes
	state["segments"] = segments
	anyMap(state["locks"])["model"] = "2.0"
	anyMap(state["locks"])["aspect"] = "9:16"
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.0", Track: "1", Combat: "none", Mode: "M1", Aspect: "9:16", Assets: 99, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("大项目分段资产编译失败: %v\n%s", err, feed)
	}
	if !result.Passed {
		t.Fatalf("大项目分段资产编译必须通过官方闸门: %+v\n%s", result, feed)
	}
	if got := len(manjuUnitHeadingPattern.FindAllStringSubmatch(feed, -1)); got != 16 {
		t.Fatalf("必须输出 16 个投喂单元，实际 %d", got)
	}
	for index, match := range manjuUnitHeadingPattern.FindAllStringSubmatchIndex(feed, -1) {
		end := len(feed)
		if index+1 < len(manjuUnitHeadingPattern.FindAllStringSubmatchIndex(feed, -1)) {
			next := manjuUnitHeadingPattern.FindAllStringSubmatchIndex(feed, -1)[index+1]
			end = next[0]
		}
		unit := feed[match[0]:end]
		for _, unwanted := range []string{"角色5", "角色20", "道具4", "道具15"} {
			if strings.Contains(feedFieldLine(unit, "【核心人物】"), "@"+unwanted) ||
				strings.Contains(feedFieldLine(unit, "【场景资产】"), "@PRP-"+unwanted) {
				t.Fatalf("段%d 注入了未声明资产 %s:\n%s", index+1, unwanted, unit)
			}
		}
	}
}

func TestCompileFeedAppliesOnlyDeclaredCharacterStates(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	characters := anyMap(state["characters"])
	characters["陈枭"] = map[string]any{
		"fig_no":     12,
		"voice_lock": "低沉中年声",
		"char_states": map[string]any{
			"S1_helmet": map[string]any{
				"item": "黑色头盔", "action": "摘下黑色头盔", "consequence": "额前头发被压扁",
			},
			"S2_cloak": map[string]any{
				"item": "灰色披风", "action": "解开灰色披风", "consequence": "肩部轮廓变化",
			},
		},
	}
	state["characters"] = characters
	segments := anySlice(state["segments"])
	segment := anyMap(segments[0])
	segment["asset_refs"] = map[string]any{
		"scenes":     []any{segment["scene"]},
		"characters": []any{"陈枭"},
		"props":      []any{},
		"character_states": map[string]any{
			"陈枭": []any{"S1_helmet"},
		},
	}
	segments[0] = segment
	state["segments"] = segments
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	feed, _, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9", Assets: 99, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	core := feedFieldLine(feed, "【核心人物】")
	if !strings.Contains(core, "黑色头盔") || strings.Contains(core, "灰色披风") {
		t.Fatalf("only the declared character state may be injected:\n%s", core)
	}
}

func feedFieldLine(feed, field string) string {
	for _, line := range strings.Split(feed, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), field) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func TestCompileFeedPreservesStateShotTimesWithoutRewrite(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	segments := anySlice(state["segments"])
	if len(segments) == 0 {
		t.Fatal("默认 project_state 缺少 segments")
	}
	first := anyMap(segments[0])
	firstShots := anySlice(first["shots"])
	if len(firstShots) == 0 {
		t.Fatal("默认 project_state 缺少 shots")
	}
	originalFirstEnd := int(anyNumber(anyMap(firstShots[len(firstShots)-1])["end"]))
	const firstEnd = 30
	for index, rawShot := range firstShots {
		shot := anyMap(rawShot)
		shot["start"] = int(float64(anyNumber(shot["start"])) * firstEnd / float64(originalFirstEnd))
		shot["end"] = int(float64(anyNumber(shot["end"])) * firstEnd / float64(originalFirstEnd))
		firstShots[index] = shot
	}
	first["shots"] = firstShots
	cloneBytes, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var second map[string]any
	if err := json.Unmarshal(cloneBytes, &second); err != nil {
		t.Fatal(err)
	}
	second["seg_id"] = "段2"
	secondShots := anySlice(second["shots"])
	for index, rawShot := range secondShots {
		shot := anyMap(rawShot)
		shot["start"] = int(anyNumber(shot["start"])) + firstEnd
		shot["end"] = int(anyNumber(shot["end"])) + firstEnd
		secondShots[index] = shot
	}
	second["shots"] = secondShots
	state["segments"] = append(segments, second)
	anyMap(state["locks"])["aspect"] = "9:16"
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, _, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "9:16", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	secondStart := strings.Index(feed, "## 【投喂单元 2/2 · 段2】")
	if secondStart < 0 {
		t.Fatalf("缺少第二投喂单元:\n%s", feed)
	}
	secondUnit := feed[secondStart:]
	if !strings.Contains(secondUnit, "00:30-") {
		t.Fatalf("BeefTV 不得改写官方编译器的累计时码:\n%s", secondUnit)
	}
}

func TestCompileFeedPreservesOverlongOfficialUnitAndLetsValidationFail(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	segments := anySlice(state["segments"])
	if len(segments) == 0 {
		t.Fatal("默认 project_state 缺少 segments")
	}
	segment := anyMap(segments[0])
	shots := anySlice(segment["shots"])
	if len(shots) < 2 {
		t.Fatal("测试需要至少两个镜头")
	}
	originalEnd := int(anyNumber(anyMap(shots[len(shots)-1])["end"]))
	const targetEnd = 50
	for index, rawShot := range shots {
		shot := anyMap(rawShot)
		shot["start"] = int(float64(anyNumber(shot["start"])) * targetEnd / float64(originalEnd))
		shot["end"] = int(float64(anyNumber(shot["end"])) * targetEnd / float64(originalEnd))
		shots[index] = shot
	}
	segment["shots"] = shots
	segments[0] = segment
	state["segments"] = segments
	anyMap(state["locks"])["aspect"] = "9:16"
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "9:16", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	if result.Passed {
		t.Fatalf("50 秒官方投喂单元应被官方机检拒绝: %+v\n%s", result, feed)
	}
	if strings.Contains(feed, "段1-2") || strings.Contains(feed, "段1-3") {
		t.Fatalf("BeefTV 不得自行拆分官方投喂单元:\n%s", feed)
	}
	if !strings.Contains(result.Output, "C1") {
		t.Fatalf("官方机检应明确报告段时长超限: %+v", result)
	}
}

func TestCompileFeedFailsClosedWithoutUpstreamScript(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := defaultManjuSkillRuntime.CompileFeed(rawState, ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9",
		Assets: 9, Timeout: 120 * time.Second,
	})
	if err == nil && result.Passed {
		t.Fatalf("缺少锁定 P3 台词基线时 P4 不得 PASS: %+v\n%s", result, feed)
	}
	message := strings.Join(result.Issues, "\n") + "\n" + result.Output
	if !strings.Contains(message, "台词") {
		t.Fatalf("缺少台词基线时必须明确报告台词完整性失败: %+v", result)
	}
	if !strings.Contains(feed, "## 【投喂单元") {
		t.Fatalf("硬门失败前仍应保留 compile_feed.py 本次输出供修正:\n%s", feed)
	}
}

func TestValidateDialogueIntegrityRejectsMissingDialogue(t *testing.T) {
	upstream := "台词：无（画外音延续）：林澈：\"别开门。\"\n"
	feed := "## 段1\n【台词】无\n"

	result, err := defaultManjuSkillRuntime.ValidateDialogueIntegrity(upstream, feed, 30*time.Second)
	if err != nil {
		t.Fatalf("script_feed_diff.py 正常返回 exit 2 时不应包装为基础设施错误: %v", err)
	}
	if result.Passed || result.ExitCode != 2 {
		t.Fatalf("缺失台词必须按 exit 2 硬失败: %+v", result)
	}
	if !strings.Contains(strings.Join(result.Issues, "\n"), "台词") {
		t.Fatalf("台词缺失失败必须保留可操作明细: %+v", result)
	}
}

func TestValidateDialogueIntegrityKeepsExitOneWarnings(t *testing.T) {
	upstream := "台词：林澈：\"别开门，外面有人，快走，千万不要回头。\"\n"
	feed := "【林澈（低沉）：\"别开门，外面有人，快走。\"】\n"

	result, err := defaultManjuSkillRuntime.ValidateDialogueIntegrity(upstream, feed, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.ExitCode != 1 {
		t.Fatalf("部分覆盖属于可操作警告而不是硬失败: %+v", result)
	}
	if len(result.DialogueWarnings) == 0 {
		t.Fatalf("exit 1 警告必须保留在结果中: %+v", result)
	}
}

func TestValidateDialogueIntegrityRequiresLockedP3Baseline(t *testing.T) {
	_, err := defaultManjuSkillRuntime.ValidateDialogueIntegrity(" \n", "## 段1\n", 30*time.Second)
	if !errors.Is(err, ErrProjectStateUnavailable) {
		t.Fatalf("空 P3 台词基线必须失败关闭，得到 %v", err)
	}
}

func TestValidateDialogueIntegrityTimeoutFailsClosed(t *testing.T) {
	result, err := defaultManjuSkillRuntime.ValidateDialogueIntegrity(
		"台词：无\n",
		"## 段1\n",
		time.Nanosecond,
	)
	if err != nil {
		t.Fatalf("超时应以失败结果返回而不是放行: %v", err)
	}
	if result.Passed {
		t.Fatalf("script_feed_diff.py 超时时不得 PASS: %+v", result)
	}
}

func TestP4ExitZeroRequiresValidatorJSON(t *testing.T) {
	tests := map[string]bool{
		"":                            false,
		"validator finished":          false,
		"{}":                          false,
		`{"issues":[],"warnings":[]}`: true,
	}
	for output, want := range tests {
		if got := validP4ValidatorOutput(output); got != want {
			t.Fatalf("validP4ValidatorOutput(%q) = %v, want %v", output, got, want)
		}
	}
}

func TestP4ExitZeroRequiresCanonicalGateSummary(t *testing.T) {
	tests := map[string]bool{
		"":                  false,
		"gate finished":     false,
		"汇总：FAIL=1  WARN=0": false,
		"汇总：FAIL=0  WARN=2": true,
	}
	for output, want := range tests {
		if got := validP4GateOutput(output); got != want {
			t.Fatalf("validP4GateOutput(%q) = %v, want %v", output, got, want)
		}
	}
}

func TestCompileFeedDoesNotOverrideStateAspectFromOptions(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	anyMap(state["locks"])["aspect"] = "21:9"
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "21:9", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	if !strings.Contains(feed, "【画幅风格】21:9") {
		t.Fatalf("BeefTV 不得覆盖 project_state 的画幅:\n%s", feed)
	}
	if !result.Passed {
		t.Fatalf("当传入与状态一致的 aspect 时官方机检应通过: %+v", result)
	}
}

func TestCompileFeedBuildsCompleteGenericBlockForCustomStyle(t *testing.T) {
	state, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	feed, result, err := compileFeedWithTestBaseline(state, ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9",
		StyleSource: "自定义画风", StylePromptParameters: "电影感画风，冷灰影调，写实材质",
		Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("自定义画风必须自动落成完整可投喂参数组: %v\n%s", err, feed)
	}
	if !result.Passed {
		t.Fatalf("自定义画风不得因缺少量化槽位被机检阻断: %+v\n%s", result, feed)
	}
	for _, required := range []string{
		"【STYLE｜视觉风格 · 全片锁死】",
		"【PERFORMANCE｜表演与动作 · 全片锁死】",
		"光比 [",
		"镜头 [",
	} {
		if !strings.Contains(feed, required) {
			t.Fatalf("自定义画风段8/9缺少 %q:\n%s", required, feed)
		}
	}
}

func TestCompileFeedPassesSeparatedStyleFieldsToOfficialCompiler(t *testing.T) {
	state, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	feed, result, err := compileFeedWithTestBaseline(state, ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9",
		StyleSource: "自定义画风", StylePromptParameters: "北宋末年历史战争片，低饱和青灰冷色调，做旧铠甲与城砖",
		StyleMotionParameters: "手持摄影，轻微倾斜，纪录片式微晃",
		StyleForbiddenItems:   []string{"现代元素", "鲜艳色彩"},
		Assets:                9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	if !result.Passed {
		t.Fatalf("官方机检应接受结构化画风输入: %+v\n%s", result, feed)
	}
	for _, required := range []string{
		"【STYLE｜视觉风格 · 全片锁死】",
		"【PERFORMANCE｜表演与动作 · 全片锁死】",
		"光比 [",
		"镜头 [",
		"运镜参数：手持摄影",
	} {
		if !strings.Contains(feed, required) {
			t.Fatalf("官方编译产物缺少 %q:\n%s", required, feed)
		}
	}
}

func TestNormalizeCustomStyleParametersClassifiesBanLineByContent(t *testing.T) {
	positive, bans := normalizeCustomStyleParameters("现代元素，鲜艳色彩，干净崭新的服装，棚拍感")
	if positive != "" {
		t.Fatalf("negative-only style line must not remain positive: %q", positive)
	}
	if !strings.Contains(bans, "现代元素") || !strings.Contains(bans, "鲜艳色彩") {
		t.Fatalf("negative style line classification failed: %q", bans)
	}
}

func TestNormalizeCustomStyleParametersSeparatesMotionFromVisualStyle(t *testing.T) {
	positive, bans := normalizeCustomStyleParameters(strings.Join([]string{
		"北宋末年历史战争片，电影级质感，2.39:1变形宽银幕画幅，低饱和青灰冷色调",
		"【手持类】handheld camera, subtle 3-5 degree dutch angle / 手持摄影，3-5度倾斜角",
		"现代元素，鲜艳色彩，干净崭新的服装，棚拍感",
	}, "\n"))

	if !strings.Contains(positive, "北宋末年历史战争片") {
		t.Fatalf("positive visual style was lost: %q", positive)
	}
	if strings.Contains(positive, "手持") || strings.Contains(positive, "handheld camera") {
		t.Fatalf("motion parameters must not remain in visual style: %q", positive)
	}
	if strings.Contains(positive, "现代元素") || strings.Contains(positive, "鲜艳色彩") {
		t.Fatalf("forbidden items must not remain in visual style: %q", positive)
	}
	if !strings.Contains(bans, "现代元素") || !strings.Contains(bans, "鲜艳色彩") {
		t.Fatalf("forbidden items were not preserved separately: %q", bans)
	}
}

func TestCompileFeedRejectsSingleOversizedShotWithoutCompression(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	segments := anySlice(state["segments"])
	if len(segments) == 0 {
		t.Fatal("default state has no segment")
	}
	segment := anyMap(segments[0])
	shots := anySlice(segment["shots"])
	const actionAnchor = "末句动作锚点必须逐字保留"
	var verboseBuilder strings.Builder
	for index := 1; index <= 45; index++ {
		fmt.Fprintf(&verboseBuilder, "%d号动作：林风压低重心盯住对手；右脚蹬地拧腰送肩推出直拳；陈枭架臂格挡卸力向后滑退；撞击后衣料凹陷呼吸加重；", index)
	}
	verbose := verboseBuilder.String() + actionAnchor
	for index, raw := range shots {
		shot := anyMap(raw)
		if index == 0 {
			shot["action"] = "画面主体：@林风 " + verbose
		} else {
			shot["action"] = "@林风 " + verbose
		}
		shots[index] = shot
	}
	segment["shots"] = shots
	segments[0] = segment
	state["segments"] = segments
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "R1", Mode: "M1", Aspect: "16:9",
		Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	if result.Passed {
		t.Fatalf("单个超预算镜头必须由官方机检拒绝:\n%s", feed)
	}
	if !strings.Contains(feed, actionAnchor) {
		t.Fatalf("BeefTV 不得压缩或删除官方正文: %s", feed)
	}
	if !strings.Contains(result.Output, "C4") {
		t.Fatalf("官方机检应报告字符预算错误: %+v", result)
	}
}

func TestCompileFeedDoesNotPruneDeclaredAssetsOrSplitOfficialOutput(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	characters := anyMap(state["characters"])
	for index := 0; index < 8; index++ {
		name := fmt.Sprintf("群像配角%d", index+1)
		characters[name] = map[string]any{
			"fig_no":      4 + index,
			"voice_lock":  strings.Repeat("distinct supporting voice lock ", 12),
			"char_states": map[string]any{},
		}
	}
	state["characters"] = characters

	segments := anySlice(state["segments"])
	segment := anyMap(segments[0])
	standing := []any{
		"林风位于画面左",
		"陈枭位于画面右",
	}
	for name := range characters {
		if name == "林风" || name == "陈枭" {
			continue
		}
		standing = append(standing, name+"位于后方")
	}
	segment["standing"] = standing
	segment["shots"] = []any{
		map[string]any{
			"no": 1, "start": 0, "end": 5,
			"spec":     "中近景 · 侧45度 · 极缓慢推",
			"action":   "画面主体：林风 右手攥紧战术手套向前逼近",
			"sound":    "脚步与衣料摩擦",
			"dialogue": "【林风（压着怒意·低沉）：“停手。”】",
		},
		map[string]any{
			"no": 2, "start": 5, "end": 10,
			"spec":     "中景 · 侧45度 · 跟移",
			"action":   "画面主体：陈枭 双臂架起向后滑退半步",
			"sound":    "鞋底摩擦地面",
			"dialogue": "无",
		},
	}
	segment["asset_refs"] = map[string]any{
		"scenes":     []any{"地下拳馆走廊"},
		"characters": []any{"林风", "陈枭"},
		"props":      []any{"旧怀表"},
	}
	segments[0] = segment
	state["segments"] = segments
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.0", Track: "1", Combat: "R1", Mode: "M1", Aspect: "16:9",
		Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	core := feedFieldLine(feed, "【核心人物】")
	for _, name := range []string{"林风", "陈枭"} {
		if !strings.Contains(core, "@"+name) {
			t.Fatalf("BeefTV 不得裁剪当前段声明角色 %q:\n%s", name, feed)
		}
	}
	for name := range characters {
		if name == "林风" || name == "陈枭" {
			continue
		}
		if strings.Contains(core, "@"+name) {
			t.Fatalf("编译器不得注入当前段未声明角色 %q:\n%s", name, feed)
		}
	}
	if !strings.Contains(feedFieldLine(feed, "【场景资产】"), "@PRP-旧怀表") {
		t.Fatalf("BeefTV 不得裁剪当前段声明道具:\n%s", feed)
	}
	if got := len(manjuUnitHeadingPattern.FindAllStringSubmatch(feed, -1)); got != 1 {
		t.Fatalf("BeefTV 不得拆分官方投喂单元，实际 %d 个:\n%s", got, feed)
	}
	if !result.Passed && len(result.Issues) > 0 && strings.Contains(strings.Join(result.Issues, "\n"), "asset_refs") {
		t.Fatalf("声明资产不应触发引用错误: %+v\n%s", result, feed)
	}
}

func assertManjuFeedUnitsWithinBudget(t *testing.T, feed string, budget int) {
	t.Helper()
	matches := manjuUnitHeadingPattern.FindAllStringSubmatchIndex(feed, -1)
	if len(matches) == 0 {
		t.Fatalf("feed has no unit headings:\n%s", feed)
	}
	for index, match := range matches {
		end := len(feed)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		section := feed[match[0]:end]
		codeStart := strings.Index(section, "```text")
		if codeStart < 0 {
			t.Fatalf("unit %d has no text fence:\n%s", index+1, section)
		}
		codeStart += len("```text")
		if newline := strings.IndexByte(section[codeStart:], '\n'); newline >= 0 {
			codeStart += newline + 1
		}
		codeEnd := strings.Index(section[codeStart:], "```")
		if codeEnd < 0 {
			t.Fatalf("unit %d has no closing fence:\n%s", index+1, section)
		}
		body := section[codeStart : codeStart+codeEnd]
		if got := len([]rune(body)); got > budget {
			t.Fatalf("unit %d exceeds budget %d: got %d\n%s", index+1, budget, got, body)
		}
	}
}

func TestOfficialCompileFeedScriptPreservesOversizedShotActions(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	segments := anySlice(state["segments"])
	segment := anyMap(segments[0])
	shots := anySlice(segment["shots"])
	const actionAnchor = "official-compile-feed-action-anchor"
	verbose := strings.Repeat(
		"林风压低重心盯住对手；右脚蹬地拧腰送肩推出直拳；陈枭架臂格挡卸力向后滑退；撞击后衣料凹陷呼吸加重；",
		18,
	) + actionAnchor
	for index, raw := range shots {
		shot := anyMap(raw)
		if index == 0 {
			shot["action"] = "画面主体：@林风 " + verbose
		} else {
			shot["action"] = "@林风 " + verbose
		}
		shots[index] = shot
	}
	segment["shots"] = shots
	segments[0] = segment
	state["segments"] = segments
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := writeManjuSkillTemp("compile-feed-budget-*.json", string(stateJSON))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	output, code, err := defaultManjuSkillRuntime.runPython(
		filepath.Join("short-drama-director", "scripts", "compile_feed.py"),
		[]string{"--state", path},
		60*time.Second,
	)
	if err != nil || code != 0 {
		t.Fatalf("official compile_feed.py failed: code=%d err=%v\n%s", code, err, output)
	}
	unit := betweenManjuUnitFence(t, output)
	if !strings.Contains(unit, actionAnchor) {
		t.Fatalf("official compile_feed.py 不得截断或删除镜头动作:\n%s", unit)
	}
	if got := len([]rune(unit)); got <= 2600 {
		t.Fatalf("测试动作长度不足，未覆盖超预算路径，实际 %d 字", got)
	}
}

func TestCompileFeedDoesNotSanitizeForbiddenStyleTerms(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	anyMap(state["world_anchor"])["forbidden_elements"] = []any{"现代元素", "鲜艳色彩"}
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9",
		StyleSource: "自定义画风", StylePromptParameters: "电影感写实，拒绝现代元素和鲜艳色彩",
		Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	if result.Passed {
		t.Fatalf("官方机检应拦截包含禁用词的原始稿: %+v\n%s", result, feed)
	}
	if !strings.Contains(feed, "现代元素") || !strings.Contains(feed, "鲜艳色彩") {
		t.Fatalf("BeefTV 不得静默清洗官方编译正文:\n%s", feed)
	}
}

func TestCompileFeedDoesNotNormalizeLegacyStyleCard(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	const legacyStyle = "冷青灰写实质感，2.39:1变形宽银幕画幅\n【手持类】handheld camera, subtle jitter\n现代元素，鲜艳色彩，塑料质感铠甲"
	anyMap(state["locks"])["aspect"] = "16:9"
	anyMap(state["locks"])["style_card"] = legacyStyle
	anyMap(state["world_anchor"])["forbidden_elements"] = []any{"现代元素", "鲜艳色彩"}
	receipt := anyMap(state["receipt"])
	receipt["fingerprints"] = []any{fmt.Sprintf("【P0 可投喂视觉参数：%s】", legacyStyle)}
	state["receipt"] = receipt
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, result, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9",
		StyleSource: "项目画风", StylePromptParameters: legacyStyle,
		Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	if result.Passed {
		t.Fatalf("旧版混合画风卡应被官方机检拒绝，而不是被 BeefTV 改写: %+v\n%s", result, feed)
	}
	for _, preserved := range []string{"2.39:1", "现代元素", "鲜艳色彩"} {
		if !strings.Contains(feed, preserved) {
			t.Fatalf("BeefTV 不得清洗或归一化旧画风字段 %q:\n%s", preserved, feed)
		}
	}
}

func TestCompileFeedPreservesP3HandoffPlaceholder(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	anyMap(state["locks"])["aspect"] = "16:9"
	segments := anySlice(state["segments"])
	if len(segments) == 0 {
		t.Fatal("默认 project_state 缺少 segments")
	}
	segment := anyMap(segments[0])
	shots := anySlice(segment["shots"])
	if len(shots) == 0 {
		t.Fatal("默认 project_state 缺少 shots")
	}
	lastShotIndex := len(shots) - 1
	shot := anyMap(shots[lastShotIndex])
	shot["action"] = "老黄含食抬头；徐风年摇头时肩背真实发力；三人仍围桌"
	shot["dialogue"] = "【未交接】承接点：徐风年“盘缠不多”；不编造。"
	shots[lastShotIndex] = shot
	segment["shots"] = shots
	segments[0] = segment
	state["segments"] = segments
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	feed, _, err := compileFeedWithTestBaseline(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "auto", Mode: "M1", Aspect: "16:9",
		StyleSource: "袁和平", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("官方编译失败: %v\n%s", err, feed)
	}
	if !strings.Contains(feed, "未交接") {
		t.Fatalf("BeefTV 不得清洗或替换官方输出中的 P3 占位:\n%s", feed)
	}
}

func TestRuntimeStageInstructionsDoNotDuplicateOfficialBusinessRules(t *testing.T) {
	for _, stage := range []string{ProductionStageStoryboard, ProductionStageFeed, ProductionStageQC} {
		instruction := productionStageInstruction(stage, AssetModeM1Value)
		for _, forbidden := range []string{
			"C23f",
			"默认 C 导演轨纯净版",
			"先执行 7 步压缩",
			"Seedance 2.5 ≤2600 字符",
			"五官 10 项锁定",
			"场景九宫格",
		} {
			if strings.Contains(instruction, forbidden) {
				t.Fatalf("%s runtime instruction duplicated official business rule %q:\n%s", stage, forbidden, instruction)
			}
		}
	}
	stateInstruction := projectStatePatchInstruction("P3")
	for _, required := range []string{
		"MANJU_PROJECT_STATE_PATCH_JSON",
		`"segments"`,
		`"start"`,
		`"end"`,
		"不得写入 locks 或 receipt",
	} {
		if !strings.Contains(stateInstruction, required) {
			t.Fatalf("P3 状态回写合同缺少 %q:\n%s", required, stateInstruction)
		}
	}
	for _, forbidden := range []string{"默认 C 导演轨纯净版", "先执行 7 步压缩", "Seedance 2.5 ≤2600 字符"} {
		if strings.Contains(stateInstruction, forbidden) {
			t.Fatalf("project_state transport contract must not repeat business rule %q:\n%s", forbidden, stateInstruction)
		}
	}
}

func TestAssetStageInstructionContainsNoTransportOrBusinessRules(t *testing.T) {
	assetInstruction := assetStageInstruction(AssetModeM1Value, AssetStageCompile)
	for _, forbidden := range []string{
		"MANJU_CANVAS_TASKS_JSON",
		"五官 10 项锁定",
		"场景九宫格",
		"先出母版",
		"禁止手持",
		"全图共 9 格",
	} {
		if strings.Contains(assetInstruction, forbidden) {
			t.Fatalf("asset runtime instruction must not contain %q:\n%s", forbidden, assetInstruction)
		}
	}
}

func TestAppendManjuSkillContextDeclaresStrictRuleBoundary(t *testing.T) {
	var out strings.Builder
	appendManjuSkillContext(&out, "官方规则原文")
	for _, required := range []string{
		"V7.4.26 官方总控强制分发原文",
		"规则原文是唯一输出依据",
		"不得摘要",
		"不得改写",
		"不得重排",
		"不得压缩",
		"不得合并",
		"不得把规则文档中的操作说明、开关状态、流程备注或检查说明复制进最终产物",
		"固定字段名必须逐字保留",
		"不合格时只允许回本阶段重新生成",
	} {
		if !strings.Contains(out.String(), required) {
			t.Fatalf("strict rule boundary is missing %q:\n%s", required, out.String())
		}
	}
	if strings.Contains(out.String(), "slice_context.py") {
		t.Fatal("stage prompt must not present slice_context as the rule authority")
	}
}

func TestV7426ProjectStatePatchContract(t *testing.T) {
	content := "# 资产\n<!-- MANJU_PROJECT_STATE_PATCH_JSON\n" +
		`{"characters":{"林风":{"id":"CHR-01","fig_no":1}},"scenes":{"走廊":{"id":"SCN-01","fig_no":2}},"props":{}}` +
		"\n-->"
	if err := validateRequiredManjuProjectStatePatch("asset:asset_compile", content); err != nil {
		t.Fatal(err)
	}
	message := manjuProjectStateFailure("P2 资产交付", ErrInvalidProjectState)
	if !strings.Contains(message, "项目状态台账回写失败") || !strings.Contains(message, "MANJU_PROJECT_STATE_PATCH_JSON") {
		t.Fatalf("状态台账失败信息不明确: %s", message)
	}
	if _, _, err := extractManjuProjectStatePatch("<!-- MANJU_PROJECT_STATE_PATCH_JSON\n{}"); err == nil {
		t.Fatal("缺少结束标记的状态回写块必须被拒绝")
	}
}

func TestCompileFeedInjectsStyleBlocks(t *testing.T) {
	feed, err := defaultManjuSkillRuntime.injectStyleLockBlocks("# P4 投喂封装\n", ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Mode: "M1", Aspect: "16:9",
		StyleSource: "路阳", StylePromptParameters: "冷青灰配烛火暖橙 · 低机位贴身跟拍",
	})
	if err != nil {
		t.Fatalf("画风锁与平台安全后处理失败: %v", err)
	}
	for _, required := range []string{
		"【STYLE｜视觉风格 · 全片锁死】",
		"【PERFORMANCE｜表演与动作 · 全片锁死】",
		"光比 [1:5, 1:9]",
		"镜头 [28mm, 85mm]",
	} {
		if !strings.Contains(feed, required) {
			t.Fatalf("画风锁段8/段9缺少 %q:\n%s", required, feed)
		}
	}
	if strings.Contains(feed, "路阳") {
		t.Fatalf("交付层不得出现画风来源人名:\n%s", feed)
	}
}

func TestEmbeddedV7426RuleLibraryDispatchesFromOfficialMaster(t *testing.T) {
	ctx, err := defaultManjuSkillRuntime.StageContext(ManjuSkillStageOptions{
		Stage: "P2", Track: "1", Combat: "none", Model: "2.0", Mode: "M1",
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := defaultManjuSkillRuntime.EmbeddedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 170 {
		t.Fatalf("V7.4.26 原规则包文件数应为 170，实际 %d", len(files))
	}
	hash, err := defaultManjuSkillRuntime.EmbeddedRuleHash()
	if err != nil {
		t.Fatal(err)
	}
	if hash != manjuSkillContentHash {
		t.Fatalf("嵌入规则包内容哈希不匹配: %s", hash)
	}
	for _, required := range []string{
		"===== V7.4.26 官方总控原文 =====",
		"===== 官方总控强制读取原文 =====",
		"sd-asset-engine/SKILL.md",
		"character-portrait-templates.md",
		"scene-grid-template.md",
		"prop-template.md",
	} {
		if !strings.Contains(ctx, required) {
			t.Fatalf("P2 官方总控分发缺少 %q", required)
		}
	}
	if strings.Contains(ctx, "阶段完整章节原文分页") {
		t.Fatal("P2 still used slice_context instead of the official master dispatch")
	}
}

func TestV7426OfficialScriptInventory(t *testing.T) {
	files, err := defaultManjuSkillRuntime.EmbeddedFiles()
	if err != nil {
		t.Fatal(err)
	}
	scripts := make(map[string]bool, len(files))
	for _, path := range files {
		scripts[path] = true
	}
	for _, required := range []string{
		"short-drama-director/scripts/check_asset_blocks.py",
		"sd-storyboard-engine/scripts/check_p45.py",
		"short-drama-director/scripts/audit_contract.py",
		"sd-feed-engine/scripts/script_feed_diff.py",
		"sd-qc-engine/scripts/validate_seedance_segment_map.py",
		"short-drama-director/scripts/test_project_bundle.py",
		"short-drama-director/scripts/test_slice_stability.py",
		"short-drama-director/scripts/test_validator_stability.py",
	} {
		if !scripts[required] {
			t.Fatalf("V7.4.26 官方脚本清单缺少 %s", required)
		}
	}
}

func TestManjuSkillSourceManifestV7426(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("skilldata", "manifests", "v7.4.26-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version         string            `json:"version"`
		UpstreamCommit  string            `json:"upstreamCommit"`
		PatchID         string            `json:"patchId"`
		PatchSHA256     string            `json:"patchSHA256"`
		BaseTreeHash    string            `json:"baseTreeHash"`
		PatchedTreeHash string            `json:"patchedTreeHash"`
		Commit          string            `json:"commit"`
		FileCount       int               `json:"fileCount"`
		TreeHash        string            `json:"treeHash"`
		Files           map[string]string `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Commit != "6f16955c57053d77ea0ffbdbbf3a7985a3e0080a" {
		t.Fatalf("上游提交未固定: %s", manifest.Commit)
	}
	if manifest.Version != "V7.4.26" ||
		manifest.UpstreamCommit != manifest.Commit ||
		manifest.PatchID != "V7.4.26+beef-segment-assets.1" ||
		len(manifest.PatchSHA256) != 64 ||
		manifest.BaseTreeHash != "f975dc22f36e1972be137ff9c04615522958a7283afd07bda50e921f5525940f" ||
		manifest.PatchedTreeHash != manifest.TreeHash {
		t.Fatalf("规则来源必须区分原始上游与本地补丁: %+v", manifest)
	}
	if manifest.FileCount != 170 || len(manifest.Files) != 170 {
		t.Fatalf("V7.4.26 来源清单必须恰好 170 个文件，count=%d files=%d", manifest.FileCount, len(manifest.Files))
	}
	paths := make([]string, 0, len(manifest.Files))
	for path := range manifest.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var lines strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&lines, "%s:%s\n", path, manifest.Files[path])
	}
	sum := sha256.Sum256([]byte(lines.String()))
	if got := fmt.Sprintf("%x", sum); got != manifest.TreeHash || got != manjuSkillContentHash {
		t.Fatalf("来源清单树哈希不一致: manifest=%s computed=%s runtime=%s", manifest.TreeHash, got, manjuSkillContentHash)
	}
}

func TestEmbeddedCompileFeedMatchesAuthoritativeV7426(t *testing.T) {
	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "short-drama-director", "scripts", "compile_feed.py"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	const authoritativeHash = "322979c2321cd4d7e883f14b3328f277a6a902c8454d1961d7dfd16d8abb0338"
	if got := fmt.Sprintf("%x", sum); got != authoritativeHash {
		t.Fatalf("内嵌 compile_feed.py 未与权威 V7.4.26 对齐: got %s want %s", got, authoritativeHash)
	}
}

func TestOfficialV7426ScriptsSelfTest(t *testing.T) {
	if err := defaultManjuSkillRuntime.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialP4GateRejectsNonFeedOutput(t *testing.T) {
	result, err := defaultManjuSkillRuntime.ValidateP4("# P4 投喂封装\n这不是七段式投喂稿。", ManjuSkillValidationOptions{
		Model: "seedance-2.0", Track: "1", Combat: "auto", Mode: "M1", Aspect: "9:16", Assets: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || len(result.Issues) == 0 {
		t.Fatalf("官方 P4 校验必须拒绝非投喂稿输出: %+v", result)
	}
}

func TestOfficialContractAuditRejectsSketchyArchive(t *testing.T) {
	result, err := defaultManjuSkillRuntime.AuditContract("# P3 分镜\n只有一句话。", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || len(result.Issues) == 0 {
		t.Fatalf("官方全流程契约审计必须拒绝简笔归档: %+v", result)
	}
}
