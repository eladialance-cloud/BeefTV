package manju

import (
	"strings"
	"testing"
)

func TestParseStoryboardImageSegmentsAcceptsPerSegmentTables(t *testing.T) {
	content := `# P3 分镜与故事板

## 分镜总表
| 段 | 镜号 | 时长 | 画面 |
|---|---|---|---|
| 段1 | 1 | 3 | 城头对峙 |

## 故事板表

### 第1段｜故事板 4 格（2×2）

| 格号 | 动作相位 | 镜头信息 | 站位+视线 | 资产引用 | 面板边界备注 |
|---|---|---|---|---|---|
| 段1-P1 | 起势 | 中景，固定 | 林澈看向城下 | CHR-林澈 / AUD-B-01 | 入口帧 |
| 段1-P2 | 过程 | 近景，缓推 | 林澈握紧竹简 | CHR-林澈、PROP-竹简 | 动作连续 |
| 段1-P3 | 顶点 | 特写，手持 | 竹简被攥皱 | PROP-竹简 | 不剪 |
| 段1-P4 | 出口 | 中景，拉远 | 林澈转身 | CHR-林澈 | 出口帧 |

### 第2段｜故事板 4 格（2×2）

| 格号 | 动作相位 | 镜头信息 | 站位+视线 | 资产引用 | 面板边界备注 |
|---|---|---|---|---|---|
| 段2-P1 | 起势 | 全景，固定 | 林澈走向城门 | CHR-林澈、SCN-城门 | 入口帧 |
| 段2-P2 | 过程 | 中景，跟移 | 林澈停步 | CHR-林澈、SCN-城门 | 动作连续 |
| 段2-P3 | 顶点 | 近景，固定 | 林澈抬眼 | CHR-林澈 | 不剪 |
| 段2-P4 | 出口 | 远景，升镜 | 林澈进入城门 | CHR-林澈、SCN-城门 | 出口帧 |

## P3 交付清单
| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |
|---|---|---|---|
| 段1 | 场1 | 4格 | 无豁免 |
| 段2 | 场2 | 4格 | 无豁免 |
`

	segments, err := parseStoryboardImageSegments(content)
	if err != nil {
		t.Fatalf("合法的逐段故事板表必须可解析: %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("两段故事板应归并为两个任务，实际为 %d: %+v", len(segments), segments)
	}
	if segments[0].Code != "段1" || segments[1].Code != "段2" {
		t.Fatalf("段号必须归一化，实际为 %q 和 %q", segments[0].Code, segments[1].Code)
	}
	for index, segment := range segments {
		if len(segment.Shots) != 4 || len(segment.Phases) != 4 || len(segment.Camera) != 4 || len(segment.Block) != 4 {
			t.Fatalf("第 %d 段没有完整保留四格信息: %+v", index+1, segment)
		}
		if len(segment.Assets) == 0 {
			t.Fatalf("第 %d 段没有保留资产引用: %+v", index+1, segment)
		}
	}
	if segments[0].Shots[0] != "段1-P1" || segments[0].Phases[0] != "起势" {
		t.Fatalf("故事板格内容解析错误: %+v", segments[0])
	}
	if len(segments[0].Panels) != 4 || segments[0].Panels[0].BoundaryNote != "入口帧" {
		t.Fatalf("故事板逐格字段没有完整保留: %+v", segments[0].Panels)
	}

	prompt := buildStoryboardImagePrompt(segments[0], AssetModeM1Value, "9:16", "live-action", "冷青灰 / 烛火暖橙")
	for _, required := range []string{
		"生成一张 16:9 横版的故事板图（CLEAN_GRID 干净版）",
		"4 个等大的面板（2×2 四宫格）",
		"面板 1 / 段1-P1（起势）",
		"中景，固定",
		"资产引用：CHR-林澈",
		"面板边界备注：入口帧",
		"面板只表示动作相位，不是剪辑点",
		"禁止：任何文字、字幕、对白框、序号、箭头、水印",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("故事板出图提示词缺少 V7.4.26 CLEAN_GRID 字段 %q:\n%s", required, prompt)
		}
	}
	if !strings.Contains(prompt, "AUD-B-01") {
		t.Fatalf("故事板提示词必须完整保留源表中的声音资产引用:\n%s", prompt)
	}
}

