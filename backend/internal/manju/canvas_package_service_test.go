package manju_test

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestCanvasPackageBuildsP2AssetTasksFromDeclaredManifest(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	artifact := &model.ManjuAssetArtifact{}
	if err := f.db.First(artifact, "run_id = ? AND kind = ?", assetRunID, model.ManjuAssetArtifactM1).Error; err != nil {
		t.Fatal(err)
	}
	artifact.Content = `> 素材形态：M1（编号占位 · 图号见接线单）

#### 图1 · 林澈（角色四格）
16:9；全图共 4 格；五官 10 项锁定；声纹；≤8 词音色速览标签；fifth panel；different face；身份外观三件套；Voice Description。

## 素材接线单
| 图号 | 用途 | 只控制什么 | 不控制什么 |
| 图1 | 角色 | 脸型 | 动作 |

` +
		"<!-- MANJU_CANVAS_TASKS_JSON\n" +
		`{"version":1,"tasks":[{"code":"图1","assetCode":"CHR-林澈","title":"林澈角色四格","category":"character","prompt":"角色四格纯图板，全图共 4 格，面部特写、正面、侧面、背面。","aspectRatio":"16:9","tags":["P2","角色"]}]}` +
		"\n-->"
	if err := f.db.Save(artifact).Error; err != nil {
		t.Fatal(err)
	}

	pkg, err := f.service.CanvasPackage(f.ownerID, f.projectID, manju.CanvasPackageStageP2Assets, assetRunID)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Hash == "" || len(pkg.Tasks) != 1 {
		t.Fatalf("P2 生产包必须包含可执行资产任务和内容指纹: %+v", pkg)
	}
	task := pkg.Tasks[0]
	if task.ID == "" || task.Code != "图1" || task.AssetCode != "CHR-林澈" || task.Kind != "image" || task.Category != "character" {
		t.Fatalf("P2 任务映射字段不完整: %+v", task)
	}
	if !strings.Contains(task.Prompt, "全图共 4 格") {
		t.Fatalf("P2 任务必须原样携带权威资产提示词: %s", task.Prompt)
	}
	if !strings.Contains(task.Prompt, "#### 图1 · 林澈（角色四格）") {
		t.Fatalf("P2 任务必须使用 V7.4.26 图N资产块原文: %s", task.Prompt)
	}
	if strings.Contains(task.Prompt, "【STYLE｜视觉风格 · 全片锁死】") ||
		strings.Contains(task.Prompt, "影像基底：真人实拍质感") {
		t.Fatalf("P2 任务提示词不应重复携带全局风格，风格由画布文本节点统一注入: %s", task.Prompt)
	}
}

