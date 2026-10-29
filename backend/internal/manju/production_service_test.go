package manju_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	localtask "infinite-canvas/backend/internal/task"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func TestCreateProductionRunRequiresCompletedAssetRun(t *testing.T) {
	f := newFixture(t)
	scriptRunID := prepareCompletedScript(t, f, "M1")
	receipt, err := f.service.ProjectDetail(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	assetRun := &model.ManjuAssetRun{
		ID: "asset-active", ProjectID: f.projectID, ScriptRunID: scriptRunID,
		AssetMode: manju.AssetModeM1Value, Status: model.ManjuAssetRunStatusActive,
		CurrentStage: manju.AssetStageCompile, LockRevision: receipt.Lock.Revision,
		CreatedBy: f.ownerID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := f.db.Create(assetRun).Error; err != nil {
		t.Fatal(err)
	}

	_, err = f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRun.ID})

	if !errors.Is(err, manju.ErrProductionAssetNotReady) {
		t.Fatalf("P2 未锁定完成时不得创建 P3，得到 %v", err)
	}
}

func TestP3CannotStartBeforeP0BDecisionIsLocked(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}

	if run.Stages[0].Key != "decision_anchor" {
		t.Fatalf("first production stage must be P0B, got %s", run.Stages[0].Key)
	}
	_, err = f.service.RunProductionStage(
		f.ownerID, f.projectID, run.Run.ID,
		manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{},
	)
	if !errors.Is(err, manju.ErrProductionStageNotReady) {
		t.Fatalf("P3 storyboard must wait for P0B, got %v", err)
	}
}

func TestLegacyFeedCannotReachOfficialCompilerBeforeP0BRepair(t *testing.T) {
	f := newFixture(t)
	run := prepareP4AwaitingConfirmation(t, f)
	f.service.SetSkillValidationEnabledForTest(true)

	var decision model.ManjuProductionStage
	if err := f.db.First(&decision, "run_id = ? AND key = ?", run.Run.ID, manju.ProductionStageDecisionAnchor).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Delete(&decision).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("run_id = ?", run.Run.ID).
		UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("run_id = ? AND key = ?", run.Run.ID, manju.ProductionStageFeed).
		Updates(map[string]any{
			"status": model.ManjuProductionStageStatusInProgress, "task_id": "",
			"artifact_id": "", "error": "", "completed_at": nil,
		}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionRun{}).
		Where("id = ?", run.Run.ID).
		Update("current_stage", manju.ProductionStageFeed).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := f.service.RunProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed,
		manju.ProductionStageRunInput{},
	); !errors.Is(err, manju.ErrProductionStageNotReady) {
		t.Fatalf("legacy P4 must not reach compiler before P0B repair, got %v", err)
	}
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor); got.Status != model.ManjuProductionStageStatusInProgress {
		t.Fatalf("legacy repair must insert P0B before dispatch: %+v", got)
	}
	if got := findProductionStage(t, detail, manju.ProductionStageStoryboard); got.Status != model.ManjuProductionStageStatusStale {
		t.Fatalf("legacy P3 must not remain eligible after P0B repair: %+v", got)
	}
	if got := findProductionStage(t, detail, manju.ProductionStageFeed); got.Status != model.ManjuProductionStageStatusNotStarted || got.ArtifactID != "" {
		t.Fatalf("legacy P4 lineage must be invalidated: %+v", got)
	}
}

func TestProductionP0BPersistsAndLocksBeforeStoryboard(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "production-p0b-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{task}}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunProductionStage(
		f.ownerID, f.projectID, run.Run.ID,
		manju.ProductionStageDecisionAnchor,
		manju.ProductionStageRunInput{},
	); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"节拍 #",
		"核心组合摘要（80%定死）",
		"sd-decision-engine/SKILL.md",
		"sd-decision-engine/references/emotion-anchor-table.md",
		"永不解锁",
		"LOCKED_SCRIPT_FACT 林澈推开旧宅的门。",
	} {
		if !strings.Contains(creator.requests[0].Prompt, required) {
			t.Fatalf("P0B production prompt missing %q", required)
		}
	}

	succeedManjuTask(t, f, task.ID, validP0BArtifact())
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	stage := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor)
	if stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation || stage.ArtifactID == "" {
		t.Fatalf("valid P0B must persist before confirmation: %+v", stage)
	}
	var artifact model.ManjuProductionArtifact
	if err := f.db.First(&artifact, "id = ?", stage.ArtifactID).Error; err != nil {
		t.Fatal(err)
	}
	if artifact.Kind != model.ManjuProductionArtifactEmotionAnchor || artifact.ContentHash == "" {
		t.Fatalf("P0B production artifact identity or hash missing: %+v", artifact)
	}

	locked, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, run.Run.ID,
		manju.ProductionStageDecisionAnchor,
		manju.ProductionConfirmInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := findProductionStage(t, locked, manju.ProductionStageDecisionAnchor).Status; got != model.ManjuProductionStageStatusLocked {
		t.Fatalf("P0B must lock before P3: %s", got)
	}
	if got := findProductionStage(t, locked, manju.ProductionStageStoryboard).Status; got != model.ManjuProductionStageStatusInProgress {
		t.Fatalf("P3 must start only after P0B lock: %s", got)
	}
}

func TestP0BRejectsMalformedAndAuthorityViolations(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "five column legacy table",
			content: strings.Replace(validP0BArtifact(),
				"| 节拍 # | 情绪关键词 |",
				"| 场次 | 情绪锚点 |",
				1,
			),
			expected: "缺少必需列",
		},
		{
			name: "extra emotion column",
			content: strings.NewReplacer(
				"| 节拍 # | 情绪关键词 | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 |",
				"| 节拍 # | 情绪关键词 | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 | 备注 |",
				"|---|---|---|---|---|---|",
				"|---|---|---|---|---|---|---|",
				"| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 | 长焦+大光圈+空旷留白 | — |",
				"| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 | 长焦+大光圈+空旷留白 | — | 额外 |",
				"| 节拍 2 | 察觉异样 | 女儿瞥见纸角，手指顿住 | E11 | 微距标点+静切动 | — |",
				"| 节拍 2 | 察觉异样 | 女儿瞥见纸角，手指顿住 | E11 | 微距标点+静切动 | — | 额外 |",
				"| 节拍 3 | 强装平静 | 母亲端起杯子继续夹菜 | E08 | 过肩+长焦+手持微动 | — |",
				"| 节拍 3 | 强装平静 | 母亲端起杯子继续夹菜 | E08 | 过肩+长焦+手持微动 | — | 额外 |",
			).Replace(validP0BArtifact()),
			expected: "表头必须严格等于官方六列顺序",
		},
		{
			name: "reordered emotion columns",
			content: strings.Replace(validP0BArtifact(),
				"| 节拍 # | 情绪关键词 | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 |",
				"| 情绪关键词 | 节拍 # | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 |",
				1,
			),
			expected: "表头必须严格等于官方六列顺序",
		},
		{
			name: "short table row",
			content: strings.Replace(validP0BArtifact(),
				"| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 | 长焦+大光圈+空旷留白 | — |",
				"| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 |",
				1,
			),
			expected: "列数为 4",
		},
		{
			name: "two beats only",
			content: strings.Replace(validP0BArtifact(),
				"| 节拍 3 | 强装平静 | 母亲端起杯子继续夹菜 | E08 | 过肩+长焦+手持微动 | — |\n",
				"",
				1,
			),
			expected: "必须包含 3-5 个节拍",
		},
		{
			name:     "invalid context id",
			content:  strings.Replace(validP0BArtifact(), "E05", "E16", 1),
			expected: "只允许 E01-E15 或 EXnn",
		},
		{
			name: "A class unlock without whitelist reason",
			content: strings.Replace(validP0BArtifact(),
				"- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"- A1 正面居中：A类；解锁；配套强对称构图、静态运镜与长焦压缩。",
				1,
			),
			expected: "A 类解锁必须写明白名单理由",
		},
		{
			name: "A class unlock with invalid whitelist reason",
			content: strings.Replace(validP0BArtifact(),
				"白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"白名单理由为看起来高级；配套强对称构图、静态运镜与长焦压缩。",
				1,
			),
			expected: "必须引用官方白名单选项",
		},
		{
			name: "official whitelist reason followed by negation",
			content: strings.Replace(validP0BArtifact(),
				"白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"白名单理由为仪式感但予以否定；配套强对称构图、静态运镜与长焦压缩。",
				1,
			),
			expected: "必须引用官方白名单选项",
		},
		{
			name: "negated official A2 straight-quote reason",
			content: strings.Replace(validP0BArtifact(),
				"- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"- A2 头顶贴边：A类；解锁；白名单理由为非表现\"被天花板压住\" / 牢笼感；配套广角、压迫感。",
				1,
			),
			expected: "必须引用官方白名单选项",
		},
		{
			name: "negated official A5 exact reason",
			content: strings.Replace(validP0BArtifact(),
				"- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"- A5 断关节：A类；解锁；白名单理由为非强调局部（手 / 脚 / 嘴）= 微距 / 特写镜头；配套微距、长焦。",
				1,
			),
			expected: "必须引用官方白名单选项",
		},
		{
			name: "negated A class whitelist reason and constraint",
			content: strings.Replace(validP0BArtifact(),
				"白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"白名单理由为非仪式感；配套非对称布景、静态运镜与长焦压缩。",
				1,
			),
			expected: "否定或矛盾",
		},
		{
			name: "semantically negated A class whitelist clause",
			content: strings.Replace(validP0BArtifact(),
				"白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"白名单理由为并非真正仪式感；配套强对称构图、静态运镜与长焦压缩均不采用。",
				1,
			),
			expected: "否定标记",
		},
		{
			name: "A class paired constraints uniformly rejected",
			content: strings.Replace(validP0BArtifact(),
				"白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩均不采用。",
				1,
			),
			expected: "否定标记",
		},
		{
			name: "A class unlock missing paired constraint",
			content: strings.Replace(validP0BArtifact(),
				"白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"白名单理由为仪式感。",
				1,
			),
			expected: "缺少配对约束",
		},
		{
			name: "malformed whitelist table row",
			content: strings.Replace(validP0BArtifact(),
				"- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"| 雷区 | 判定 |\n| --- | --- |\n| A1 正面居中 | 解锁 | 缺少判定 |",
				1,
			),
			expected: "列数为 3",
		},
		{
			name: "missing B class authority",
			content: strings.Replace(validP0BArtifact(),
				"- B类 · 永不解锁：物体穿身、角色服装突变、杂乱前景刺眼物、比例失调、重复元素无重点、空鸡蛋构图、道具状态突变。\n",
				"",
				1,
			),
			expected: "缺少 B 类判定",
		},
		{
			name: "B class hazard in unlock column",
			content: strings.Replace(validP0BArtifact(),
				"| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 | 长焦+大光圈+空旷留白 | — |",
				"| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 | 长焦+大光圈+空旷留白 | 物体穿身解锁 |",
				1,
			),
			expected: "B 类技术雷区不得出现在解锁雷区",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			assetRunID := prepareCompletedProductionAsset(t, f, "M1")
			run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
			if err != nil {
				t.Fatal(err)
			}
			task := &model.Task{
				ID: "task-p0b-" + strings.ReplaceAll(testCase.name, " ", "-"), UserID: f.ownerID,
				ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued,
			}
			if err := f.db.Create(task).Error; err != nil {
				t.Fatal(err)
			}
			f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)
			if _, err := f.service.RunProductionStage(
				f.ownerID, f.projectID, run.Run.ID,
				manju.ProductionStageDecisionAnchor,
				manju.ProductionStageRunInput{},
			); err != nil {
				t.Fatal(err)
			}
			succeedManjuTask(t, f, task.ID, testCase.content)
			detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
			if err != nil {
				t.Fatal(err)
			}
			stage := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor)
			if stage.Status != model.ManjuProductionStageStatusBlocked ||
				stage.ArtifactID != "" ||
				!strings.Contains(stage.Error, testCase.expected) {
				t.Fatalf("invalid P0B must block before persistence: %+v", stage)
			}
		})
	}
}

