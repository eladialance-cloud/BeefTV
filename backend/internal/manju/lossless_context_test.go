package manju

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestStageContextIncludesCompleteRuleFiles(t *testing.T) {
	contextText, err := defaultManjuSkillRuntime.StageContext(ManjuSkillStageOptions{
		Stage: "P4", Track: "1", Combat: "none", Model: "2.5", Mode: "M1", Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "sd-feed-engine", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	tail := lastNonEmptyTextLine(string(data))
	if tail == "" || !strings.Contains(contextText, tail) {
		t.Fatalf("stage context omitted the complete rule file tail %q", tail)
	}
	if strings.Contains(contextText, "[已截断]") {
		t.Fatal("stage context must not include truncated rule text")
	}
}

func TestP2StageContextUsesOfficialMasterRulesOnly(t *testing.T) {
	contextText, err := defaultManjuSkillRuntime.StageContext(ManjuSkillStageOptions{
		Stage: "P2", Track: "1", Combat: "none", Model: "2.5", Mode: "M1", Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"sd-asset-engine/SKILL.md",
		"sd-asset-engine/references/scene-grid-template.md",
		"sd-asset-engine/references/character-portrait-templates.md",
		"sd-asset-engine/references/prop-template.md",
	} {
		if !strings.Contains(contextText, required) {
			t.Fatalf("P2 official master rule missing %q", required)
		}
	}
	if strings.Contains(contextText, "V7.4.26 P2 资产专项补充原文") {
		t.Fatal("P2 included a BeefTV-written supplemental rule block")
	}
}

func TestBuildPrevisStoryboardPromptDoesNotTruncateAnyInput(t *testing.T) {
	rules := strings.Repeat("规则完整内容；", 4000) + "RULES_TAIL_MARKER"
	feed := strings.Repeat("投喂完整内容；", 6000) + "FEED_TAIL_MARKER"
	stateJSON := `{"locks":{},"segments":[]}` + strings.Repeat(" ", 50000) + "STATE_TAIL_MARKER"
	prompt := buildPrevisStoryboardPrompt(
		"测试项目",
		model.ManjuProductionRun{ID: "run-full"},
		feed,
		stateJSON,
		1,
		rules,
	)
	for _, marker := range []string{"RULES_TAIL_MARKER", "FEED_TAIL_MARKER", "STATE_TAIL_MARKER"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("P4.5 prompt truncated %s", marker)
		}
	}
}

func TestPrevisPromptIncludesExactPerSegmentPanelSkeleton(t *testing.T) {
	stateJSON := `{"segments":[
		{"seg_id":"段1","storyboard_panels":["起始","过程","反应","出口"]},
		{"seg_id":"段2","storyboard_panels":["起始","过程","接触","顶点","出口"]}
	]}`
	prompt := buildPrevisStoryboardPrompt("测试项目", model.ManjuProductionRun{ID: "run-p45"}, "P4", stateJSON, 2, "规则")
	for _, required := range []string{
		"## 段 1",
		"### 故事板提示词（4 格）",
		"第 1 格：",
		"第 4 格：",
		"## 段 2",
		"### 故事板提示词（5 格）",
		"第 5 格：",
		"不得把多格合并成一个“第 1 格”",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("P4.5 skeleton missing %q:\n%s", required, prompt)
		}
	}
}

func TestBuildProductionStagePromptDoesNotTruncateLockedArtifacts(t *testing.T) {
	assetTail := "P2_ASSET_TAIL_MARKER"
	productionTail := "P3_PRODUCTION_TAIL_MARKER"
	prompt := buildProductionStagePrompt(
		"测试项目",
		model.ManjuProductionRun{ID: "run-full", AssetRunID: "asset-run"},
		model.ManjuProductionStage{Key: ProductionStageStoryboard, Title: "P3 分镜与故事板"},
		model.ManjuAssetRun{ID: "asset-run", AssetMode: AssetModeM1Value},
		nil,
		nil,
		[]model.ManjuAssetArtifact{{
			Kind: "ASSET_COMPILE.md", Version: 1,
			Content: strings.Repeat("P2完整工件；", 5000) + assetTail,
		}},
		[]model.ManjuProductionArtifact{{
			Kind: "UPSTREAM.md", Version: 1,
			Content: strings.Repeat("P3完整工件；", 5000) + productionTail,
		}},
		nil,
		nil,
		"",
		"",
		"",
	)
	for _, marker := range []string{assetTail, productionTail} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("production prompt truncated %s", marker)
		}
	}
}