func TestP3StoryboardExemptionDoesNotTreatNoExemptionAsExempt(t *testing.T) {
	segment := p3RequiredSegment{Code: "段1", Key: "1", ShotCount: 1}
	exempt, _, err := p3ParseStoryboardExemption(
		"无豁免；等待故事板图",
		"理由：普通图片任务尚未完成",
		segment,
	)
	if err != nil {
		t.Fatal(err)
	}
	if exempt {
		t.Fatal("“无豁免”不得被当成已豁免")
	}
}

func TestP3PlotExemptionRejectsCompositePlaceholders(t *testing.T) {
	testCases := []string{
		"理由：暂无；后续补充",
		"理由：略；见上文",
		"理由：待补 / 后续补充",
		"理由：TODO；补充",
		"理由：……",
		"理由：略……",
		"理由：同上……",
		"理由：略.…",
	}
	for _, reason := range testCases {
		t.Run(reason, func(t *testing.T) {
			exempt, _, err := parseP3PlotExemption(
				"覆盖：场1 ⛔豁免（纯空镜场）",
				reason,
				"场1",
			)
			if err == nil {
				t.Fatalf("composite placeholder must fail: exempt=%v reason=%q", exempt, reason)
			}
		})
	}
}

func TestParseStoryboardImageSegmentsWithoutBoundaryColumn(t *testing.T) {
	content := `## 故事板表
| 段 | 格号 | 动作相位 | 镜头信息 | 站位+视线 | 资产引用 |
|---|---|---|---|---|---|
| 段1 | 段1-P1 | 起势 | 中景 | 林澈看向城门 | CHR-林澈 |
| 段1 | 段1-P2 | 出口 | 近景 | 林澈转身 | CHR-林澈 |
`

	segments, err := parseStoryboardImageSegments(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || len(segments[0].Panels) != 2 {
		t.Fatalf("故事板分格解析错误: %+v", segments)
	}
	if segments[0].Panels[0].BoundaryNote != "" {
		t.Fatalf("没有边界备注列时不得把格号误当成备注: %+v", segments[0].Panels[0])
	}
}

func TestParseStoryboardImageSegmentsPreservesCanonicalFieldsAndTitle(t *testing.T) {
	content := `## 故事板表

### 第1段｜暴雨夜归

| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 | 面板边界备注 |
|---|---|---|---|---|---|---|
| 第1段 | 段1-P1 | 起始 | 中景 · 侧45度+平视 | 林澈在左前景，面朝门内，视线落向门缝 | 图1 / 图2 | 入口帧 |
| 第1段 | 段1-P2 | 过程 | 近景 · 手持微晃 | 林澈右手推门，重心前压 | 图1 / 图2 | 动作连续 |
| 第1段 | 段1-P3 | 反应 | 特写 · 侧脸 | 林澈瞳孔收紧，呼吸停顿 | 图1 | 不剪 |
| 第1段 | 段1-P4 | 出口 | 中景 · 平视 | 林澈停在门内，右手扶门，左脚踏过门槛 | 图1 / 图2 | 出口帧 |
`

	segments, err := parseStoryboardImageSegments(content)
	if err != nil {
		t.Fatalf("原版故事板表头必须可解析: %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("应解析一段故事板，实际为 %d: %+v", len(segments), segments)
	}
	segment := segments[0]
	if segment.Title != "暴雨夜归" {
		t.Fatalf("段落标题必须保留，实际为 %q", segment.Title)
	}
	if len(segment.Panels) != 4 {
		t.Fatalf("应保留四格，实际为 %+v", segment.Panels)
	}
	if segment.Panels[0].ID != "段1-P1" ||
		segment.Panels[0].Block != "林澈在左前景，面朝门内，视线落向门缝" ||
		segment.Panels[0].Assets != "图1 / 图2" {
		t.Fatalf("原版字段发生串位或丢失: %+v", segment.Panels[0])
	}

	prompt := buildStoryboardImagePrompt(segment, AssetModeM1Value, "9:16", "live-action", "冷青灰 / 烛火暖橙")
	for _, required := range []string{
		"本图为第 1 段「暴雨夜归」的故事板",
		"面板 1 / 段1-P1（起始）",
		"中景 · 侧45度+平视",
		"林澈在左前景，面朝门内，视线落向门缝",
		"资产引用：图1 / 图2",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("故事板提示词未承接原版字段 %q:\n%s", required, prompt)
		}
	}
	if strings.Contains(prompt, "镜头信息未提供") {
		t.Fatalf("合法镜头信息不得被替换为缺省值:\n%s", prompt)
	}
}

func TestParseStoryboardImageSegmentsAcceptsCompactProductionTable(t *testing.T) {
	content := `## 故事板表
### 第1段｜边城风雪
| 格号 | 动作相位 | 景别/焦段/机位/运镜 | 站位+视线 | 资产引用 |
|---|---|---|---|---|
| 1 | 起势（宏观揭示） | 大远景/18mm/高空俯视15°/缓慢推进 | 城外→城头纵深，视线向孤城汇聚 | SCN-01（图12）、CHR-12 |
| 2 | 过程（地面营地） | 远景/24mm/地面平视略低/横移 | 帐幕间横移，视线扫过铁器反光 | SCN-01（图12）、CHR-12 |
| 3 | 过程（城头守军） | 中景/35mm/城头平视/固定 | 垛口后守军群像，视线向城外 | SCN-01（图13）、CHR-13 |
| 4 | 接触（号角近部） | 特写/85mm/平视/固定 | 号角与冻手，视线不适用 | AUD-01、CHR-13局部 |
| 5 | 结果（烟柱与霜） | 远景/24mm/高空/固定 | 烟柱升空，视线向上 | SCN-01（图12） |
| 6 | 出口（城头剪影定格） | 中景/35mm/平视/固定 | 守军与垛口同框，视线向外 | SCN-01（图13）、CHR-13 |
`

	segments, err := parseStoryboardImageSegments(content)
	if err != nil {
		t.Fatalf("实际 P3 紧凑故事板表必须可解析: %v", err)
	}
	if len(segments) != 1 || len(segments[0].Panels) != 6 {
		t.Fatalf("紧凑故事板表应解析为一段六格，实际为 %+v", segments)
	}
	panel := segments[0].Panels[0]
	if panel.ID != "1" ||
		panel.Camera != "大远景/18mm/高空俯视15°/缓慢推进" ||
		panel.Block != "城外→城头纵深，视线向孤城汇聚" ||
		panel.Assets != "SCN-01（图12）、CHR-12" {
		t.Fatalf("紧凑故事板表字段解析错误: %+v", panel)
	}
	if got := visualCanvasCodes("SCN-01（图12）、CHR-12"); len(got) != 2 ||
		!strings.Contains(strings.Join(got, ","), "SCN-01") ||
		!strings.Contains(strings.Join(got, ","), "CHR-12") {
		t.Fatalf("资产编号不得吞入括注或后续资产，实际为 %+v", got)
	}
	prompt := buildStoryboardImagePrompt(segments[0], AssetModeM1Value, "9:16", "live-action", "冷青灰 / 烛火暖橙")
	for _, required := range []string{
		"本图为第 1 段「边城风雪」的故事板",
		"各面板资产引用以逐格字段为准",
		"面板 1 / 1（起势（宏观揭示））",
		"大远景/18mm/高空俯视15°/缓慢推进",
		"城外→城头纵深，视线向孤城汇聚",
		"资产引用：SCN-01（图12）、CHR-12",
		"资产引用：AUD-01、CHR-13局部",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("紧凑故事板提示词缺少 %q:\n%s", required, prompt)
		}
	}
	if strings.Contains(prompt, "执行约束：") {
		t.Fatalf("提示词不得追加源故事板表之外的任务说明:\n%s", prompt)
	}
	referenceCodes := storyboardCanvasReferenceCodes([]string{
		"SCN-01（图12）、CHR-12",
		"AUD-01、CHR-13局部",
	}, nil)
	if strings.Join(referenceCodes, ",") != "CHR-12,CHR-13,SCN-01" {
		t.Fatalf("故事板画布接线必须提取干净的可解析视觉编号，实际为 %+v", referenceCodes)
	}
}