func TestP0BAcceptsCarriedOfficialA2AndA5WhitelistOptions(t *testing.T) {
	testCases := []struct {
		name        string
		hazard      string
		reason      string
		constraints string
	}{
		{
			name:        "A2 authoritative straight quotes",
			hazard:      "A2 头顶贴边",
			reason:      `表现"被天花板压住" / 牢笼感`,
			constraints: "广角、压迫感",
		},
		{
			name:        "A5 authoritative full option",
			hazard:      "A5 断关节",
			reason:      "强调局部（手 / 脚 / 嘴）= 微距 / 特写镜头",
			constraints: "微距、长焦",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			assetRunID := prepareCompletedProductionAsset(t, f, "M1")
			run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
			if err != nil {
				t.Fatal(err)
			}
			task := &model.Task{
				ID:     "task-p0b-carried-" + strings.ReplaceAll(strings.ToLower(testCase.name), " ", "-"),
				UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued,
			}
			if err := f.db.Create(task).Error; err != nil {
				t.Fatal(err)
			}
			f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)
			if _, err := f.service.RunProductionStage(
				f.ownerID, f.projectID, run.Run.ID,
				manju.ProductionStageDecisionAnchor,
				manju.ProductionStageRunInput{},
			); err != nil {
				t.Fatal(err)
			}
			content := strings.Replace(
				validP0BArtifact(),
				"- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"- "+testCase.hazard+"：A类；解锁；白名单理由为"+testCase.reason+"；配套"+testCase.constraints+"。",
				1,
			)
			succeedManjuTask(t, f, task.ID, content)
			detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
			if err != nil {
				t.Fatal(err)
			}
			stage := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor)
			if stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation || stage.ArtifactID == "" {
				t.Fatalf("official carried whitelist option must pass validation: %+v", stage)
			}
		})
	}
}

func TestP0BAcceptsOfficialA4WhitelistReasons(t *testing.T) {
	testCases := []struct {
		name   string
		reason string
	}{
		{
			name:   "invisible force",
			reason: "看不见的力量",
		},
		{
			name:   "unseen threat source",
			reason: "看不到的威胁源",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			assetRunID := prepareCompletedProductionAsset(t, f, "M1")
			run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
			if err != nil {
				t.Fatal(err)
			}
			task := &model.Task{
				ID: "task-p0b-a4-" + strings.ReplaceAll(testCase.name, " ", "-"), UserID: f.ownerID,
				ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued,
			}
			if err := f.db.Create(task).Error; err != nil {
				t.Fatal(err)
			}
			f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)
			if _, err := f.service.RunProductionStage(
				f.ownerID, f.projectID, run.Run.ID,
				manju.ProductionStageDecisionAnchor,
				manju.ProductionStageRunInput{},
			); err != nil {
				t.Fatal(err)
			}
			content := strings.NewReplacer(
				"- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。",
				"- A4 眼神无空间：A类；解锁；白名单理由为"+testCase.reason+"；配套长焦/虚化背景、凝视虚空、心理失序。",
			).Replace(validP0BArtifact())
			succeedManjuTask(t, f, task.ID, content)
			detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
			if err != nil {
				t.Fatal(err)
			}
			stage := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor)
			if stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation || stage.ArtifactID == "" {
				t.Fatalf("official A4 whitelist reason must pass validation: %+v", stage)
			}
		})
	}
}