func TestP3CanvasPackageSkipsExactSingleShotStoryboardExemption(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	storyboardStageID := findProductionStage(t, run, manju.ProductionStageStoryboard).ID
	artifact := &model.ManjuProductionArtifact{
		ID: "p3-exemption-artifact", ProjectID: f.projectID, RunID: run.Run.ID, StageID: storyboardStageID,
		Kind: model.ManjuProductionArtifactStoryboard, Version: 1, CreatedBy: f.ownerID,
		Content: `# P3 分镜与故事板
## 分镜总表
| 段号 | 镜号 | 时长 | 景别 | 机位 | 运镜 | 画面内容 | 台词 | 音效 | 资产引用 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 段1 | 01 | 2s | 中景 | 固定 | 固定 | 林澈跨门 | 无 | 门声 | CHR-林澈 |
| 段2 | 01 | 5s | 中景 | 固定 | 固定 | 林澈停步 | 无 | 雨声 | CHR-林澈 |

## 故事板表
### 第1段｜过门
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 |
| --- | --- | --- | --- | --- | --- |
| 段1 | 段1-P1 | 起始 | 中景 | 林澈看门 | CHR-林澈 |
| 段1 | 段1-P2 | 出口 | 中景 | 林澈入门 | CHR-林澈 |
### 第2段｜停步
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 |
|---|---|---|---|---|---|
| 段2 | 段2-P1 | 起始 | 中景 | 林澈停步 | CHR-林澈 |
| 段2 | 段2-P2 | 出口 | 中景 | 林澈回头 | CHR-林澈 |

## 台词精简建议清单
本稿全量承接上游台词，无精简建议。

## P3 交付清单
| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |
|---|---|---|---|
| 段1 | 覆盖：场1 ✅ 四宫格 | ⛔豁免（单镜过场段） | 理由：段1只有一个过门动作，动作相位已完整列入分镜总表。 |
| 段2 | 覆盖：场2 ✅ 四宫格 | 4格(2×2 四宫格) | 无豁免；必须生成段2故事板图。 |`,
	}
	if err := f.db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", storyboardStageID).Update("artifact_id", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}

	pkg, err := f.service.CanvasPackage(f.ownerID, f.projectID, manju.CanvasPackageStageP3Images, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundStoryboardSegment1 := false
	foundStoryboardSegment2 := false
	for _, task := range pkg.Tasks {
		if task.Group == "storyboard" && task.Code == "段1" {
			foundStoryboardSegment1 = true
		}
		if task.Group == "storyboard" && task.Code == "段2" {
			foundStoryboardSegment2 = true
		}
	}
	if foundStoryboardSegment1 || !foundStoryboardSegment2 {
		t.Fatalf("only non-exempt storyboard segments may become image tasks: %+v", pkg.Tasks)
	}
	foundExemption := false
	for _, exemption := range pkg.Exemptions {
		if exemption.Kind == "storyboard" && exemption.Code == "段1" {
			foundExemption = true
			if strings.TrimSpace(exemption.Reason) == "" {
				t.Fatalf("storyboard exemption must retain a substantive reason: %+v", exemption)
			}
		}
	}
	if !foundExemption {
		t.Fatalf("expected a storyboard exemption for 段1: %+v", pkg.Exemptions)
	}
}

func TestP3CanvasPackageRejectsLooseOrMalformedStoryboardExemption(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	storyboardStageID := findProductionStage(t, run, manju.ProductionStageStoryboard).ID
	artifact := &model.ManjuProductionArtifact{
		ID: "p3-loose-exemption-artifact", ProjectID: f.projectID, RunID: run.Run.ID, StageID: storyboardStageID,
		Kind: model.ManjuProductionArtifactStoryboard, Version: 1, CreatedBy: f.ownerID,
		Content: `# P3 分镜与故事板
## 分镜总表
| 段号 | 镜号 | 时长 | 画面内容 | 台词 | 音效 | 资产引用 |
|---|---|---|---|---|---|---|
| 段1 | 01 | 3s | 林澈停步 | 无 | 雨声 | CHR-林澈 |
## 故事板表
### 第1段｜停步
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 |
|---|---|---|---|---|---|
| 段1 | 段1-P1 | 起始 | 中景 | 林澈停步 | CHR-林澈 |
## 台词精简建议清单
无
## P3 交付清单
故事板图：段1 ⛔豁免（单镜过场段） 理由：动作相位已列。`,
	}
	if err := f.db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", storyboardStageID).Update("artifact_id", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := f.service.CanvasPackage(f.ownerID, f.projectID, manju.CanvasPackageStageP3Images, run.Run.ID); err == nil {
		t.Fatal("loose storyboard exemption text must not satisfy the checklist table")
	}
}