func TestCompileFeedDoesNotCompressRuleGeneratedContent(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(rawState), &state); err != nil {
		t.Fatal(err)
	}
	shots := make([]any, 0, 8)
	for index := 0; index < 8; index++ {
		shots = append(shots, map[string]any{
			"no":    index + 1,
			"start": index * 2,
			"end":   (index + 1) * 2,
			"spec":  "中景 · 24mm · 平视 · 固定",
			"action": "画面主体：@林风 确保画面连贯；" +
				strings.Repeat("林风向前推进，衣料随动作摆动；", 15) +
				fmt.Sprintf("无压缩镜头锚点%d", index+1),
			"sound":    "脚步与衣料摩擦",
			"dialogue": "无",
		})
	}
	state["segments"] = []any{map[string]any{
		"seg_id": "段1", "scene": "地下拳馆走廊", "segment_type": "对话戏",
		"asset_refs": map[string]any{
			"scenes": []any{"地下拳馆走廊"}, "characters": []any{"林风"}, "props": []any{},
		},
		"standing": []any{"@林风（画面左 · 走廊中段 · 朝右 · 立 · 身后是混凝土墙面）"},
		"shots":    shots, "storyboard_panels": []any{"起始", "出口"}, "sfx_summary": "脚步与衣料摩擦",
	}}
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	feed, _, err := defaultManjuSkillRuntime.CompileFeed(string(stateJSON), ManjuSkillValidationOptions{
		Model: "2.0", Track: "1", Combat: "none", Mode: "M1", Aspect: "9:16", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("lossless compile failed: %v\n%s", err, feed)
	}
	if !strings.Contains(feed, "确保画面连贯") {
		t.Fatalf("P4 compressed a rule-generated low-value clause:\n%s", feed)
	}
	for index := 1; index <= 8; index++ {
		marker := fmt.Sprintf("无压缩镜头锚点%d", index)
		if !strings.Contains(feed, marker) {
			t.Fatalf("P4 compression or splitting lost %s", marker)
		}
	}
}

func TestCompileFeedReturnsOfficialCompilerOutputUnchanged(t *testing.T) {
	rawState, err := defaultManjuSkillRuntime.DefaultProjectStateTemplate()
	if err != nil {
		t.Fatal(err)
	}
	normalized := rawState
	statePath, cleanup, err := writeManjuSkillTemp("official-feed-*.json", normalized)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	official, code, err := defaultManjuSkillRuntime.runPython(
		filepath.Join("short-drama-director", "scripts", "compile_feed.py"),
		[]string{"--state", statePath},
		120*time.Second,
	)
	if err != nil || code != 0 {
		t.Fatalf("official compiler failed: code=%d err=%v\n%s", code, err, official)
	}
	actual, _, err := defaultManjuSkillRuntime.CompileFeed(normalized, ManjuSkillValidationOptions{
		Model: "2.5", Track: "1", Combat: "none", Mode: "M1", Aspect: "16:9", Assets: 9, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("CompileFeed failed: %v", err)
	}
	if strings.TrimSpace(actual) != strings.TrimSpace(official) {
		t.Fatalf(
			"BeefTV modified official compile_feed output\n--- official ---\n%s\n--- actual ---\n%s",
			official,
			actual,
		)
	}
}

func lastNonEmptyTextLine(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return line
		}
	}
	return ""
}
