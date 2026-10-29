package manju

import (
	"strings"
	"testing"
)

func TestCompressCompiledManjuFeedOnlyRemovesSafeText(t *testing.T) {
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】9:16 竖屏，古代战场（aspect_ratio=9:16）
【场景资产】@城头（参考图1）；时段光影：冷白月光·硬光·暗部冷灰
【核心人物】@梁崇（参考图2，声线锁：低沉男声）
【站位声明】梁崇位于画面左侧，面向城外
【时间轴分镜】
00:00-00:05 [镜头1] 中景 · 35mm · 侧45度 · 摄影机极缓慢地向前推进。接下一段；梁崇推门；梁崇推门；非常非常缓慢地向前推进；承接上一段；城砖碎屑从墙头落下；末1秒全体人物定格。声音：木门吱呀。台词：【梁崇（压着怒意）："接下一段，别回头。"】。
【音效】木门吱呀
【强制禁止项】画面：禁字幕、禁水印；音频：禁背景音乐、禁旁白。
` + "```" + `
`
	feed = strings.Replace(feed, "城砖碎屑从墙头落下；", strings.Repeat("城砖碎屑从墙头落下；", 250), 1)

	compressed, stats := compressCompiledManjuFeed(feed, "seedance-2.0")
	for _, removed := range []string{"接下一段；", "承接上一段；", "梁崇推门；梁崇推门", "摄影机极缓慢地向前推进", "非常非常缓慢地向前推进"} {
		if strings.Contains(compressed, removed) {
			t.Fatalf("safe compression kept %q:\n%s", removed, compressed)
		}
	}
	for _, protected := range []string{
		"【场景资产】@城头（参考图1）；时段光影：冷白月光·硬光·暗部冷灰",
		"【核心人物】@梁崇（参考图2，声线锁：低沉男声）",
		"【站位声明】梁崇位于画面左侧，面向城外",
		"梁崇推门",
		"极缓推",
		"缓慢推进",
		"城砖碎屑从墙头落下",
		"声音：木门吱呀",
		`台词：【梁崇（压着怒意）："接下一段，别回头。"】`,
		"末1秒全体人物定格",
	} {
		if !strings.Contains(compressed, protected) {
			t.Fatalf("safe compression changed protected content %q:\n%s", protected, compressed)
		}
	}
	if stats.ManagementRemoved == 0 || stats.DuplicateClausesRemoved == 0 {
		t.Fatalf("compression report missing counts: %+v", stats)
	}
}

func TestCompressCompiledManjuFeedLeavesUnderBudgetUnitsUnchanged(t *testing.T) {
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】16:9 横屏，古代战场（aspect_ratio=16:9）
【场景资产】@城头（参考图1）
【核心人物】@梁崇（参考图2，声线锁：低沉男声）
【站位声明】梁崇位于画面左侧
【时间轴分镜】
00:00-00:05 [镜头1] 中景 · 平视 · 固定。梁崇击中目标。声音：甲片碰撞。台词：无。
【音效】甲片碰撞
【强制禁止项】画面：禁字幕、禁水印；音频：禁背景音乐、禁旁白。
` + "```" + `
`

	compressed, _ := compressCompiledManjuFeed(feed, "seedance-2.5")
	if compressed != feed {
		t.Fatalf("未超预算单元不应被改写:\n%s", compressed)
	}
}

func compiledShotAction(feed string) string {
	for _, line := range strings.Split(feed, "\n") {
		if !strings.Contains(line, "[镜头") {
			continue
		}
		sound := strings.Index(line, "。声音：")
		if sound < 0 {
			continue
		}
		return line[:sound]
	}
	return ""
}
