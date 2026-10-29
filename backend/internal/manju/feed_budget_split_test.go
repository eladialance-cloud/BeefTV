package manju

import (
	"strings"
	"testing"
)

func TestSplitCompiledManjuFeedByBudgetPreservesEveryShot(t *testing.T) {
	feed := oversizedCompiledManjuFeed()

	split, err := splitCompiledManjuFeedByBudget(feed, "seedance-2.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"动作锚点甲", "动作锚点乙", "动作锚点丙", "动作锚点丁",
		"台词锚点甲", "台词锚点丁", "【音效】", "【强制禁止项】",
	} {
		if !strings.Contains(split, marker) {
			t.Fatalf("split feed dropped %q:\n%s", marker, split)
		}
	}
	if got := len(manjuUnitHeadingPattern.FindAllStringSubmatch(split, -1)); got < 2 {
		t.Fatalf("feed should be split into multiple units, got %d:\n%s", got, split)
	}
	assertManjuFeedUnitsWithinBudget(t, split, 1900)

	matches := manjuUnitHeadingPattern.FindAllStringSubmatchIndex(split, -1)
	previousLastScale := ""
	for index, match := range matches[1:] {
		previousSection := split[matches[index][0]:matches[index+1][0]]
		previousLastScale = compiledShotScale(lastCompiledShotLine(previousSection))
		end := len(split)
		if index+2 < len(matches) {
			end = matches[index+2][0]
		}
		section := split[match[0]:end]
		if !strings.Contains(section, "00:00-") || !strings.Contains(section, "[镜头1]") {
			t.Fatalf("split unit must reset timeline and shot numbering:\n%s", section)
		}
		if !strings.Contains(section, "末 1 秒全体人物保持姿态与站位、动作完全静止、画面定格") {
			t.Fatalf("every split unit must add an end-freeze seam frame:\n%s", section)
		}
		if firstScale := compiledShotScale(firstCompiledShotLine(section)); firstScale != previousLastScale {
			t.Fatalf("split seam must keep 0-level scale: previous=%s current=%s\n%s", previousLastScale, firstScale, section)
		}
	}
}

func TestSplitCompiledManjuFeedByBudgetRejectsOversizedSingleShot(t *testing.T) {
	feed := strings.Replace(
		oversizedCompiledManjuFeed(),
		"动作锚点甲",
		strings.Repeat("单镜超长动作锚点甲；", 260),
		1,
	)

	_, err := splitCompiledManjuFeedByBudget(feed, "seedance-2.0")
	if err == nil ||
		!strings.Contains(err.Error(), "单个镜头") ||
		!strings.Contains(err.Error(), "镜头1") ||
		!strings.Contains(err.Error(), "拆成多个连续 shot") {
		t.Fatalf("oversized single shot must fail with an actionable error, got %v", err)
	}
}

func oversizedCompiledManjuFeed() string {
	shot := func(index int, marker string) string {
		times := []string{"00:00-00:05", "00:05-00:10", "00:10-00:15", "00:15-00:20"}[index-1]
		specs := []string{"中景 · 侧45度 · 固定", "中景 · 侧45度 · 固定", "全景 · 侧45度 · 固定", "全景 · 侧45度 · 固定"}
		return times + " [镜头" + string(rune('0'+index)) + "] " + specs[index-1] + "。" +
			strings.Repeat("人物继续完成当前动作与受力反应；", 35) + marker +
			"。声音：环境声与动作声。台词：【角色（压着怒意）：\"台词锚点" +
			[]string{"甲", "乙", "丙", "丁"}[index-1] + "\"】。"
	}
	return `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】9:16 竖屏，古代战场，电影级写实（aspect_ratio=9:16）
【场景资产】@城头（参考图1）
【核心人物】@梁崇（参考图2，声线锁：低沉男声）
【站位声明】梁崇位于画面左侧，面向城外
【时间轴分镜】
` + shot(1, "动作锚点甲") + `
` + shot(2, "动作锚点乙") + `
` + shot(3, "动作锚点丙") + `
` + shot(4, "动作锚点丁") + `
【音效】风声、甲片碰撞、呼吸声
【强制禁止项】画面：禁字幕、禁水印；音频：禁背景音乐、禁旁白。
` + "```" + `
`
}

func firstCompiledShotLine(section string) string {
	for _, line := range strings.Split(section, "\n") {
		if strings.Contains(line, "[镜头") {
			return line
		}
	}
	return ""
}

func lastCompiledShotLine(section string) string {
	last := ""
	for _, line := range strings.Split(section, "\n") {
		if strings.Contains(line, "[镜头") {
			last = line
		}
	}
	return last
}
