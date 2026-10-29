package manju

import (
	"strings"
	"testing"
)

func TestGenericStyleBlockExpandsToActualShotLensRange(t *testing.T) {
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】16:9 横屏，古代战场（aspect_ratio=16:9）
【场景资产】@城门（参考图1）
【核心人物】@梁崇（参考图2）
【站位声明】梁崇位于画面左侧
【时间轴分镜】
00:00-00:05 [镜头1] 全景 · 24mm · 平视 · 缓慢后拉。梁崇望向城外。声音：风雪。台词：无。
00:05-00:10 [镜头2] 特写 · 100mm · 平视 · 固定。梁崇眼眶收紧。声音：呼吸。台词：无。
【音效】风雪
【强制禁止项】画面：禁字幕、禁水印；音频：禁背景音乐、禁旁白。
` + "```" + `
`

	injected, err := defaultManjuSkillRuntime.injectStyleLockBlocks(feed, ManjuSkillValidationOptions{
		StyleSource:           "自定义画风",
		StylePromptParameters: "冷灰低饱和，硬光侧照，真实材质",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(injected, "光学区间：镜头 [24mm, 100mm]") {
		t.Fatalf("generic style lens range must include actual 100mm shot:\n%s", injected)
	}
}

func TestInjectStyleLockBlocksInsertsBlocksInsideEveryFeedUnit(t *testing.T) {
	feed := `> 素材形态：M1

## 【投喂单元 1/2 · 段1】
` + "```text" + `
【画幅风格】16:9 横屏（aspect_ratio=16:9）
【场景资产】@//scn/ - 旧宅（图1）
【核心人物】@//chr/ - 林澈（图2）
【站位声明】林澈在画面左侧
【时间轴分镜】
00:00-00:05 [镜头1] 中景 · 35mm · 平视 · 固定。林澈停步。声音：风声。台词：无。
【音效】风声
【强制禁止项】画面：禁字幕。
` + "```" + `

## 【投喂单元 2/2 · 段2】
` + "```text" + `
【画幅风格】16:9 横屏（aspect_ratio=16:9）
【场景资产】@//scn/ - 城头（图3）
【核心人物】@//chr/ - 林澈（图2）
【站位声明】林澈在垛口后
【时间轴分镜】
00:00-00:05 [镜头1] 全景 · 24mm · 仰拍 · 固定。林澈望向城外。声音：号角。台词：无。
【音效】号角
【强制禁止项】画面：禁字幕。
` + "```" + `
`
	injected, err := defaultManjuSkillRuntime.injectStyleLockBlocks(feed, ManjuSkillValidationOptions{
		StyleSource:           "自定义画风",
		StylePromptParameters: "冷灰低饱和，硬光侧照，真实材质",
	})
	if err != nil {
		t.Fatal(err)
	}
	matches := manjuUnitHeadingPattern.FindAllStringIndex(injected, -1)
	if len(matches) != 2 {
		t.Fatalf("expected 2 feed units, got %d:\n%s", len(matches), injected)
	}
	if strings.Contains(injected[:matches[0][0]], "【STYLE｜") {
		t.Fatalf("style block must not remain global outside feed units:\n%s", injected)
	}
	for index, match := range matches {
		end := len(injected)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		section := injected[match[0]:end]
		for _, marker := range []string{
			"【STYLE｜视觉风格 · 全片锁死】",
			"【PERFORMANCE｜表演与动作 · 全片锁死】",
		} {
			if !strings.Contains(section, marker) {
				t.Fatalf("feed unit %d is missing %s:\n%s", index+1, marker, section)
			}
		}
		codeStart := strings.Index(section, "```text")
		codeEnd := strings.LastIndex(section, "```")
		stylePos := strings.Index(section, "【STYLE｜")
		if codeStart < 0 || codeEnd <= codeStart || stylePos < codeStart || stylePos > codeEnd {
			t.Fatalf("style block must be inside the unit text fence:\n%s", section)
		}
	}
}

func TestProductionFeedGlobalStyleStaysEmptyForPerUnitContract(t *testing.T) {
	content := `## 【投喂单元 1/2 · 段1】
` + "```text" + `
【STYLE｜视觉风格 · 全片锁死】
影调区间：光比 [1:5, 1:9]
【PERFORMANCE｜表演与动作 · 全片锁死】
帧率区间：[24fps, 60fps]
【画幅风格】16:9
` + "```" + `
## 【投喂单元 2/2 · 段2】
` + "```text" + `
【STYLE｜视觉风格 · 全片锁死】
影调区间：光比 [1:5, 1:9]
` + "```" + `
`
	global := productionFeedGlobalStyle(content)
	if global != "" {
		t.Fatalf("per-unit style must not be extracted into a global prefix: %s", global)
	}
}