func TestLegacyActiveProductionRunGetsDecisionAnchorWithoutDeletingHistory(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	decision := findProductionStage(t, run, manju.ProductionStageDecisionAnchor)
	storyboard := findProductionStage(t, run, manju.ProductionStageStoryboard)
	if err := f.db.Where("id = ?", decision.ID).Delete(&model.ManjuProductionStage{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", storyboard.ID).Updates(map[string]any{
		"position": 1, "status": model.ManjuProductionStageStatusAwaitingConfirmation,
		"artifact_id": "legacy-storyboard-artifact",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("run_id = ? AND key <> ?", run.Run.ID, manju.ProductionStageStoryboard).
		UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionRun{}).Where("id = ?", run.Run.ID).Update("current_stage", manju.ProductionStageStoryboard).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &model.ManjuProductionArtifact{
		ID: "legacy-storyboard-artifact", ProjectID: f.projectID, RunID: run.Run.ID,
		StageID: storyboard.ID, Kind: model.ManjuProductionArtifactStoryboard, Version: 1,
		Content: validStoryboardArtifact(), ContentHash: "legacy-storyboard-hash",
		CreatedBy: f.ownerID, CreatedAt: time.Now(),
	}
	if err := f.db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	image := &model.ManjuProductionImage{
		ID: "legacy-storyboard-image", ProjectID: f.projectID, RunID: run.Run.ID,
		StageID: storyboard.ID, Position: 1, SegmentCode: "段1", ShotLabel: "起始",
		Prompt: "legacy", Status: manju.ProductionImageStatusReady, ResourceID: "legacy-resource",
		CreatedBy: f.ownerID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := f.db.Create(image).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Run.CurrentStage != manju.ProductionStageDecisionAnchor {
		t.Fatalf("legacy run must return to P0B: %+v", detail.Run)
	}
	repairedDecision := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor)
	repairedStoryboard := findProductionStage(t, detail, manju.ProductionStageStoryboard)
	if repairedDecision.Position != 1 || repairedDecision.Status != model.ManjuProductionStageStatusInProgress {
		t.Fatalf("legacy P0B repair stage is invalid: %+v", repairedDecision)
	}
	if repairedStoryboard.Position != 2 || repairedStoryboard.ArtifactID != artifact.ID {
		t.Fatalf("legacy storyboard history must be preserved after position shift: %+v", repairedStoryboard)
	}
	if len(detail.Artifacts) == 0 || len(detail.Images) == 0 {
		t.Fatalf("legacy repair must not delete artifacts or media: %+v", detail)
	}
	if repairedStoryboard.Status != model.ManjuProductionStageStatusStale ||
		!strings.Contains(repairedStoryboard.Error, "重新生成 P3") {
		t.Fatalf("legacy pre-P0B P3 must require regeneration: %+v", repairedStoryboard)
	}

	locked := lockProductionP0BForTest(t, f, run.Run.ID)
	if got := findProductionStage(t, locked, manju.ProductionStageStoryboard).Status; got != model.ManjuProductionStageStatusStale {
		t.Fatalf("P0B confirmation must not grandfather an old locked P3: %s", got)
	}
	task := &model.Task{
		ID: "legacy-p3-regenerate-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)
	regenerated, err := f.service.RegenerateProductionArtifact(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{Instruction: "按已锁定 P0B 重新生成旧 P3"},
	)
	if err != nil {
		t.Fatalf("stale legacy P3 must be regenerable into a new lineage: %v", err)
	}
	regeneratedP3 := findProductionStage(t, regenerated, manju.ProductionStageStoryboard)
	if regenerated.Run.ID != run.Run.ID ||
		findProductionStage(t, regenerated, manju.ProductionStageDecisionAnchor).Status != model.ManjuProductionStageStatusLocked ||
		regeneratedP3.Status != model.ManjuProductionStageStatusInProgress ||
		regeneratedP3.TaskID != task.ID ||
		regeneratedP3.ArtifactID != "" {
		t.Fatalf("regenerated lineage must start from locked P0B and fresh P3: %+v", regenerated)
	}
	if got := findProductionStage(t, regenerated, manju.ProductionStageFeed); got.Status != model.ManjuProductionStageStatusNotStarted || got.ArtifactID != "" {
		t.Fatalf("old P3 must not feed a new P4: %+v", got)
	}
	findProductionArtifact(t, regenerated, artifact.ID)
	if countModels[model.ManjuProductionArtifact](t, f, run.Run.ID) < 2 ||
		countModels[model.ManjuProductionImage](t, f, run.Run.ID) == 0 {
		t.Fatal("legacy P3 artifacts and media must remain as historical rows")
	}
}

func TestLegacyInProgressProductionStageP3TaskCannotBecomeRepairedArtifact(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	oldResult := `{"mode":"text","text":` + mustJSONString(validStoryboardArtifact()+"\nOLD_PRE_P0B_TASK_RESULT") + `}`
	oldTask := &model.Task{
		ID: "legacy-in-progress-p3-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusSucceeded, ResultJSON: oldResult,
	}
	if err := f.db.Create(oldTask).Error; err != nil {
		t.Fatal(err)
	}
	decision := findProductionStage(t, run, manju.ProductionStageDecisionAnchor)
	storyboard := findProductionStage(t, run, manju.ProductionStageStoryboard)
	if err := f.db.Where("id = ?", decision.ID).Delete(&model.ManjuProductionStage{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("run_id = ?", run.Run.ID).
		UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", storyboard.ID).Updates(map[string]any{
		"status": model.ManjuProductionStageStatusInProgress, "task_id": oldTask.ID,
		"started_at": now, "completed_at": nil, "artifact_id": "", "error": "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionRun{}).
		Where("id = ?", run.Run.ID).
		Update("current_stage", manju.ProductionStageStoryboard).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard,
		manju.ProductionConfirmInput{},
	); !errors.Is(err, manju.ErrProductionStageNotReady) {
		t.Fatalf("confirmation must not reconcile a stale pre-P0B task, got %v", err)
	}
	repaired, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	repairedP3 := findProductionStage(t, repaired, manju.ProductionStageStoryboard)
	if repairedP3.Status != model.ManjuProductionStageStatusNotStarted ||
		repairedP3.TaskID != "" || repairedP3.ArtifactID != "" ||
		repairedP3.StartedAt != nil || repairedP3.CompletedAt != nil {
		t.Fatalf("repair must clear the old task before P0B: %+v", repairedP3)
	}
	if countModels[model.ManjuProductionArtifact](t, f, run.Run.ID) != 0 {
		t.Fatal("old P3 task result must not become an artifact during repair")
	}

	locked := lockProductionP0BForTest(t, f, run.Run.ID)
	if got := findProductionStage(t, locked, manju.ProductionStageStoryboard).Status; got != model.ManjuProductionStageStatusInProgress {
		t.Fatalf("P3 must be ready for a new task after P0B lock: %s", got)
	}
	newTask := &model.Task{
		ID: "legacy-p3-new-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(newTask).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{newTask}}
	f.service.SetScriptTaskCreator(creator.create)
	started, err := f.service.RunProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	startedP3 := findProductionStage(t, started, manju.ProductionStageStoryboard)
	if startedP3.TaskID != newTask.ID || startedP3.ArtifactID != "" {
		t.Fatalf("P3 must use a new task and no stale result: %+v", startedP3)
	}
	if countModels[model.ManjuProductionArtifact](t, f, run.Run.ID) != 1 {
		t.Fatal("old P3 task must not create an artifact when reconciliation runs")
	}
}

func TestProductionStageDispatchesOfficialCombatRulesFromLockedFacts(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	if err := f.db.Model(&model.ManjuAssetArtifact{}).
		Where("run_id = ?", assetRunID).
		Update("content", "LOCKED_ASSET_FACT\n场景动作：主角御剑施放法阵。").Error; err != nil {
		t.Fatal(err)
	}

	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	task := &model.Task{
		ID: "production-r3-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{task}}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunProductionStage(
		f.ownerID,
		f.projectID,
		run.Run.ID,
		manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{},
	); err != nil {
		t.Fatal(err)
	}
	if len(creator.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(creator.requests))
	}
	if !strings.Contains(creator.requests[0].Prompt, "sd-xuanhuan-engine/references/core-combat-laws.md") {
		t.Fatalf("P3 did not dispatch the official R3 combat rules from locked project facts")
	}
}

func TestProductionChainUsesLockedUpstreamArtifactsAndHumanConfirmation(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	if err := f.db.Create(&model.ManjuScriptSource{
		ID: "raw-novel-secret", ProjectID: f.projectID, RunID: "script-ready-M1",
		Branch: manju.ScriptBranchAdaptation, Title: "小说原文", ContentText: "RAW_NOVEL_SECRET_MUST_NOT_REACH_P3",
		ContentHash: "raw-secret", CreatedBy: f.ownerID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatalf("创建 P3-P5 生产链失败: %v", err)
	}
	if len(run.Stages) != 4 ||
		run.Stages[0].Key != manju.ProductionStageDecisionAnchor ||
		run.Stages[0].Status != manju.ProductionStageStatusInProgress ||
		run.Stages[1].Status != model.ManjuProductionStageStatusNotStarted ||
		run.Stages[2].Status != model.ManjuProductionStageStatusNotStarted ||
		run.Stages[3].Status != model.ManjuProductionStageStatusNotStarted {
		t.Fatalf("P0B-P5 必须按顺序建立四个阶段: %+v", run.Stages)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)

	tasks := []*model.Task{
		{ID: "production-p3-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "production-p3-image-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued},
		{ID: "production-p4-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "production-p4-video-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_video", Status: model.TaskStatusQueued},
		{ID: "production-p5-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: tasks}
	f.service.SetScriptTaskCreator(creator.create)

	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{
		Instruction: "逐镜保留台词，标清故事板格",
	}); err != nil {
		t.Fatalf("启动 P3 失败: %v", err)
	}
	p3Prompt := creator.requests[0].Prompt
	if !strings.Contains(p3Prompt, "LOCKED_ASSET_FACT") {
		t.Fatalf("P3 必须读取锁定 P2 工件: %s", p3Prompt)
	}
	if !strings.Contains(p3Prompt, "MANJU_PROJECT_STATE_PATCH_JSON") {
		t.Fatalf("P3 必须要求 V7.4.26 project_state segments 回写块: %s", p3Prompt)
	}
	for _, required := range []string{
		"【asset_refs 精确键目录】",
		"`林澈`",
		"`旧宅`",
		"必须逐字复制",
	} {
		if !strings.Contains(p3Prompt, required) {
			t.Fatalf("P3 提示词缺少精确资产键 %q: %s", required, p3Prompt)
		}
	}
	if strings.Contains(p3Prompt, "RAW_NOVEL_SECRET_MUST_NOT_REACH_P3") {
		t.Fatalf("P3 不得读取小说原文: %s", p3Prompt)
	}
	if !strings.Contains(p3Prompt, "高反差大光比") {
		t.Fatalf("P3-P5 必须收到画风卡的可投喂视觉参数: %s", p3Prompt)
	}
	if !strings.Contains(p3Prompt, "P0 画风焦段硬锁") || !strings.Contains(p3Prompt, "[21mm, 28mm]") {
		t.Fatalf("P3 必须收到 P0 画风锁的焦段硬区间: %s", p3Prompt)
	}
	p0Prompt := p3Prompt
	if boundary := strings.Index(p0Prompt, "===== V7.4.26 严格规则执行边界 ====="); boundary >= 0 {
		p0Prompt = p0Prompt[:boundary]
	}
	if strings.Contains(p0Prompt, "徐克") || strings.Contains(p0Prompt, "sd-style-009") || strings.Contains(p0Prompt, "styleCardName") || strings.Contains(p0Prompt, "styleCardId") {
		t.Fatalf("P3-P5 P0 交付参数不得携带画风卡名称或选择层 ID: %s", p0Prompt)
	}
	metadata, _ := creator.requests[0].Input["metadata"].(map[string]any)
	if metadata["manjuProductionRunId"] != run.Run.ID || metadata["manjuProductionStageKey"] != manju.ProductionStageStoryboard {
		t.Fatalf("P3 任务必须携带生产链追踪元数据: %+v", metadata)
	}
	succeedManjuTask(t, f, tasks[0].ID, validStoryboardArtifactRequiringImage())
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findProductionStage(t, detail, manju.ProductionStageStoryboard); got.Status != model.ManjuProductionStageStatusAwaitingConfirmation || got.ArtifactID == "" {
		t.Fatalf("P3 成功成果必须等待人工确认: %+v", got)
	}
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed, manju.ProductionStageRunInput{}); !errors.Is(err, manju.ErrProductionStageNotReady) {
		t.Fatalf("P3 未确认不得启动 P4，得到 %v", err)
	}

	startedImages, err := f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatalf("启动 P3 故事板图片任务失败: %v", err)
	}
	if len(startedImages.Images) != 1 || startedImages.Images[0].TaskID != tasks[1].ID {
		t.Fatalf("P3 必须为每个故事板段创建真实图片任务: %+v", startedImages.Images)
	}
	if len(creator.requests) != 2 || creator.requests[1].Type != "canvas_image" {
		t.Fatalf("P3 文本之后必须创建 canvas_image 任务: %+v", creator.requests)
	}
	imagePrompt := creator.requests[1].Prompt
	if !strings.Contains(imagePrompt, "高反差大光比") {
		t.Fatalf("P3 故事板图必须收到可投喂视觉参数: %s", imagePrompt)
	}
	if strings.Contains(imagePrompt, "徐克") || strings.Contains(imagePrompt, "sd-style-009") {
		t.Fatalf("P3 故事板图不得携带画风卡名称或选择层 ID: %s", imagePrompt)
	}
	succeedStoryboardImageTask(t, f, tasks[1].ID, "res-production-storyboard-seg-1")
	detail, err = f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Images) != 1 || detail.Images[0].Status != manju.ProductionImageStatusReady || detail.Images[0].ResourceID != "res-production-storyboard-seg-1" {
		t.Fatalf("P3 故事板图必须对账到真实资源: %+v", detail.Images)
	}
	if _, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionConfirmInput{Note: "分镜确认"}); err != nil {
		t.Fatalf("确认 P3 失败: %v", err)
	}
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed, manju.ProductionStageRunInput{}); err != nil {
		t.Fatalf("启动 P4 失败: %v", err)
	}
	p4Prompt := creator.requests[2].Prompt
	if !strings.Contains(p4Prompt, "P3_STORYBOARD_LOCKED_FACT") ||
		!strings.Contains(p4Prompt, "seedance-2.0") ||
		!strings.Contains(p4Prompt, "P3 已锁定故事板图资源映射") ||
		!strings.Contains(p4Prompt, "段1") ||
		!strings.Contains(p4Prompt, "res-production-storyboard-seg-1") {
		t.Fatalf("P4 必须读取锁定 P3、故事板图资源映射与 P0 模型锁: %s", p4Prompt)
	}
	succeedManjuTask(t, f, tasks[2].ID, validFeedArtifact())
	detail, err = f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	feedStage := findProductionStage(t, detail, manju.ProductionStageFeed)
	previsArtifact := &model.ManjuProductionArtifact{
		ID: "production-previs-ready", ProjectID: f.projectID, RunID: run.Run.ID, StageID: feedStage.ID,
		Kind: model.ManjuProductionArtifactPrevis, Version: 1, Content: validP45Artifact(),
		ContentHash: "production-previs-ready", CreatedBy: f.ownerID, CreatedAt: time.Now(),
	}
	if err := f.db.Create(previsArtifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProductionPrevis{
		ID: "production-previs-ready-row", ProjectID: f.projectID, RunID: run.Run.ID, StageID: feedStage.ID,
		ArtifactID: previsArtifact.ID, Status: model.ManjuProductionPrevisStatusReady, Version: 1,
		CreatedBy: f.ownerID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed, manju.ProductionConfirmInput{Note: "投喂稿确认"}); !errors.Is(err, manju.ErrProductionVideosNotReady) {
		t.Fatalf("P4 真实视频未就绪时不得锁定，得到 %v", err)
	}
	startedVideos, err := f.service.StartProductionVideos(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatalf("启动 P4 视频生成失败: %v", err)
	}
	if len(startedVideos.Videos) != 1 || startedVideos.Videos[0].TaskID != tasks[3].ID {
		t.Fatalf("P4 必须为每个投喂段创建真实视频任务: %+v", startedVideos.Videos)
	}
	if len(creator.requests) != 4 || creator.requests[3].Type != "canvas_video" || creator.requests[3].Operation != "manju_production_video" {
		t.Fatalf("P4 文本之后必须创建 canvas_video 任务: %+v", creator.requests)
	}
	videoRequest := creator.requests[3]
	config, _ := videoRequest.Input["config"].(map[string]any)
	if videoRequest.Model != "video-pro" || config["size"] != "9:16" || config["videoSeconds"] != "5" {
		t.Fatalf("P4 视频任务必须使用项目视频模型与 P0 画幅/段时长: %+v", videoRequest)
	}
	refs, _ := videoRequest.Input["referenceImages"].([]any)
	if len(refs) != 1 {
		t.Fatalf("P4 视频任务必须携带 P3 真实故事板图作为参考: %+v", videoRequest.Input)
	}
	ref, _ := refs[0].(map[string]any)
	if ref["storageKey"] != "resource:res-production-storyboard-seg-1" {
		t.Fatalf("P4 视频参考必须绑定真实资源而非伪造 URL: %+v", ref)
	}
	succeedProductionVideoTask(t, f, tasks[3].ID, "res-production-video-seg-1")
	detail, err = f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Videos) != 1 || detail.Videos[0].Status != manju.ProductionVideoStatusReady || detail.Videos[0].ResourceID != "res-production-video-seg-1" {
		t.Fatalf("P4 视频必须对账到真实视频资源: %+v", detail.Videos)
	}
	if _, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed, manju.ProductionConfirmInput{Note: "投喂稿确认"}); err != nil {
		t.Fatalf("确认 P4 失败: %v", err)
	}

	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageQC, manju.ProductionStageRunInput{}); err != nil {
		t.Fatalf("启动 P5 失败: %v", err)
	}
	if !strings.Contains(creator.requests[4].Prompt, "res-production-video-seg-1") ||
		!strings.Contains(creator.requests[4].Prompt, "P4 已锁定真实生成视频资源映射") {
		t.Fatalf("P5 必须读取真实生成视频资源: %s", creator.requests[4].Prompt)
	}
	succeedManjuTask(t, f, tasks[4].ID, validQCHandoffArtifact("res-production-video-seg-1"))
	detail, err = f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Run.Status != model.ManjuProductionRunStatusActive || findProductionStage(t, detail, manju.ProductionStageQC).Status != model.ManjuProductionStageStatusAwaitingConfirmation {
		t.Fatalf("P5 文本质检不得自动完成项目: %+v", detail)
	}
	detail, err = f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageQC, manju.ProductionConfirmInput{Note: "主理人验收"})
	if err != nil {
		t.Fatalf("确认 P5 失败: %v", err)
	}
	if detail.Run.Status != model.ManjuProductionRunStatusCompleted {
		t.Fatalf("P5 主理人确认后生产链才可完成: %+v", detail.Run)
	}
}