func TestP3CanvasPackageResolvesOnlyCurrentSegmentStoryboardFigures(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	assetArtifact := &model.ManjuAssetArtifact{}
	if err := f.db.First(assetArtifact, "run_id = ? AND kind = ?", assetRunID, model.ManjuAssetArtifactM1).Error; err != nil {
		t.Fatal(err)
	}
	assetArtifact.Content = `> 素材形态：M1（编号占位 · 图号见接线单）
# 资产编号册
图1 = CHR-林澈
图2 = SCN-旧宅
图3 = PRP-竹简
`
	if err := f.db.Save(assetArtifact).Error; err != nil {
		t.Fatal(err)
	}

	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	storyboardStageID := findProductionStage(t, run, manju.ProductionStageStoryboard).ID
	artifact := &model.ManjuProductionArtifact{
		ID: "p3-storyboard-figure-artifact", ProjectID: f.projectID, RunID: run.Run.ID, StageID: storyboardStageID,
		Kind: model.ManjuProductionArtifactStoryboard, Version: 1, CreatedBy: f.ownerID,
		Content: `# P3 分镜与故事板
## 分镜总表
| 段号 | 镜号 | 时长 | 景别 | 机位 | 运镜 | 画面内容 | 台词 | 音效 | 资产引用 |
|---|---|---|---|---|---|---|---|---|---|
| 段1 | 01 | 5s | 中景 | 侧45度 | 固定 | 林澈推门 | 无 | 门声 | 图1 / 图2 |
| 段2 | 01 | 4s | 近景 | 平视 | 固定 | 竹简落在桌上 | 无 | 纸响 | 图3 |

## 故事板表
### 第1段｜夜归
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 |
|---|---|---|---|---|---|
| 段1 | 段1-P1 | 起始 | 中景 | 林澈在门外看向门缝 | 图1 / 图2 |
| 段1 | 段1-P2 | 过程 | 近景 | 林澈推门 | 图1 / 图2 |
| 段1 | 段1-P3 | 反应 | 特写 | 林澈瞳孔收紧 | 图1 |
| 段1 | 段1-P4 | 出口 | 中景 | 林澈停在门内 | 图1 / 图2 |
### 第2段｜竹简
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 |
|---|---|---|---|---|---|
| 段2 | 段2-P1 | 起始 | 近景 | 竹简在桌面上 | 图2 / 图3 |
| 段2 | 段2-P2 | 过程 | 特写 | 指尖压住竹简 | 图3 |
| 段2 | 段2-P3 | 反应 | 特写 | 竹简停住 | 图3 |
| 段2 | 段2-P4 | 出口 | 中景 | 竹简留在桌面 | 图2 / 图3 |

## 台词精简建议清单
无

## P3 交付清单
| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |
|---|---|---|---|
| 段1 | 覆盖：场1 ✅ 四宫格 | 4格(2×2 四宫格) | 无豁免；必须生成段1故事板图。 |
| 段2 | 覆盖：场2 ✅ 四宫格 | 4格(2×2 四宫格) | 无豁免；必须生成段2故事板图。 |
`,
	}
	if err := f.db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", storyboardStageID).Update("artifact_id", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}

	pkg, err := f.service.CanvasPackage(f.ownerID, f.projectID, manju.CanvasPackageStageP3Images, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	taskByCode := make(map[string]manju.CanvasPackageTask)
	for _, task := range pkg.Tasks {
		if task.Group == "storyboard" {
			taskByCode[task.Code] = task
		}
	}
	if got := taskByCode["段1"].ReferenceCodes; strings.Join(got, ",") != "CHR-林澈,SCN-旧宅" {
		t.Fatalf("段1 只应接线自己的图1/图2，实际为 %+v", got)
	}
	if got := taskByCode["段2"].ReferenceCodes; strings.Join(got, ",") != "SCN-旧宅,PRP-竹简" {
		t.Fatalf("段2 只应接线自己的图2/图3，实际为 %+v", got)
	}
	if strings.Contains(taskByCode["段1"].Prompt, "PRP-竹简") ||
		strings.Contains(taskByCode["段2"].Prompt, "CHR-林澈") {
		t.Fatalf("故事板提示词不得混入其他段落的全部资产: 段1=%s\n段2=%s", taskByCode["段1"].Prompt, taskByCode["段2"].Prompt)
	}
}
