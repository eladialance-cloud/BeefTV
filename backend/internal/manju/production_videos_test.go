package manju

import (
	"strings"
	"testing"
)

func TestParseProductionVideoSegmentsAcceptsOfficialCompileFeedFormat(t *testing.T) {
	content := `> 素材形态：M1

## 【投喂单元 1/4 · 段1】
` + "```text" + `
【STYLE｜视觉风格 · 全片锁死】
影调区间：光比 [1:5, 1:9]
【PERFORMANCE｜表演与动作 · 全片锁死】
帧率区间：[24fps, 60fps]
【画幅风格】9:16 横屏
【时间轴分镜】
00:00-00:04 [镜头1] 中景。林风向前。声音：脚步。台词：无。
00:04-00:08 [镜头2] 近景。林风停步。声音：风。台词：无。
` + "```" + `

## 【投喂单元 2/4 · 段2】
` + "```text" + `
【STYLE｜视觉风格 · 全片锁死】
影调区间：光比 [1:5, 1:9]
【PERFORMANCE｜表演与动作 · 全片锁死】
帧率区间：[24fps, 60fps]
【画幅风格】9:16 横屏
【时间轴分镜】
00:00-00:03 [镜头1] 全景。城门起火。声音：火。台词：无。
00:03-00:06 [镜头2] 中景。众人后退。声音：脚步。台词：无。
` + "```" + `
`

	segments, err := parseProductionVideoSegments(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 2 {
		t.Fatalf("官方 compile_feed 四段式标题应解析为 2 个测试段: %+v", segments)
	}
	if segments[0].Code != "段1" || segments[0].DurationSeconds != 8 {
		t.Fatalf("第一段解析错误: %+v", segments[0])
	}
	if segments[1].Code != "段2" || segments[1].DurationSeconds != 6 {
		t.Fatalf("第二段解析错误: %+v", segments[1])
	}
	if !strings.Contains(segments[0].Prompt, "【时间轴分镜】") || strings.Contains(segments[0].Prompt, "```") {
		t.Fatalf("视频提示词必须取代码框正文而不是整块 Markdown: %s", segments[0].Prompt)
	}
	if segments[0].GlobalStyle != "" {
		t.Fatalf("逐单元画风不得再抽成全局前缀: %+v", segments[0])
	}
	for _, required := range []string{
		"【STYLE｜视觉风格 · 全片锁死】",
		"【PERFORMANCE｜表演与动作 · 全片锁死】",
	} {
		if !strings.Contains(segments[0].Prompt, required) {
			t.Fatalf("逐单元提示词缺少契约画风块 %q: %s", required, segments[0].Prompt)
		}
	}
	for _, forbidden := range []string{
		"【本段投喂】",
		"你是 VisiDeep AI 漫剧工作室的 P4 视频生成执行器",
	} {
		if strings.Contains(segments[0].Prompt, forbidden) {
			t.Fatalf("画布视频提示词必须原样等于 P4 分段正文，不得拼接 %q: %s", forbidden, segments[0].Prompt)
		}
	}
	executionPrompt := composeProductionVideoPrompt(segments[0].GlobalStyle, segments[0].Prompt)
	if executionPrompt != segments[0].Prompt {
		t.Fatalf("逐单元画风不得在生成时重复拼接:\n%s", executionPrompt)
	}
	for _, required := range []string{"【STYLE｜视觉风格 · 全片锁死】", "【画幅风格】9:16 横屏"} {
		if !strings.Contains(executionPrompt, required) {
			t.Fatalf("执行提示词缺少 %q: %s", required, executionPrompt)
		}
	}
}

func TestParseProductionVideoSegmentsKeepsLegacyDurationFormat(t *testing.T) {
	content := `## 段1｜旧版
【时长】5秒
这是旧版投喂正文。`

	segments, err := parseProductionVideoSegments(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || segments[0].Code != "段1" || segments[0].DurationSeconds != 5 {
		t.Fatalf("旧版段标题与时长格式必须继续兼容: %+v", segments)
	}
}
