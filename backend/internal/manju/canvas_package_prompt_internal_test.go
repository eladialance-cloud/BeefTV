package manju

import (
	"strings"
	"testing"
)

func TestP2CanvasTasksUseValidatedAssetBlocksAsImagePrompts(t *testing.T) {
	contract := mustLoadAssetBlockContract()
	content := buildAssetBlockContractWitness(contract) + `

## 素材接线单

| 图号 | 用途 | 只控制什么 | 不控制什么 |
| 图1 | 角色 | 脸型与服装 | 姿态与场景 |
| 图2 | 场景母版 | 空间结构 | 人物与动作 |
| 图3 | 场景九宫格 | 机位与区域 | 人物身份 |
| 图4 | 道具 | 外观与材质 | 手部与场景 |

<!-- MANJU_CANVAS_TASKS_JSON
{"version":1,"tasks":[
  {"code":"图1","assetCode":"CHR-林澈","title":"林澈角色四格","category":"character","prompt":"简化角色摘要","aspectRatio":"16:9"},
  {"code":"图2","assetCode":"SCN-旧宅","title":"旧宅母版","category":"environment","prompt":"简化场景摘要","aspectRatio":"16:9"},
  {"code":"图3","assetCode":"SCN-旧宅","title":"旧宅九宫格","category":"environment","prompt":"简化九宫格摘要","aspectRatio":"16:9"},
  {"code":"图4","assetCode":"PRP-旧表","title":"旧表道具图","category":"prop","prompt":"简化道具摘要","aspectRatio":"16:9"}
]}
-->`

	tasks, err := parseP2CanvasTasks(content, AssetModeM1Value, "16:9")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 4 {
		t.Fatalf("task count = %d, want 4", len(tasks))
	}
	required := map[string][]string{
		"图1": {"#### 图1", "全图共 4 格", "五官 10 项锁定", "≤8 词音色速览标签"},
		"图2": {"#### 图2", "16:9", "无人空镜"},
		"图3": {"#### 图3", "【场景定位】", "全图共 9 格", "1~9 标注降级对照表"},
		"图4": {"#### 图4", "道具设定图", "正面视图", "关键细节特写"},
	}
	for _, task := range tasks {
		for _, needle := range required[task.Code] {
			if !strings.Contains(task.Prompt, needle) {
				t.Fatalf("%s prompt did not use the validated asset block, missing %q:\n%s", task.Code, needle, task.Prompt)
			}
		}
		if strings.Contains(task.Prompt, "简化") {
			t.Fatalf("%s used the model-authored summary instead of the canonical asset block:\n%s", task.Code, task.Prompt)
		}
	}
}

func TestP2CanvasTaskRequiresMatchingV7426AssetBlock(t *testing.T) {
	content := `> 素材形态：M1（编号占位）

## 素材接线单

| 图号 | 用途 | 只控制什么 | 不控制什么 |
| 图1 | 角色 | 脸型与服装 | 姿态与场景 |

<!-- MANJU_CANVAS_TASKS_JSON
{"version":1,"tasks":[{"code":"图1","assetCode":"CHR-林澈","title":"林澈角色四格","category":"character","prompt":"角色四格纯图板，全图共 4 格。","aspectRatio":"16:9"}]}
-->`

	if _, err := parseP2CanvasTasks(content, AssetModeM1Value, "16:9"); err == nil ||
		!strings.Contains(err.Error(), "V7.4.26") {
		t.Fatalf("missing canonical asset block must be rejected, got %v", err)
	}
}

func TestP2CanvasTasksDeriveFromOfficialAssetBlocksWithoutTransportEnvelope(t *testing.T) {
	content := `> 素材形态：M1（编号占位）

#### 图1 · 林澈（角色四格）
16:9；全图共 4 格；五官 10 项锁定；声纹；≤8 词音色速览标签；fifth panel；different face。

## 素材接线单

| 图号 | 资产编码 | 用途 | 只控制什么 | 不控制什么 |
|---|---|---|---|---|
| 图1 | CHR-林澈 | 角色 | 脸型与服装 | 姿态与场景 |`

	tasks, err := parseP2CanvasTasks(content, AssetModeM1Value, "16:9")
	if err != nil {
		t.Fatalf("official asset blocks must be sufficient without MANJU_CANVAS_TASKS_JSON: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.Code != "图1" || task.AssetCode != "CHR-林澈" || task.Category != "character" {
		t.Fatalf("derived task identity mismatch: %+v", task)
	}
	if !strings.Contains(task.Prompt, "#### 图1 · 林澈（角色四格）") ||
		!strings.Contains(task.Prompt, "全图共 4 格") {
		t.Fatalf("task prompt must be the exact official asset block: %+v", task)
	}
	if strings.Contains(task.Prompt, "MANJU_CANVAS_TASKS_JSON") {
		t.Fatal("transport envelope leaked into the official asset prompt")
	}
}

func TestP2CanvasTaskAcceptsSceneMasterWithPunctuationSeparatedFields(t *testing.T) {
	content := `#### 图1 · 城头（场景母版）
16:9；无人空镜；空间类型=室外+半封闭城楼；光线时段=黎明冷灰基准；主色调=冷蓝灰+暖金点光；核心固定参照物=垛口、城楼木柱、城头旗杆、警锣位；布局=城墙最前/城楼前平台/马道/垛口内侧通道；材质=青砖、夯土、木柱、冻土薄冰；氛围=高反差大光比、强轮廓硬光、体积雾、城墙压迫；禁止人物、人影、动物。

<!-- MANJU_CANVAS_TASKS_JSON
{"version":1,"tasks":[{"code":"图1","assetCode":"SCN-城头","title":"城头场景母版","category":"environment","prompt":"摘要","aspectRatio":"16:9"}]}
-->`

	tasks, err := parseP2CanvasTasks(content, AssetModeM1Value, "16:9")
	if err != nil {
		t.Fatalf("scene master fields separated by Chinese punctuation must be accepted: %v", err)
	}
	if len(tasks) != 1 || !strings.Contains(tasks[0].Prompt, "16:9；无人空镜") {
		t.Fatalf("scene master prompt was not preserved: %+v", tasks)
	}
}
