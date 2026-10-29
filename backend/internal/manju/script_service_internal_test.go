package manju

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestScriptStagePromptTreatsLockedSystemStateAsAuthoritative(t *testing.T) {
	prompt := buildScriptStagePrompt(
		"测试项目",
		model.ManjuScriptRun{Branch: ScriptBranchAdaptation, Mode: ScriptModeStepwise},
		model.ManjuScriptStage{Title: "全剧开发圣经", Key: ScriptStageSeriesBible},
		nil,
		[]model.ManjuScriptArtifact{{
			Kind:    "IP_DEVELOPMENT.md",
			Version: 1,
			Content: "# IP 与角色开发\n> 状态：工作稿；未审核、未锁定\n> 核心命题：测试事实。",
		}},
		nil,
		"",
		"",
	)
	for _, required := range []string{
		"系统门禁状态具有最终权威",
		"工作稿/未审核/未锁定只是生成时自述",
		"不得据此输出门禁阻断单",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("P1 下游提示词缺少系统状态权威说明 %q:\n%s", required, prompt)
		}
	}
}

func TestAssetStagePromptTreatsLockedUpstreamStateAsAuthoritative(t *testing.T) {
	prompt := buildAssetStagePrompt(
		"测试项目",
		model.ManjuAssetRun{ID: "asset-run", AssetMode: AssetModeM1Value},
		model.ManjuAssetStage{Title: "实体提取", Key: AssetStageExtraction},
		model.ManjuScriptRun{Branch: ScriptBranchAdaptation, Mode: ScriptModeStepwise},
		nil,
		[]model.ManjuScriptArtifact{{
			Kind:    "IP_DEVELOPMENT.md",
			Version: 1,
			Content: "# IP 与角色开发\n> 状态：工作稿；未审核、未锁定\n> 核心命题：测试事实。",
		}},
		nil,
		nil,
		"",
		"",
	)
	for _, required := range []string{
		"系统门禁状态具有最终权威",
		"工作稿/未审核/未锁定只是生成时自述",
		"不得据此输出门禁阻断单",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("P2 提示词缺少锁定状态权威说明 %q:\n%s", required, prompt)
		}
	}
}