func TestP4RequiresReadyP45BeforeConfirmation(t *testing.T) {
	f := newFixture(t)
	run := prepareP4AwaitingConfirmation(t, f)

	if _, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed,
		manju.ProductionConfirmInput{Note: "未生成 P4.5"},
	); !errors.Is(err, manju.ErrProductionPrevisRequired) {
		t.Fatalf("P4.5 未通过时 P4 不得锁定，得到 %v", err)
	}

	previsTask := &model.Task{
		ID: "production-previs-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(previsTask).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{previsTask}}
	f.service.SetScriptTaskCreator(creator.create)
	started, err := f.service.GenerateProductionPrevis(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Previs == nil || started.Previs.Status != model.ManjuProductionPrevisStatusRunning {
		t.Fatalf("P4.5 必须创建独立文本任务并进入 RUNNING: %+v", started.Previs)
	}
	if len(creator.requests) != 1 ||
		creator.requests[0].Operation != "manju_previs_storyboard" ||
		creator.requests[0].Type != "canvas_text" {
		t.Fatalf("P4.5 必须使用独立文本任务: %+v", creator.requests)
	}
	if !strings.Contains(creator.requests[0].Prompt, "P4.5") ||
		!strings.Contains(creator.requests[0].Prompt, "p45-previs-storyboard.md") {
		t.Fatalf("P4.5 文本任务必须读取 V7.4.26 权威规则: %s", creator.requests[0].Prompt)
	}
	for _, required := range []string{
		"第 k 格：谁在哪：",
		"朝哪：",
		"看哪：",
		"做什么：",
		"镜头在哪：",
		"光从哪来：",
		"输出前逐格执行 P453 六问自检",
		"规则原文是唯一输出依据",
		"不得摘要、改写、重排、压缩或合并官方模板",
	} {
		if !strings.Contains(creator.requests[0].Prompt, required) {
			t.Fatalf("P4.5 提示词缺少 P453 六问硬模板 %q: %s", required, creator.requests[0].Prompt)
		}
	}

	succeedManjuTask(t, f, previsTask.ID, validP45Artifact())
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Previs == nil ||
		detail.Previs.Status != model.ManjuProductionPrevisStatusReady ||
		detail.Previs.ArtifactID == "" {
		t.Fatalf("P4.5 机检通过后必须生成独立工件: %+v", detail.Previs)
	}
	if _, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed,
		manju.ProductionConfirmInput{Note: "P4.5 已通过"},
	); !errors.Is(err, manju.ErrProductionVideosNotReady) {
		t.Fatalf("P4.5 通过后应继续卡真实视频门禁，得到 %v", err)
	}
}

