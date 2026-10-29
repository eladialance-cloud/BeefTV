package manju

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestNormalizeLockSplitsLegacyCustomStyleIntoPersistedFields(t *testing.T) {
	rawStyle := strings.Join([]string{
		"北宋末年历史战争片，电影级质感，2.39:1变形宽银幕画幅，低饱和青灰冷色调与暖色点光源对撞",
		"【手持类】handheld camera, subtle 3-5 degree dutch angle / 手持摄影，3-5度倾斜角，纪录片式微晃",
		"现代元素，鲜艳色彩，干净崭新的服装，棚拍感，塑料质感铠甲",
	}, "\n")
	snapshot, err := normalizeLock(LockInput{
		Track:                  "domestic",
		Source:                 "adaptation",
		VideoModel:             "seedance-2.5",
		AspectRatio:            "9:16",
		DeliveryMode:           "both",
		TargetDurationSeconds:  15,
		StyleBase:              StyleBaseLiveAction,
		StyleCardID:            StyleCustomCardID,
		StyleCustomDescription: rawStyle,
		AssetMode:              AssetModeM1,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(payload, &stored); err != nil {
		t.Fatal(err)
	}
	visual, _ := stored["styleVisualPrompt"].(string)
	motion, _ := stored["styleMotionParameters"].(string)
	forbidden, _ := stored["styleForbiddenItems"].([]any)

	if !strings.Contains(visual, "北宋末年历史战争片") {
		t.Fatalf("positive visual style was not persisted separately: %+v", stored)
	}
	if strings.Contains(visual, "手持摄影") || strings.Contains(visual, "现代元素") || strings.Contains(visual, "2.39:1") {
		t.Fatalf("visual field still mixes motion, forbidden items or forced aspect: %q", visual)
	}
	if !strings.Contains(motion, "运镜·手持类：") || !strings.Contains(motion, "手持摄影") {
		t.Fatalf("motion field was not persisted separately: %q", motion)
	}
	if len(forbidden) < 2 || !strings.Contains(strings.Join(anyStrings(forbidden), "；"), "现代元素") {
		t.Fatalf("forbidden field was not persisted separately: %+v", forbidden)
	}
	if strings.Contains(snapshot.StylePromptParameters, "手持") || strings.Contains(snapshot.StylePromptParameters, "现代元素") {
		t.Fatalf("legacy compatibility field must contain only positive visual style: %q", snapshot.StylePromptParameters)
	}
}

func TestNormalizeLockPreservesExplicitSeparatedCustomStyleFields(t *testing.T) {
	snapshot, err := normalizeLock(LockInput{
		Track:                  "domestic",
		Source:                 "adaptation",
		VideoModel:             "seedance-2.5",
		AspectRatio:            "9:16",
		DeliveryMode:           "both",
		TargetDurationSeconds:  15,
		StyleBase:              StyleBaseLiveAction,
		StyleCardID:            StyleCustomCardID,
		StyleCustomDescription: "旧字段残留：现代元素，鲜艳色彩，棚拍感",
		StyleVisualPrompt:      "北宋末年历史战争片，低饱和青灰冷色调，做旧铠甲与城砖",
		StyleMotionParameters:  "手持摄影，3-5度倾斜角，纪录片式微晃",
		StyleForbiddenItems:    []string{"现代元素", "鲜艳色彩", "塑料质感铠甲"},
		AssetMode:              AssetModeM1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.StyleVisualPrompt != "北宋末年历史战争片，低饱和青灰冷色调，做旧铠甲与城砖" {
		t.Fatalf("explicit visual style changed unexpectedly: %q", snapshot.StyleVisualPrompt)
	}
	if !strings.Contains(snapshot.StyleMotionParameters, "手持摄影") {
		t.Fatalf("explicit motion parameters were lost: %q", snapshot.StyleMotionParameters)
	}
	if strings.Join(snapshot.StyleForbiddenItems, "；") != "现代元素；鲜艳色彩；塑料质感铠甲" {
		t.Fatalf("explicit forbidden items must override stale legacy text: %+v", snapshot.StyleForbiddenItems)
	}
	if snapshot.StylePromptParameters != snapshot.StyleVisualPrompt {
		t.Fatalf("legacy style parameter field must mirror visual-only style: %q", snapshot.StylePromptParameters)
	}
	if !strings.Contains(snapshot.StylePerformancePrompt, "手持摄影") {
		t.Fatalf("motion parameters must flow into the performance block: %q", snapshot.StylePerformancePrompt)
	}
	for _, blocked := range []string{"现代元素", "鲜艳色彩", "塑料质感铠甲"} {
		if strings.Contains(snapshot.StyleImagePrompt, blocked) {
			t.Fatalf("禁用项 %q 不得进入模型可见图片提示词: %q", blocked, snapshot.StyleImagePrompt)
		}
	}
}

func TestDefaultManjuProjectStateMergesStyleForbiddenItems(t *testing.T) {
	snapshot := LockSnapshot{
		Track:                   "domestic",
		Source:                  "adaptation",
		VideoModel:              "seedance-2.5",
		AspectRatio:             "9:16",
		DeliveryMode:            "both",
		TargetDurationSeconds:   15,
		SegmentCount:            1,
		SingleSegmentMaxSeconds: 30,
		StyleBase:               StyleBaseLiveAction,
		StyleCardID:             StyleCustomCardID,
		StyleCardName:           "自定义画风",
		StyleVisualPrompt:       "北宋末年历史战争片，低饱和青灰冷色调",
		StyleMotionParameters:   "手持摄影，纪录片式微晃",
		StyleForbiddenItems:     []string{"现代元素", "鲜艳色彩"},
		StylePromptParameters:   "北宋末年历史战争片，低饱和青灰冷色调",
		AssetMode:               AssetModeM1,
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	state, err := defaultManjuProjectState("测试项目", &model.ManjuLockReceipt{
		ProjectID:   "project-style-boundary",
		Revision:    1,
		Status:      model.ManjuLockStatusLocked,
		PayloadJSON: string(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
	forbidden := strings.Join(anyStrings(anySlice(anyMap(state["world_anchor"])["forbidden_elements"])), "；")
	if !strings.Contains(forbidden, "现代元素") || !strings.Contains(forbidden, "鲜艳色彩") {
		t.Fatalf("P0 project_state world anchor must inherit forbidden style items: %+v", state["world_anchor"])
	}
	if styleCard := anyString(anyMap(state["locks"])["style_card"]); strings.Contains(styleCard, "现代元素") || strings.Contains(styleCard, "手持") {
		t.Fatalf("P0 style_card must contain only positive visual style: %q", styleCard)
	}
}

func anyStrings(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
