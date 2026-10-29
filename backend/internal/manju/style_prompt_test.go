package manju

import (
	"strings"
	"testing"
)

func TestCompileStylePromptBlockUsesAuthoritativeMainCardBlock(t *testing.T) {
	stylePrompt, performancePrompt, err := compileStylePromptBlock("袁和平", "兵器形制特写 · 布料与皮甲质感", "参考图：中等反差，暖褐主色，低饱和")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"【STYLE｜视觉风格 · 全片锁死】",
		"成像底层：实拍胶片质感",
		"影调区间：光比 [1:3, 1:6]",
		"禁隔空发光对波",
		"参考图风格分析：参考图：中等反差，暖褐主色，低饱和",
		"只迁移光线、色彩、材质、镜头与后期质感",
	} {
		if !strings.Contains(stylePrompt, required) {
			t.Fatalf("style prompt missing %q:\n%s", required, stylePrompt)
		}
	}
	if !strings.Contains(performancePrompt, "【PERFORMANCE｜表演与动作 · 全片锁死】") ||
		!strings.Contains(performancePrompt, "命中硬顿 2~3 帧") {
		t.Fatalf("performance prompt incomplete:\n%s", performancePrompt)
	}
	if strings.Contains(stylePrompt, "袁和平") || strings.Contains(performancePrompt, "袁和平") {
		t.Fatal("model-facing style prompt must not contain director names")
	}
}

func TestStylePrefixMovesToFrontOfImagePrompt(t *testing.T) {
	stylePrompt, _, err := compileStylePromptBlock("徐克", "高反差大光比", "")
	if err != nil {
		t.Fatal(err)
	}
	prompt := prependStylePrompt(stylePrompt, "角色四格立绘任务正文")
	if !strings.HasPrefix(prompt, "【STYLE｜视觉风格 · 全片锁死】") {
		t.Fatalf("style block must be first:\n%s", prompt)
	}
	if strings.Index(prompt, stylePrompt) < 0 || strings.Index(prompt, "角色四格立绘任务正文") < strings.Index(prompt, stylePrompt) {
		t.Fatalf("task prompt order incorrect:\n%s", prompt)
	}
}

func TestHydrateLockStylePromptsUpgradesLegacySnapshotWithoutNewRevision(t *testing.T) {
	snapshot := LockSnapshot{
		StyleBase:             StyleBaseLiveAction,
		StyleCardID:           "sd-style-012",
		StylePromptParameters: "兵器形制特写 · 布料与皮甲质感",
	}
	if err := hydrateLockStylePrompts(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.StyleCardName != "袁和平" ||
		!strings.Contains(snapshot.StyleImagePrompt, "光比 [1:3, 1:6]") ||
		!strings.Contains(snapshot.StylePerformancePrompt, "命中硬顿 2~3 帧") {
		t.Fatalf("legacy snapshot was not hydrated: %+v", snapshot)
	}
}