func TestP45MachineCheckFailureCannotUnlockP4(t *testing.T) {
	f := newFixture(t)
	run := prepareP4AwaitingConfirmation(t, f)
	task := &model.Task{
		ID: "production-previs-invalid-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{task}}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.GenerateProductionPrevis(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, task.ID, "# P4.5\n\n## 段 1\n只有一句话。")
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Previs == nil ||
		detail.Previs.Status != model.ManjuProductionPrevisStatusFailed ||
		!strings.Contains(detail.Previs.Error, "check_p45.py") {
		t.Fatalf("P4.5 机检失败必须保留失败工件状态与原因: %+v", detail.Previs)
	}
	if _, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed,
		manju.ProductionConfirmInput{Note: "尝试绕过 P4.5"},
	); !errors.Is(err, manju.ErrProductionPrevisRequired) {
		t.Fatalf("P4.5 机检失败时 P4 仍不得锁定，得到 %v", err)
	}
}

func TestLegacyP3WithoutAssetRefsBlocksP4AndRoutesToP3(t *testing.T) {
	f := newFixture(t)
	run := prepareP4AwaitingConfirmation(t, f)
	f.service.SetSkillValidationEnabledForTest(true)
	var snapshot model.ManjuProjectStateSnapshot
	if err := f.db.First(&snapshot, "id = ?", "state-production-p45").Error; err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(snapshot.StateJSON), &state); err != nil {
		t.Fatal(err)
	}
	segments, _ := state["segments"].([]any)
	for _, raw := range segments {
		segment, _ := raw.(map[string]any)
		delete(segment, "asset_refs")
	}
	legacyState, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProjectStateSnapshot{}).
		Where("id = ?", snapshot.ID).
		Update("state_json", string(legacyState)).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("run_id = ? AND key = ?", run.Run.ID, manju.ProductionStageFeed).
		Updates(map[string]any{
			"status":      model.ManjuProductionStageStatusInProgress,
			"task_id":     "",
			"artifact_id": "",
			"error":       "",
		}).Error; err != nil {
		t.Fatal(err)
	}

	_, err = f.service.RunProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed,
		manju.ProductionStageRunInput{},
	)
	if err == nil || !strings.Contains(err.Error(), "P3_CONTRACT_UPGRADE_REQUIRED") {
		t.Fatalf("旧 P3 缺 asset_refs 时必须路由回 P3: %v", err)
	}
	detail, detailErr := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if detailErr != nil {
		t.Fatal(detailErr)
	}
	stage := findProductionStage(t, detail, manju.ProductionStageFeed)
	if stage.Status != model.ManjuProductionStageStatusBlocked ||
		!strings.Contains(stage.Error, "重新生成 P3") {
		t.Fatalf("P4 必须阻断并提示只重生成 P3: %+v", stage)
	}
}