func TestVisualCanvasCodesSplitCompositeCellsAndExcludeAudio(t *testing.T) {
	codes := visualCanvasCodes(
		"CHR-A-01 / PRP-A-02 / AUD-B-01",
		"SCN-A-01、AUD-C-02",
		"AUD-A-01",
	)
	expected := []string{"CHR-A-01", "PRP-A-02", "SCN-A-01"}
	if len(codes) != len(expected) {
		t.Fatalf("视觉资产编号提取错误: %+v", codes)
	}
	for index := range expected {
		if codes[index] != expected[index] {
			t.Fatalf("视觉资产编号提取错误: %+v", codes)
		}
	}
}

func TestParseStoryboardImageSegmentsRejectsImplicitSequentialCode(t *testing.T) {
	content := `## 故事板表
| 格号 | 动作相位 | 镜头信息 | 站位+视线 | 资产引用 |
|---|---|---|---|---|
| 1 | 起始 | 中景 | 林澈看门 | CHR-林澈 |
| 2 | 出口 | 中景 | 林澈入门 | CHR-林澈 |
`
	if _, err := parseStoryboardImageSegments(content); err == nil {
		t.Fatal("storyboard rows without an exact segment id or heading must not fall back to 段1")
	}
}

func TestParseStoryboardImageSegmentsRejectsMalformedRowEvenWhenOtherRowsAreValid(t *testing.T) {
	content := `## 故事板表
### 第1段｜停步
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 |
|---|---|---|---|---|---|
| 段1 | 段1-P1 | 起始 | 中景 | 林澈停步 | CHR-林澈 |
| 段1-extra | 段1-extra-P1 | 出口 | 中景 | 林澈回头 | CHR-林澈 |
`
	if _, err := parseStoryboardImageSegments(content); err == nil {
		t.Fatal("an invalid segment row must fail the entire storyboard table")
	}
}