func TestGenerateProductionPrevisUsesNewOperationKeyPerAttempt(t *testing.T) {
	f := newFixture(t)
	run := prepareP4AwaitingConfirmation(t, f)
	firstTask := &model.Task{
		ID: "previs-attempt-1", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	secondTask := &model.Task{
		ID: "previs-attempt-2", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(&[]model.Task{*firstTask, *secondTask}).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{firstTask, secondTask}}
	f.service.SetScriptTaskCreator(creator.create)

	if _, err := f.service.GenerateProductionPrevis(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionPrevis{}).
		Where("run_id = ?", run.Run.ID).
		Updates(map[string]any{
			"status":  model.ManjuProductionPrevisStatusFailed,
			"error":   "首次生成未通过",
			"task_id": "",
		}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.GenerateProductionPrevis(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if len(creator.requests) != 2 {
		t.Fatalf("expected two previs attempts, got %d", len(creator.requests))
	}
	operationID := func(request localtask.CreateRequest) string {
		metadata, _ := request.Input["metadata"].(map[string]any)
		value, _ := metadata["clientOperationId"].(string)
		return value
	}
	firstID := operationID(creator.requests[0])
	secondID := operationID(creator.requests[1])
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatalf("previs retries must use different operation keys: first=%q second=%q", firstID, secondID)
	}
	if !strings.Contains(firstID, ":v1") || !strings.Contains(secondID, ":v2") {
		t.Fatalf("previs retry versions must increment: first=%q second=%q", firstID, secondID)
	}
}

func prepareP4AwaitingConfirmation(t *testing.T, f fixture) *manju.ProductionRunDetail {
	t.Helper()
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	tasks := []*model.Task{
		{ID: "production-p3-for-p45", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "production-p4-for-p45", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: tasks}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{},
	); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[0].ID, validStoryboardArtifact())
	if _, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard,
		manju.ProductionConfirmInput{Note: "P3 确认"},
	); err != nil {
		t.Fatal(err)
	}
	projectDetail, err := f.service.ProjectDetail(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	stateJSON := `{"schema_version":"7.4.23","locks":{"asset_mode":"M1","model":"2.0","aspect":"9:16"},"characters":{"林澈":{"fig_no":1,"voice_lock":"清亮青年声"}},"scenes":{"旧宅":{"fig_no":2,"coords":"门在右侧","set_states":{"default":"清晨"},"media_layer":"薄尘","lighting":"清晨冷光"}},"props":{},"segments":[{"seg_id":"段1","scene":"旧宅","asset_refs":{"scenes":["旧宅"],"characters":["林澈"],"props":[]},"shots":[{"no":1,"start":0,"end":5,"spec":"中景 · 35mm · 侧45度 · 固定","action":"林澈推门","dialogue":"无"}],"standing":["林澈在门左侧"]}]}`
	if err := f.db.Create(&model.ManjuProjectStateSnapshot{
		ID: "state-production-p45", ProjectID: f.projectID, LockRevision: projectDetail.Lock.Revision,
		StageKey: "production:director_storyboard", SchemaVersion: "7.4.23",
		StateJSON: stateJSON, ContentHash: "state-production-p45", CreatedBy: f.ownerID,
		CreatedAt: time.Now().Add(time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunProductionStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed,
		manju.ProductionStageRunInput{},
	); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[1].ID, validFeedArtifact())
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findProductionStage(t, detail, manju.ProductionStageFeed); got.Status != model.ManjuProductionStageStatusAwaitingConfirmation {
		t.Fatalf("P4 应等待确认: %+v", got)
	}
	return detail
}

func validP45Artifact() string {
	return `# 线上漫剧项目 · 预演故事板

> 以下提示词用于在图像平台生成预演图，供人工核对空间、站位、人物一致性；不是投喂素材，不参与视频生成。

## 段 1 · 林澈在旧宅门口停步
**本段要验什么**：单主体站位与门口空间关系
**风险提示**：人物与门框左右关系容易互换

### 故事板提示词（3 格）
画一张分镜故事板，3 格，横向排列，9:16，写实电影感，画面中没有任何文字、数字、水印或边框标注。
统一人物设定：林澈：26岁，短发，黑色短打，瘦高。
场景：旧宅木门在画面右侧，门内冷白灯，门外清晨冷光。
第 1 格：林澈站在门外石阶上，面朝木门，目光落在门把上，右手抬起准备推门，镜头在台阶侧前方平视中景，清晨冷光从左上方照下来。
第 2 格：林澈向前半步，身体朝向门内，眼睛看向门缝，右手推开木门，镜头在门左侧平视中近景，门内冷白灯光从右侧照到脸上。
第 3 格：林澈停在门框内侧，面朝屋内，视线看向走廊深处，左手扶住门框，镜头在屋内侧后方平视中景，冷白灯从头顶照下。

### 看图时先看这三点
1. 林澈是否始终单人入画
2. 左右门框关系是否连续
3. 服装发型是否前后一致

预演故事板：段1 ✅
`
}

func prepareCompletedProductionAsset(t *testing.T, f fixture, mode string) string {
	t.Helper()
	scriptRunID := prepareCompletedScript(t, f, mode)
	project, err := f.service.ProjectDetail(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	assetRun := &model.ManjuAssetRun{
		ID: "asset-completed-" + mode, ProjectID: f.projectID, ScriptRunID: scriptRunID,
		AssetMode: mode, Status: model.ManjuAssetRunStatusCompleted, CurrentStage: "",
		LockRevision: project.Lock.Revision, CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}
	if err := f.db.Create(assetRun).Error; err != nil {
		t.Fatal(err)
	}
	stage := &model.ManjuAssetStage{
		ID: "asset-compile-" + mode, RunID: assetRun.ID, Key: manju.AssetStageCompile,
		Title: "资产交付编译", Position: 3, Status: model.ManjuAssetStageStatusLocked,
		Version: 1, ArtifactID: "asset-artifact-" + mode, CreatedAt: now, UpdatedAt: now,
	}
	if err := f.db.Create(stage).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &model.ManjuAssetArtifact{
		ID: stage.ArtifactID, ProjectID: f.projectID, RunID: assetRun.ID, StageID: stage.ID,
		Kind: model.ManjuAssetArtifactM1, Version: 1,
		Content: "LOCKED_ASSET_FACT\n图1 = CHR-林澈\nSCN-旧宅。", ContentHash: "locked-asset",
		CreatedBy: f.ownerID, CreatedAt: now,
	}
	if err := f.db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	stateJSON, err := json.Marshal(map[string]any{
		"schema_version": "7.4.23",
		"locks": map[string]any{
			"asset_mode": mode,
			"model":      "2.0",
			"aspect":     "9:16",
		},
		"characters": map[string]any{
			"林澈": map[string]any{
				"id": "CHR-林澈", "fig_no": 1,
				"voice_lock": "清亮青年声",
			},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{
				"id": "SCN-旧宅", "fig_no": 2,
				"coords":      "木门在画面右侧，门内为纵深走廊",
				"set_states":  map[string]any{"default": "清晨薄雾"},
				"media_layer": "冷白薄尘",
				"lighting":    "清晨冷光从门外左上方照入",
			},
		},
		"props":    map[string]any{},
		"segments": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectStateSnapshot{
		ID: "state-asset-completed-" + mode, ProjectID: f.projectID,
		LockRevision: project.Lock.Revision, StageKey: "asset:asset_compile",
		ArtifactID: artifact.ID, SchemaVersion: "7.4.23",
		StateJSON: string(stateJSON), ContentHash: "locked-asset-state",
		CreatedBy: f.ownerID, CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return assetRun.ID
}

func findProductionStage(t *testing.T, detail *manju.ProductionRunDetail, key string) manju.ProductionStageView {
	t.Helper()
	for _, stage := range detail.Stages {
		if stage.Key == key {
			return stage
		}
	}
	t.Fatalf("缺少 P3-P5 阶段 %s: %+v", key, detail.Stages)
	return manju.ProductionStageView{}
}

func lockProductionP0BForTest(t *testing.T, f fixture, runID string) *manju.ProductionRunDetail {
	t.Helper()
	var stage model.ManjuProductionStage
	if err := f.db.First(&stage, "run_id = ? AND key = ?", runID, manju.ProductionStageDecisionAnchor).Error; err != nil {
		t.Fatal(err)
	}
	content := validP0BArtifact()
	sum := sha256.Sum256([]byte(content))
	artifact := &model.ManjuProductionArtifact{
		ID: "p0b-" + runID[:8], ProjectID: f.projectID, RunID: runID, StageID: stage.ID,
		Kind: model.ManjuProductionArtifactEmotionAnchor, Version: 1, Content: content,
		ContentHash: hex.EncodeToString(sum[:]), CreatedBy: f.ownerID, CreatedAt: time.Now(),
	}
	if err := f.db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", stage.ID).Updates(map[string]any{
		"status": model.ManjuProductionStageStatusAwaitingConfirmation, "artifact_id": artifact.ID,
		"version": 1, "completed_at": now, "error": "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	detail, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, runID,
		manju.ProductionStageDecisionAnchor,
		manju.ProductionConfirmInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return detail
}

func validP0BArtifact() string {
	return `# P0B 决策锚点
## 情绪锚点表

【大分镜 #B01 · 情绪锚点表】

| 节拍 # | 情绪关键词 | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 |
|---|---|---|---|---|---|
| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 | 长焦+大光圈+空旷留白 | — |
| 节拍 2 | 察觉异样 | 女儿瞥见纸角，手指顿住 | E11 | 微距标点+静切动 | — |
| 节拍 3 | 强装平静 | 母亲端起杯子继续夹菜 | E08 | 过肩+长焦+手持微动 | — |

## 构图雷区白名单

- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。
- A类：其余项目无解锁；未命中白名单时按默认规避。
- B类 · 永不解锁：物体穿身、角色服装突变、杂乱前景刺眼物、比例失调、重复元素无重点、空鸡蛋构图、道具状态突变。
`
}

func validStoryboardArtifact() string {
	return `# P3 分镜与故事板
P3_STORYBOARD_LOCKED_FACT

## 角色表演档案
角色表演档案 · 林澈。克制的青年，肩背常紧。声音偏低，紧张时更轻。

## 逐场节拍表
| 场 | 目标 | 障碍 | 战术 | 转折 |
| --- | --- | --- | --- | --- |
| 场1 | 进门 | 门锁 | 推门 | 门开 |

## 导演稿
文字平面图：门在画面右侧，林澈从左侧入画，轴线保持左林澈右门。

## 分镜总表
| 段号 | 镜号 | 时长 | 景别 | 机位 | 运镜 | 画面内容 | 台词 | 音效 | 资产引用 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 段1 | 01 | 5s | 中景 | 侧45度 | 固定 | 林澈推开旧宅门 | 无 | 木门吱呀 | CHR-林澈 |

## 故事板表
### 第1段｜旧宅门
| 段 | 格 | 动作相位 | 镜头信息 | 站位+视线 | 资产引用 |
| --- | --- | --- | --- | --- | --- |
| 段1 | 1 | 起始 | 中景侧45度 | 林澈在左前景看门 | CHR-林澈 |
| 段1 | 2 | 过程 | 手部近景 | 右手推门 | CHR-林澈 |
| 段1 | 3 | 反应 | 中近景 | 视线看向门内 | CHR-林澈 |
| 段1 | 4 | 出口 | 中景 | 人物停在门内 | CHR-林澈 |

## 台词精简建议清单
本稿全量承接上游台词，无精简建议。

## P3 交付清单
| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |
| --- | --- | --- | --- |
| 段1 | 覆盖：场1 ✅ 四宫格 | ⛔豁免（单镜过场段） | 理由：单镜过场，动作相位已在分镜总表完整列出。 |

<!-- MANJU_PROJECT_STATE_PATCH_JSON
{
  "segments": [{
    "seg_id": "段1",
    "scene": "旧宅",
    "asset_refs": {
      "scenes": ["旧宅"],
      "characters": ["林澈"],
      "props": []
    },
    "shots": [{
      "no": 1,
      "start": 0,
      "end": 5,
      "spec": "中景 · 35mm · 侧45度 · 固定",
      "action": "林澈推门",
      "sound": "木门吱呀",
      "dialogue": "无"
    }]
  }]
}
-->`
}

func validStoryboardArtifactRequiringImage() string {
	return strings.Replace(
		validStoryboardArtifact(),
		"| 段1 | 覆盖：场1 ✅ 四宫格 | ⛔豁免（单镜过场段） | 理由：单镜过场，动作相位已在分镜总表完整列出。 |",
		"| 段1 | 覆盖：场1 ✅ 四宫格 | 4格(2×2 四宫格) | 无豁免；等待已就绪故事板图。 |",
		1,
	)
}

func validMixedP3StoryboardArtifact() string {
	return `# P3 分镜与故事板

## 角色表演档案
角色表演档案 · 林澈。克制的青年，肩背常紧，开口前会先看门缝。

## 逐场节拍表
| 场 | 目标 | 障碍 | 战术 | 转折 |
| --- | --- | --- | --- | --- |
| 场1 | 穿过雨幕 | 门锁 | 推门 | 门开 |
| 场2 | 观察屋内 | 黑暗 | 停顿 | 灯亮 |

## 导演稿
文字平面图：门在画面右侧，林澈从左侧入画，轴线保持左林澈右门。

## 分镜总表
| 段号 | 镜号 | 时长 | 景别 | 机位 | 运镜 | 画面内容 | 台词 | 音效 | 资产引用 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 段1 | 01 | 2s | 中景 | 侧45度 | 固定 | 林澈跨过门槛 | 无 | 雨声 | CHR-林澈 |
| 段2 | 01 | 5s | 中景 | 侧45度 | 固定 | 林澈停在门内 | 无 | 木门吱呀 | CHR-林澈 |

## 故事板表
### 第1段｜过门
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 | 面板边界备注 |
|---|---|---|---|---|---|---|
| 段1 | 段1-P1 | 起始 | 中景 · 侧45度 | 林澈在门外 | CHR-林澈 | 入口帧 |
| 段1 | 段1-P2 | 过程 | 中景 · 固定 | 林澈抬脚 | CHR-林澈 | 动作连续 |
| 段1 | 段1-P3 | 反应 | 近景 · 固定 | 视线看向屋内 | CHR-林澈 | 不剪 |
| 段1 | 段1-P4 | 出口 | 中景 · 固定 | 林澈跨过门槛 | CHR-林澈 | 出口帧 |

### 第2段｜停步
| 段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 | 面板边界备注 |
|---|---|---|---|---|---|---|
| 段2 | 段2-P1 | 起始 | 中景 · 侧45度 | 林澈在门内 | CHR-林澈 | 入口帧 |
| 段2 | 段2-P2 | 过程 | 近景 · 固定 | 右手松开门把 | CHR-林澈 | 动作连续 |
| 段2 | 段2-P3 | 反应 | 近景 · 固定 | 视线扫过屋内 | CHR-林澈 | 不剪 |
| 段2 | 段2-P4 | 出口 | 中景 · 固定 | 林澈站定 | CHR-林澈 | 出口帧 |

## 台词精简建议清单
本稿全量承接上游台词，无精简建议。

## P3 交付清单
| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |
|---|---|---|---|
| 段1 | 覆盖：场1 ✅ 四宫格 | ⛔豁免（单镜过场段） | 理由：本段只有一个过门动作，动作相位已完整列入分镜总表。 |
| 段2 | 覆盖：场2 ✅ 四宫格 | 4格(2×2 四宫格) | 无豁免；必须生成段2故事板图。 |

<!-- MANJU_PROJECT_STATE_PATCH_JSON
{
  "segments": [
    {
      "seg_id": "段1",
      "scene": "旧宅",
      "asset_refs": {"scenes": ["旧宅"], "characters": ["林澈"], "props": []},
      "shots": [{"no": 1, "start": 0, "end": 2, "spec": "中景 · 35mm · 侧45度 · 固定", "action": "林澈跨门", "sound": "雨声", "dialogue": "无"}]
    },
    {
      "seg_id": "段2",
      "scene": "旧宅",
      "asset_refs": {"scenes": ["旧宅"], "characters": ["林澈"], "props": []},
      "shots": [{"no": 1, "start": 0, "end": 5, "spec": "中景 · 35mm · 侧45度 · 固定", "action": "林澈停步", "sound": "木门吱呀", "dialogue": "无"}]
    }
  ]
}
-->`
}

func prepareReadyP3ImageForResourceRecheck(t *testing.T) (fixture, string, string) {
	t.Helper()
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	p3Task := &model.Task{ID: "production-p3-resource-recheck", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	imageTask := &model.Task{ID: "production-image-resource-recheck", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued}
	if err := f.db.Create(&[]model.Task{*p3Task, *imageTask}).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{p3Task, imageTask}}).create)
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, p3Task.ID, validStoryboardArtifactRequiringImage())
	started, err := f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(started.Images) != 1 {
		t.Fatalf("expected one ready-image candidate: %+v", started.Images)
	}
	resourceID := "resource-p3-recheck"
	succeedStoryboardImageTask(t, f, imageTask.ID, resourceID)
	return f, run.Run.ID, resourceID
}

func validFeedArtifact() string {
	return `> 素材形态：M1（编号占位 · 图号见接线单）
# P4 投喂封装
P4_FEED_LOCKED_FACT

## 段1
【画面主体】将 图1 中的档案修复师定义为 林澈。林澈右手推门，肩部收紧，视线落入门内。中景，侧45度，固定镜头。
【场景资产】@SCN-旧宅
【声音】木门吱呀声。
【台词】无
【时长】5秒
【画幅】9:16
【强制禁止项】不得换脸、不得新增人物。

## 素材接线单
| 图号 | 类型 | 用途 | 你该挂哪张 | 不控制什么 |
| --- | --- | --- | --- | --- |
| 图1 | CHR | 锁定林澈外观 | LOCKED_ASSET_FACT 对应角色图 | 不控制动作和场景 |

## 台词完整性
上游 0 句，本稿承接 0 句，缺失 0 句。

## 六道闸门
节奏、台词时长、字数、可投喂语义、图片数量、编号与素材形态一致性均通过。
`
}

func validQCHandoffArtifact(videoResourceID string) string {
	return strings.ReplaceAll(`# P5 质检与交付
## 质检边界
已核对真实生成视频资源存在，不把文本检查冒充看片；画面与声音质量由主理人看片验收。
## 连续性台账
| 项目 | 入口 | 变化 | 出口 | 结论 |
| --- | --- | --- | --- | --- |
| 人物 | 林澈在门外 | 推门 | 林澈在门内 | WARN 需在成片复核 |
## 视频交付核对
| 段 | resourceId | 资源状态 | 看片结论 |
| --- | --- | --- | --- |
| 段1 | VIDEO_RESOURCE_ID | READY | 待主理人看片 |
## 剪辑与段缝门
Seedance 2.0 单段 5 秒，未超过 15 秒；D 锁未触发。
## 声音设计
对白 0 句，环境音 1 层，未生成 BGM。
## 机检结果
FAIL 0 / WARN 1 / PASS 0
## 最终裁决
WARN：文本层可交付，成片与逐帧一致性仍需生成后复核。`, "VIDEO_RESOURCE_ID", videoResourceID)
}
func TestP3CannotLockWhileStoryboardImageBatchIsRunning(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	p3Task := &model.Task{ID: "production-p3-with-images", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	imageTask := &model.Task{ID: "production-storyboard-image-1", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued}
	if err := f.db.Create(&[]model.Task{*p3Task, *imageTask}).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{p3Task, imageTask}}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, p3Task.ID, validStoryboardArtifactRequiringImage())

	started, err := f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatalf("启动故事板图片批次失败: %v", err)
	}
	if len(started.Images) != 1 || started.Images[0].Status != manju.ProductionImageStatusRunning || started.Images[0].TaskID != imageTask.ID {
		t.Fatalf("故事板图片必须创建真实任务并按段跟踪: %+v", started.Images)
	}
	if len(creator.requests) != 2 {
		t.Fatalf("应创建 P3 文本与 1 个故事板图片任务，得到 %d", len(creator.requests))
	}
	imageRequest := creator.requests[1]
	if imageRequest.Type != "canvas_image" || imageRequest.Model != "image-pro" || !strings.Contains(imageRequest.Prompt, "起始") || !strings.Contains(imageRequest.Prompt, "出口") {
		t.Fatalf("故事板图片任务必须使用图片模型并承接完整动作相位: %+v", imageRequest)
	}
	metadata, _ := imageRequest.Input["metadata"].(map[string]any)
	if metadata["manjuProductionRunId"] != run.Run.ID || metadata["manjuStoryboardImageJobId"] != started.Images[0].ID {
		t.Fatalf("故事板图片任务必须携带生产链和图片批次追踪元数据: %+v", metadata)
	}

	_, err = f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionConfirmInput{})
	if !errors.Is(err, manju.ErrStoryboardImagesNotReady) {
		t.Fatalf("P3 must fail closed while its storyboard image batch is partial, got %v", err)
	}

	succeedStoryboardImageTask(t, f, imageTask.ID, "res-storyboard-seg-1")
	confirmed, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionConfirmInput{})
	if err != nil {
		t.Fatalf("P3 must lock after every storyboard image is ready: %v", err)
	}
	if findProductionStage(t, confirmed, manju.ProductionStageStoryboard).Status != model.ManjuProductionStageStatusLocked {
		t.Fatalf("P3 未按预期锁定: %+v", confirmed.Stages)
	}

	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Images) != 1 || detail.Images[0].Status != manju.ProductionImageStatusReady || detail.Images[0].ResourceID != "res-storyboard-seg-1" {
		t.Fatalf("图片任务成功后必须记录真实资源 ID: %+v", detail.Images)
	}

}

func TestP3MixedExemptionCreatesOnlyRequiredImageTaskAndLocks(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	p3Task := &model.Task{ID: "production-p3-mixed-exemption", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	imageTask := &model.Task{ID: "production-image-seg-2", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued}
	if err := f.db.Create(&[]model.Task{*p3Task, *imageTask}).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{p3Task, imageTask}}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, p3Task.ID, validMixedP3StoryboardArtifact())

	started, err := f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(started.Images) != 1 || started.Images[0].SegmentCode != "段2" {
		t.Fatalf("only the non-exempt segment may receive an image task: %+v", started.Images)
	}
	if len(creator.requests) != 2 {
		t.Fatalf("mixed exemption must create exactly one image request, got %+v", creator.requests)
	}

	succeedStoryboardImageTask(t, f, imageTask.ID, "resource-mixed-seg-2")
	confirmed, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionConfirmInput{})
	if err != nil {
		t.Fatalf("mixed exemption with the required image ready must lock: %v", err)
	}
	if findProductionStage(t, confirmed, manju.ProductionStageStoryboard).Status != model.ManjuProductionStageStatusLocked {
		t.Fatalf("P3 must lock after the required image is ready: %+v", confirmed.Stages)
	}
}

func TestP3ConfirmRevalidatesStoryboardImageResource(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(t *testing.T, f fixture, resourceID string)
	}{
		{
			name: "deleted resource",
			mutate: func(t *testing.T, f fixture, resourceID string) {
				t.Helper()
				if err := f.db.Unscoped().Delete(&model.Resource{}, "id = ?", resourceID).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "foreign resource",
			mutate: func(t *testing.T, f fixture, resourceID string) {
				t.Helper()
				if err := f.db.Model(&model.Resource{}).Where("id = ?", resourceID).Update("user_id", f.producer).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "wrong resource kind",
			mutate: func(t *testing.T, f fixture, resourceID string) {
				t.Helper()
				if err := f.db.Model(&model.Resource{}).Where("id = ?", resourceID).Update("kind", "video").Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "resource not ready",
			mutate: func(t *testing.T, f fixture, resourceID string) {
				t.Helper()
				if err := f.db.Model(&model.Resource{}).Where("id = ?", resourceID).Update("status", model.ResourceStatusPending).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, runID, resourceID := prepareReadyP3ImageForResourceRecheck(t)
			tc.mutate(t, f, resourceID)

			_, err := f.service.ConfirmProductionStage(
				f.ownerID,
				f.projectID,
				runID,
				manju.ProductionStageStoryboard,
				manju.ProductionConfirmInput{},
			)
			if !errors.Is(err, manju.ErrStoryboardImagesNotReady) {
				t.Fatalf("P3 must revalidate the live resource and fail closed, got %v", err)
			}
		})
	}
}

func TestP3CanvasWorkflowMaterializesReadyStoryboardImagesForConfirmation(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	p3Task := &model.Task{ID: "p3-canvas-workflow-text", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	if err := f.db.Create(p3Task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{p3Task}}).create)
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, p3Task.ID, validStoryboardArtifactRequiringImage())
	if _, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}

	pkg, err := f.service.CanvasPackage(f.ownerID, f.projectID, manju.CanvasPackageStageP3Images, run.Run.ID)
	if err != nil {
		t.Fatalf("CanvasPackage failed after P3 reconciliation: %v", err)
	}
	nodes := make([]map[string]any, 0, len(pkg.Tasks))
	var storyboardTask manju.CanvasPackageTask
	var storyboardResourceID string
	for _, task := range pkg.Tasks {
		resourceID := "canvas-workflow-resource-" + task.ID
		if task.Group == "storyboard" {
			storyboardTask = task
			storyboardResourceID = resourceID
		}
		if err := f.db.Create(&model.Resource{
			ID: resourceID, UserID: f.ownerID, Kind: task.Kind, Status: model.ResourceStatusReady,
			MimeType: "image/png", PublicURL: "/api/resources/" + resourceID + "/file",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}).Error; err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, map[string]any{
			"id":    "canvas-workflow-node-" + task.ID,
			"type":  task.Kind,
			"title": task.Title,
			"metadata": map[string]any{
				"manjuStage":  manju.CanvasPackageStageP3Images,
				"manjuTaskId": task.ID,
				"storageKey":  "resource:" + resourceID,
			},
		})
	}
	if storyboardTask.ID == "" {
		t.Fatal("P3 canvas package must contain a storyboard task")
	}
	payload, err := json.Marshal(map[string]any{"nodes": nodes})
	if err != nil {
		t.Fatal(err)
	}
	canvas := &model.CanvasProject{
		ID: "canvas-p3-workflow", UserID: f.ownerID, ProjectID: f.projectID,
		Title: "P3 线上画布工作流", PayloadJSON: string(payload), Revision: 1,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := f.db.Create(canvas).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.SaveCanvasImport(f.ownerID, f.projectID, manju.CanvasImportInput{
		Stage: manju.CanvasPackageStageP3Images, ProductionRunID: run.Run.ID,
		CanvasID: canvas.ID, PackageHash: pkg.Hash,
	}); err != nil {
		t.Fatal(err)
	}
	before, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Images) != 0 {
		t.Fatalf("test must begin without hidden production image rows: %+v", before.Images)
	}

	confirmed, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionConfirmInput{})
	if err != nil {
		t.Fatalf("a completed canvas workflow must satisfy P3 confirmation: %v", err)
	}
	storyboardStage := findProductionStage(t, confirmed, manju.ProductionStageStoryboard)
	if storyboardStage.Status != model.ManjuProductionStageStatusLocked {
		t.Fatalf("P3 must lock from the canvas workflow: %+v", confirmed.Stages)
	}
	if len(confirmed.Images) != 1 {
		t.Fatalf("canvas workflow must materialize one storyboard production image: %+v", confirmed.Images)
	}
	image := confirmed.Images[0]
	if image.SegmentCode != storyboardTask.Code || image.Status != manju.ProductionImageStatusReady ||
		image.ResourceID != storyboardResourceID || image.TaskID != storyboardTask.ID ||
		image.ArtifactID != storyboardStage.ArtifactID {
		t.Fatalf("materialized canvas image lost task/resource lineage: %+v", image)
	}
}