func TestStoryboardCanvasReferenceCodesUsesOnlyExplicitStoryboardFigures(t *testing.T) {
	figureAssets := parseCanvasFigureAssetMap(`> 素材形态：M1（编号占位 · 图号见接线单）
# 资产编号册
图1 = CHR-林澈
图2 = SCN-旧宅
图3 = PRP-竹简
`)

	codes := storyboardCanvasReferenceCodes([]string{"图1 / 图2"}, figureAssets)
	if strings.Join(codes, ",") != "CHR-林澈,SCN-旧宅" {
		t.Fatalf("故事板参考图必须按当前格显式图号接线，实际为 %+v", codes)
	}
	if strings.Contains(strings.Join(codes, ","), "PRP-竹简") {
		t.Fatalf("未在故事板格中引用的资产不得兜底加入: %+v", codes)
	}
}

func TestProductionStoryboardRuntimeInstructionDoesNotDuplicateTableTemplate(t *testing.T) {
	instruction := productionStageInstruction(ProductionStageStoryboard, AssetModeM1Value)
	for _, required := range []string{
		"### 第N段｜段落标题",
		"段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 | 面板边界备注",
		"资产引用优先写 P2《素材接线单》中的 `图N`",
	} {
		if strings.Contains(instruction, required) {
			t.Fatalf("P3 runtime prompt duplicated official table field %q:\n%s", required, instruction)
		}
	}
}