func TestP3RegenerationSupersedesStaleStoryboardImagesWithoutDeletingMedia(t *testing.T) {
	f, runID, oldResourceID := prepareReadyP3ImageForResourceRecheck(t)
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, runID)
	if err != nil {
		t.Fatal(err)
	}
	oldImage := detail.Images[0]

	newTask := &model.Task{
		ID: "p3-regenerated-exempt-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(newTask).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{newTask}}).create)
	if _, err := f.service.RegenerateProductionArtifact(
		f.ownerID, f.projectID, runID, manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{Instruction: "改为单镜过场豁免"},
	); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, newTask.ID, validStoryboardArtifact())

	confirmed, err := f.service.ConfirmProductionStage(
		f.ownerID, f.projectID, runID, manju.ProductionStageStoryboard,
		manju.ProductionConfirmInput{},
	)
	if err != nil {
		t.Fatalf("regenerated P3 exemption must not deadlock on stale image rows: %v", err)
	}
	if findProductionStage(t, confirmed, manju.ProductionStageStoryboard).Status != model.ManjuProductionStageStatusLocked {
		t.Fatalf("P3 must lock after stale image supersession: %+v", confirmed.Stages)
	}
	if len(confirmed.Images) != 1 || confirmed.Images[0].Status != "SUPERSEDED" {
		t.Fatalf("stale image row must be superseded, not deleted: %+v", confirmed.Images)
	}
	if confirmed.Images[0].ID != oldImage.ID || confirmed.Images[0].ResourceID != oldResourceID {
		t.Fatalf("supersession must preserve image history and media lineage: before=%+v after=%+v", oldImage, confirmed.Images[0])
	}
	var resourceCount int64
	if err := f.db.Model(&model.Resource{}).Where("id = ?", oldResourceID).Count(&resourceCount).Error; err != nil {
		t.Fatal(err)
	}
	if resourceCount != 1 {
		t.Fatalf("supersession must not delete the historical resource: %s", oldResourceID)
	}
}

func succeedStoryboardImageTask(t *testing.T, f fixture, taskID, resourceID string) {
	t.Helper()
	if err := f.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成",
		"result_json": `{"mode":"image","images":[{"resourceId":"` + resourceID + `"}]}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.Resource{
		ID: resourceID, UserID: f.ownerID, Kind: "image", Status: model.ResourceStatusReady,
		MimeType: "image/png", PublicURL: "/api/resources/" + resourceID + "/file", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	payload, err := localtask.EncodeOutputPayload(localtask.CanonicalOutput{
		OutputIndex: 0, MediaType: "image", ResourceID: resourceID,
		MaterializedAssetID: localtask.MaterializedAssetID(taskID, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.Result{
		ID: localtask.OutputResultID(taskID, 0), UserID: f.ownerID, TaskID: taskID,
		Kind: localtask.ResultKindGenerationOutput, Payload: payload, CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func succeedProductionVideoTask(t *testing.T, f fixture, taskID, resourceID string) {
	t.Helper()
	if err := f.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成",
		"result_json": `{"mode":"video","video":{"resourceId":"` + resourceID + `"}}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.Resource{
		ID: resourceID, UserID: f.ownerID, Kind: "video", Status: model.ResourceStatusReady,
		MimeType: "video/mp4", PublicURL: "/api/resources/" + resourceID + "/file", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	payload, err := localtask.EncodeOutputPayload(localtask.CanonicalOutput{
		OutputIndex: 0, MediaType: "video", ResourceID: resourceID,
		MaterializedAssetID: localtask.MaterializedAssetID(taskID, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.Result{
		ID: localtask.OutputResultID(taskID, 0), UserID: f.ownerID, TaskID: taskID,
		Kind: localtask.ResultKindGenerationOutput, Payload: payload, CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestStoryboardImageReconciliationRejectsTaskOwnedByAnotherUser(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	p3Task := &model.Task{ID: "production-p3-foreign-image-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	imageTask := &model.Task{ID: "production-image-owner-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued}
	foreignTask := &model.Task{ID: "production-image-foreign-task", UserID: f.producer, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusSucceeded}
	if err := f.db.Create(&[]model.Task{*p3Task, *imageTask, *foreignTask}).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{p3Task, imageTask}}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, p3Task.ID, validStoryboardArtifactRequiringImage())
	started, err := f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(started.Images) != 1 {
		t.Fatalf("缺少故事板图片任务: %+v", started.Images)
	}
	if err := f.db.Model(&model.ManjuProductionImage{}).Where("run_id = ?", run.Run.ID).Update("task_id", foreignTask.ID).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Images) != 1 || detail.Images[0].Status != manju.ProductionImageStatusFailed {
		t.Fatalf("其他账号的任务不得作为当前故事板图交付: %+v", detail.Images)
	}
	if !strings.Contains(detail.Images[0].Error, "图片任务不存在") {
		t.Fatalf("图片任务归属校验错误不明确: %+v", detail.Images[0])
	}
}
func TestStoryboardImageBatchRequiresBoundImageModel(t *testing.T) {

	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	p3Task := &model.Task{ID: "production-p3-no-image-model", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	if err := f.db.Create(p3Task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{p3Task}}).create)
	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, p3Task.ID, validStoryboardArtifactRequiringImage())
	if err := f.db.Model(&model.ManjuProjectModelBinding{}).Where("project_id = ?", f.projectID).Updates(map[string]any{
		"image_channel_id": "", "image_model_key": "",
	}).Error; err != nil {
		t.Fatal(err)
	}

	_, err = f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if !errors.Is(err, manju.ErrInvalidModelBinding) {
		t.Fatalf("未绑定图片模型时不得创建故事板图片任务，得到 %v", err)
	}
}
