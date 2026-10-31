package manju_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
	localtask "infinite-canvas/backend/internal/task"
)

type recordingScriptTaskCreator struct {
	userID  string
	request localtask.CreateRequest
	task    *model.Task
	err     error
}

func (f *recordingScriptTaskCreator) create(userID string, request localtask.CreateRequest) (*model.Task, error) {
	f.userID = userID
	f.request = request
	return f.task, f.err
}

func TestCreateScriptRunRequiresLockedP0(t *testing.T) {
	f := newFixture(t)

	_, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise,
	})

	if !errors.Is(err, manju.ErrP0Required) {
		t.Fatalf("未锁定 P0 必须阻断 P1，得到 %v", err)
	}
}

func TestFinishedScriptPolishingRequiresAdaptationInput(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	lock.ConfirmedLocks = fullConfirmedLocks(lock)
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}

	source, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title:              "待润色剧本",
		AuthorizationScope: "已获原作者润色授权",
		Content:            "第一章 雨夜\n林澈：别开门。",
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode:             manju.ScriptModeStepwise,
		InputKind:        manju.ScriptInputFinished,
		SourceDocumentID: source.Document.ID,
		ChapterIDs:       []string{source.Chapters[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.Run.Branch != manju.ScriptBranchPolish {
		t.Fatalf("finished-script adaptation must enter polish branch: %+v", run.Run)
	}
	if run.Run.InputKind != manju.ScriptInputFinished {
		t.Fatalf("finished-script input kind must persist: %+v", run.Run)
	}
	var stored model.ManjuScriptRun
	if err := f.db.First(&stored, "id = ?", run.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.InputKind != manju.ScriptInputFinished {
		t.Fatalf("stored input kind = %q, want %q", stored.InputKind, manju.ScriptInputFinished)
	}
	assertSourceManifestRouting(t, f, run.Run.ID, manju.ScriptInputFinished, "")
}

func TestTrendSourceRequiresExplicitResolvedRoute(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceTrend
	lock.ConfirmedLocks = fullConfirmedLocks(lock)
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}

	_, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode:      manju.ScriptModeStepwise,
		InputKind: manju.ScriptInputOutline,
	})
	if !errors.Is(err, manju.ErrTrendResolutionRequired) {
		t.Fatalf("trend must require a selected route, got %v", err)
	}
}

func TestTrendSourceUsesExplicitResolvedRoute(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		resolution     string
		expectedBranch string
	}{
		{name: "original", resolution: manju.TrendResolutionOriginal, expectedBranch: manju.ScriptBranchOriginal},
		{name: "benchmark", resolution: manju.TrendResolutionBenchmark, expectedBranch: manju.ScriptBranchBenchmark},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			lock := validLock()
			lock.Source = manju.SourceTrend
			if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
				t.Fatal(err)
			}

			run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
				Mode:            manju.ScriptModeStepwise,
				InputKind:       manju.ScriptInputOutline,
				TrendResolution: testCase.resolution,
			})
			if err != nil {
				t.Fatal(err)
			}
			if run.Run.Branch != testCase.expectedBranch {
				t.Fatalf("trend resolution %q branch = %q, want %q", testCase.resolution, run.Run.Branch, testCase.expectedBranch)
			}
			if run.Run.TrendResolution != testCase.resolution {
				t.Fatalf("trend resolution did not persist: %+v", run.Run)
			}
			if testCase.expectedBranch == manju.ScriptBranchOriginal {
				if _, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{
					Title: "热点方向", Content: "复仇女主在雨夜发现真相。",
				}); err != nil {
					t.Fatal(err)
				}
				assertSourceManifestRouting(t, f, run.Run.ID, manju.ScriptInputOutline, testCase.resolution)
			}
		})
	}
}

func assertSourceManifestRouting(t *testing.T, f fixture, runID, inputKind, trendResolution string) {
	t.Helper()
	var artifact model.ManjuScriptArtifact
	if err := f.db.Order("version desc").First(
		&artifact,
		"run_id = ? AND kind = ?",
		runID,
		model.ManjuScriptArtifactSourceManifest,
	).Error; err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		InputKind           *string  `json:"inputKind"`
		TrendResolution     *string  `json:"trendResolution"`
		DownstreamConsumers []string `json:"downstreamConsumers"`
	}
	if err := json.Unmarshal([]byte(artifact.Content), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.InputKind == nil || *manifest.InputKind != inputKind {
		t.Fatalf("SOURCE_MANIFEST inputKind = %v, want %q", manifest.InputKind, inputKind)
	}
	if manifest.TrendResolution == nil || *manifest.TrendResolution != trendResolution {
		t.Fatalf("SOURCE_MANIFEST trendResolution = %v, want %q", manifest.TrendResolution, trendResolution)
	}
	expectedConsumers := "ip_development,series_bible,episode_planning,script_draft"
	if got := strings.Join(manifest.DownstreamConsumers, ","); got != expectedConsumers {
		t.Fatalf("SOURCE_MANIFEST downstreamConsumers = %q, want %q", got, expectedConsumers)
	}
}

func TestLegacyPolishSourceCannotBeSavedAsP0(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = "polish"

	_, err := f.service.SaveLock(f.ownerID, f.projectID, lock)
	if !errors.Is(err, manju.ErrInvalidLock) {
		t.Fatalf("polish is a branch input, not a fifth P0 source; got %v", err)
	}
}

func TestLegacyPolishRunRemainsReadable(t *testing.T) {
	f := newFixture(t)
	run := &model.ManjuScriptRun{
		ID: "legacy-polish-run", ProjectID: f.projectID, Branch: manju.ScriptBranchPolish,
		Mode:      manju.ScriptModeStepwise,
		Status:    model.ManjuScriptRunStatusCompleted,
		CreatedBy: f.ownerID,
	}
	if err := f.db.Create(run).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Run.Branch != manju.ScriptBranchPolish || detail.Run.InputKind != "" {
		t.Fatalf("legacy polish run must remain readable: %+v", detail.Run)
	}
}

func TestAdaptationRunRegistersSourceRangeAndPausesStepwise(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatalf("保存改编 P0 失败: %v", err)
	}

	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise,
	})
	if err != nil {
		t.Fatalf("创建小说改编 P1 失败: %v", err)
	}
	if run.Run.Branch != manju.ScriptBranchAdaptation {
		t.Fatalf("P0 adaptation 必须推导小说改编支线，得到 %q", run.Run.Branch)
	}

	const novel = "第一章 雨夜\n林澈推开旧宅的门。"
	detail, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{
		Title:              "雨夜旧宅",
		AuthorizationScope: "仅用于本项目改编，不得转授权",
		RangeLabel:         "第 1 章第 1-2 节",
		Content:            novel,
	})
	if err != nil {
		t.Fatalf("登记小说来源失败: %v", err)
	}
	if detail.Source == nil {
		t.Fatal("来源登记必须返回服务端来源账本")
	}
	if detail.Source.RangeLabel != "第 1 章第 1-2 节" || detail.Source.AuthorizationScope == "" {
		t.Fatalf("来源范围与授权范围不能丢失: %+v", detail.Source)
	}
	if detail.Source.ContentHash == "" || detail.Source.CharacterCount != int64(len([]rune(novel))) {
		t.Fatalf("来源必须保存内容哈希与字符数: %+v", detail.Source)
	}
	stage := findScriptStage(t, detail, manju.ScriptStageSourceRegistration)
	if stage.Status != manju.ScriptStageStatusAwaitingConfirmation {
		t.Fatalf("逐步模式来源登记后必须等待确认，得到 %s", stage.Status)
	}
	next := findScriptStage(t, detail, manju.ScriptStageIPDevelopment)
	if next.Status != manju.ScriptStageStatusNotStarted {
		t.Fatalf("来源未确认前下一阶段不得启动，得到 %s", next.Status)
	}
}

func findScriptStage(t *testing.T, detail *manju.ScriptRunDetail, key string) manju.ScriptStageView {
	t.Helper()
	for _, stage := range detail.Stages {
		if stage.Key == key {
			return stage
		}
	}
	t.Fatalf("缺少阶段 %s: %+v", key, detail.Stages)
	return manju.ScriptStageView{}
}

func prepareRoundtableAwaitingDecision(t *testing.T, f fixture) *manju.ScriptRunDetail {
	t.Helper()
	return prepareRoundtableAwaitingDecisionWithReview(t, f, "# 五席圆桌审核\ngrade: A\n等待用户选择。")
}

func prepareRoundtableAwaitingDecisionWithReview(t *testing.T, f fixture, reviewContent string) *manju.ScriptRunDetail {
	t.Helper()
	now := time.Now()
	run := &model.ManjuScriptRun{
		ID: "run-roundtable-decision", ProjectID: f.projectID, Branch: manju.ScriptBranchOriginal,
		Mode: manju.ScriptModeStepwise, Status: model.ManjuScriptRunStatusActive,
		CurrentStage: manju.ScriptStageRoundtableReview, LockRevision: 1,
		CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}
	if err := f.db.Create(run).Error; err != nil {
		t.Fatal(err)
	}
	scriptArtifact := &model.ManjuScriptArtifact{
		ID: "artifact-script-v1", ProjectID: f.projectID, RunID: run.ID, StageID: "stage-script-draft",
		Kind: "SCRIPT_DRAFT.md", Version: 1, Content: "# 正式剧本 v1\n林澈推开旧宅的门。",
		ContentHash: "script-v1", CreatedBy: f.ownerID, CreatedAt: now,
	}
	reviewArtifact := &model.ManjuScriptArtifact{
		ID: "artifact-roundtable-v1", ProjectID: f.projectID, RunID: run.ID, StageID: "stage-roundtable-review",
		Kind: "ROUNDTABLE_REVIEW.md", Version: 1, Content: reviewContent,
		ContentHash: "roundtable-v1", CreatedBy: f.ownerID, CreatedAt: now,
	}
	if err := f.db.Create(&[]model.ManjuScriptArtifact{*scriptArtifact, *reviewArtifact}).Error; err != nil {
		t.Fatal(err)
	}
	stages := []model.ManjuScriptStage{
		{ID: "stage-source", RunID: run.ID, Key: manju.ScriptStageSourceRegistration, Title: "来源登记", Position: 1, Status: manju.ScriptStageStatusLocked, Version: 1},
		{ID: "stage-ip", RunID: run.ID, Key: manju.ScriptStageIPDevelopment, Title: "IP 与角色开发", Position: 2, Status: manju.ScriptStageStatusLocked, Version: 1},
		{ID: "stage-series", RunID: run.ID, Key: manju.ScriptStageSeriesBible, Title: "全剧开发圣经", Position: 3, Status: manju.ScriptStageStatusLocked, Version: 1},
		{ID: "stage-episodes", RunID: run.ID, Key: manju.ScriptStageEpisodePlanning, Title: "分集规划", Position: 4, Status: manju.ScriptStageStatusLocked, Version: 1},
		{ID: "stage-script-draft", RunID: run.ID, Key: manju.ScriptStageScriptDraft, Title: "正式剧本", Position: 5, Status: manju.ScriptStageStatusLocked, Version: 1, ArtifactID: scriptArtifact.ID},
		{ID: "stage-roundtable-review", RunID: run.ID, Key: manju.ScriptStageRoundtableReview, Title: "五席圆桌审核", Position: 6, Status: manju.ScriptStageStatusAwaitingDecision, Version: 1, ArtifactID: reviewArtifact.ID},
	}
	if err := f.db.Create(&stages).Error; err != nil {
		t.Fatal(err)
	}
	detail, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return detail
}

func TestRoundtableCannotLockWithoutExplicitDecision(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecision(t, f)

	_, err := f.service.ConfirmScriptStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
	)
	if !errors.Is(err, manju.ErrRoundtableDecisionRequired) {
		t.Fatalf("roundtable confirmation must require a decision, got %v", err)
	}
	detail, detailErr := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if detailErr != nil {
		t.Fatal(detailErr)
	}
	if got := findScriptStage(t, detail, manju.ScriptStageRoundtableReview).Status; got != manju.ScriptStageStatusAwaitingDecision {
		t.Fatalf("missing decision must not mutate review status, got %s", got)
	}
	if detail.Run.Status != model.ManjuScriptRunStatusActive {
		t.Fatalf("missing decision must keep run active, got %s", detail.Run.Status)
	}
}

func TestRoundtableManualCorrectionCannotBypassDecision(t *testing.T) {
	t.Run("awaiting decision", func(t *testing.T) {
		f := newFixture(t)
		run := prepareRoundtableAwaitingDecision(t, f)

		_, err := f.service.ManualCorrectScriptArtifact(
			f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
			manju.ManualArtifactCorrectionInput{Content: "# 人工改写圆桌审核\ngrade: A"},
		)
		if !errors.Is(err, manju.ErrRoundtableDecisionEndpointRequired) {
			t.Fatalf("roundtable manual correction must require the decision endpoint, got %v", err)
		}
		detail, detailErr := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
		if detailErr != nil {
			t.Fatal(detailErr)
		}
		review := findScriptStage(t, detail, manju.ScriptStageRoundtableReview)
		if review.Status != manju.ScriptStageStatusAwaitingDecision ||
			review.ArtifactID != "artifact-roundtable-v1" ||
			review.Version != 1 {
			t.Fatalf("rejected manual correction must not mutate the review: %+v", review)
		}
	})

	t.Run("locked by decision ledger", func(t *testing.T) {
		f := newFixture(t)
		run := prepareRoundtableAwaitingDecision(t, f)
		if _, err := f.service.DecideRoundtable(
			f.ownerID, f.projectID, run.Run.ID,
			manju.RoundtableDecisionInput{Decision: manju.RoundtableProceed, Note: "accept"},
		); err != nil {
			t.Fatal(err)
		}

		_, err := f.service.ManualCorrectScriptArtifact(
			f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
			manju.ManualArtifactCorrectionInput{Content: "# 人工改写已锁定圆桌审核\ngrade: A"},
		)
		if !errors.Is(err, manju.ErrRoundtableDecisionEndpointRequired) {
			t.Fatalf("locked roundtable review must remain immutable, got %v", err)
		}
	})
}

func TestRoundtableGenericConfirmRequiresDecisionLedger(t *testing.T) {
	t.Run("rejects legacy awaiting confirmation state", func(t *testing.T) {
		f := newFixture(t)
		run := prepareRoundtableAwaitingDecision(t, f)
		if err := f.db.Model(&model.ManjuScriptStage{}).
			Where("run_id = ? AND key = ?", run.Run.ID, manju.ScriptStageRoundtableReview).
			Update("status", manju.ScriptStageStatusAwaitingConfirm).Error; err != nil {
			t.Fatal(err)
		}

		_, err := f.service.ConfirmScriptStage(
			f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
		)
		if !errors.Is(err, manju.ErrRoundtableDecisionEndpointRequired) {
			t.Fatalf("generic roundtable confirmation must be rejected without a ledger row, got %v", err)
		}
	})

	t.Run("rejects locked state without ledger", func(t *testing.T) {
		f := newFixture(t)
		run := prepareRoundtableAwaitingDecision(t, f)
		if err := f.db.Model(&model.ManjuScriptStage{}).
			Where("run_id = ? AND key = ?", run.Run.ID, manju.ScriptStageRoundtableReview).
			Update("status", manju.ScriptStageStatusLocked).Error; err != nil {
			t.Fatal(err)
		}

		_, err := f.service.ConfirmScriptStage(
			f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
		)
		if !errors.Is(err, manju.ErrRoundtableDecisionEndpointRequired) {
			t.Fatalf("generic confirmation of a forged locked review must be rejected, got %v", err)
		}
	})

	t.Run("allows idempotent confirmation after ledger decision", func(t *testing.T) {
		f := newFixture(t)
		run := prepareRoundtableAwaitingDecision(t, f)
		if _, err := f.service.DecideRoundtable(
			f.ownerID, f.projectID, run.Run.ID,
			manju.RoundtableDecisionInput{Decision: manju.RoundtableProceed, Note: "accept"},
		); err != nil {
			t.Fatal(err)
		}

		if _, err := f.service.ConfirmScriptStage(
			f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
		); err != nil {
			t.Fatalf("idempotent confirmation after a ledger decision must remain valid: %v", err)
		}
	})
}

func TestRoundtableDecisionRequiresAllowedValue(t *testing.T) {
	for _, decision := range []manju.RoundtableDecision{"", "CONFIRM"} {
		t.Run(string(decision), func(t *testing.T) {
			f := newFixture(t)
			run := prepareRoundtableAwaitingDecision(t, f)

			_, err := f.service.DecideRoundtable(
				f.ownerID, f.projectID, run.Run.ID,
				manju.RoundtableDecisionInput{Decision: decision},
			)
			if !errors.Is(err, manju.ErrRoundtableDecisionRequired) {
				t.Fatalf("decision %q must be rejected, got %v", decision, err)
			}
			detail, detailErr := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
			if detailErr != nil {
				t.Fatal(detailErr)
			}
			if got := findScriptStage(t, detail, manju.ScriptStageRoundtableReview).Status; got != manju.ScriptStageStatusAwaitingDecision {
				t.Fatalf("invalid decision must not mutate review status, got %s", got)
			}
		})
	}
}

func TestRoundtableGradeMustBeSingleAndAuthoritative(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{name: "missing", content: "# 五席圆桌审核\n等待用户选择。"},
		{name: "upgrade substring is not authoritative", content: `# 五席圆桌审核
{"upgrade":"A"}`},
		{name: "downgrade substring is not authoritative", content: `# 五席圆桌审核
{"downgrade":"A"}`},
		{name: "conflicting field and table", content: "# 五席圆桌审核\ngrade: A\n| 等级 | B |\n等待用户选择。"},
		{name: "conflicting JSON and table", content: "# 五席圆桌审核\n{\"grade\":\"A\"}\n| 等级 | B |\n等待用户选择。"},
		{name: "unsupported", content: "# 五席圆桌审核\ngrade: B+\n等待用户选择。"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			run := prepareRoundtableAwaitingDecisionWithReview(t, f, testCase.content)

			_, err := f.service.DecideRoundtable(
				f.ownerID, f.projectID, run.Run.ID,
				manju.RoundtableDecisionInput{
					Decision: manju.RoundtableProceed, AcceptQualityRisk: true,
				},
			)
			if !errors.Is(err, manju.ErrRoundtableGradeInvalid) {
				t.Fatalf("ambiguous roundtable grade must fail closed, got %v", err)
			}
			detail, detailErr := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
			if detailErr != nil {
				t.Fatal(detailErr)
			}
			review := findScriptStage(t, detail, manju.ScriptStageRoundtableReview)
			if review.Status != manju.ScriptStageStatusAwaitingDecision ||
				review.ReviewGrade != manju.RoundtableGradeUnknown {
				t.Fatalf("invalid grade must not advance or invent a grade: %+v", review)
			}
			if len(detail.RoundtableDecisions) != 0 {
				t.Fatalf("invalid grade must not create a decision ledger row: %+v", detail.RoundtableDecisions)
			}
		})
	}
}

func TestRoundtableChineseGradeLabelStillRequiresRiskAcceptance(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecisionWithReview(
		t, f, "# 五席圆桌审核\n等级：B\n等待用户选择。",
	)

	_, err := f.service.DecideRoundtable(
		f.ownerID, f.projectID, run.Run.ID,
		manju.RoundtableDecisionInput{Decision: manju.RoundtableProceed},
	)
	if !errors.Is(err, manju.ErrRoundtableQualityRiskRequired) {
		t.Fatalf("Chinese grade label must preserve the B risk gate, got %v", err)
	}

	detail, err := f.service.DecideRoundtable(
		f.ownerID, f.projectID, run.Run.ID,
		manju.RoundtableDecisionInput{
			Decision: manju.RoundtableProceed, AcceptQualityRisk: true,
			Note: "USER_ACCEPTED_QUALITY_RISK",
		},
	)
	if err != nil {
		t.Fatalf("explicit risk acceptance must allow an authoritative B grade: %v", err)
	}
	if len(detail.RoundtableDecisions) != 1 || detail.RoundtableDecisions[0].Grade != "B" {
		t.Fatalf("Chinese grade label must persist the authoritative grade: %+v", detail.RoundtableDecisions)
	}
}

func TestRoundtableProceedRequiresRiskAcceptanceForBAndNotReady(t *testing.T) {
	for _, grade := range []string{"B", "NOT_READY"} {
		t.Run(grade+" rejected without risk acceptance", func(t *testing.T) {
			f := newFixture(t)
			run := prepareRoundtableAwaitingDecisionWithReview(
				t, f, "# 五席圆桌审核\ngrade: "+grade+"\n等待用户选择。",
			)

			_, err := f.service.DecideRoundtable(
				f.ownerID, f.projectID, run.Run.ID,
				manju.RoundtableDecisionInput{Decision: manju.RoundtableProceed},
			)
			if !errors.Is(err, manju.ErrRoundtableQualityRiskRequired) {
				t.Fatalf("grade %s proceed without risk acceptance must fail, got %v", grade, err)
			}
			detail, detailErr := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
			if detailErr != nil {
				t.Fatal(detailErr)
			}
			review := findScriptStage(t, detail, manju.ScriptStageRoundtableReview)
			if review.Status != manju.ScriptStageStatusAwaitingDecision {
				t.Fatalf("rejected risk acceptance must leave review awaiting decision, got %s", review.Status)
			}
			if review.ReviewGrade != grade {
				t.Fatalf("review grade must be exposed before decision: got %q want %q", review.ReviewGrade, grade)
			}
			if len(detail.RoundtableDecisions) != 0 {
				t.Fatalf("rejected decision must not create a ledger row: %+v", detail.RoundtableDecisions)
			}
		})

		t.Run(grade+" accepted with risk acceptance", func(t *testing.T) {
			f := newFixture(t)
			run := prepareRoundtableAwaitingDecisionWithReview(
				t, f, "# 五席圆桌审核\ngrade: "+grade+"\n等待用户选择。",
			)

			detail, err := f.service.DecideRoundtable(
				f.ownerID, f.projectID, run.Run.ID,
				manju.RoundtableDecisionInput{
					Decision: manju.RoundtableProceed, AcceptQualityRisk: true,
					Note: "接受 " + grade + " 质量风险",
				},
			)
			if err != nil {
				t.Fatalf("grade %s proceed with explicit risk acceptance must pass: %v", grade, err)
			}
			if got := findScriptStage(t, detail, manju.ScriptStageRoundtableReview).Status; got != manju.ScriptStageStatusLocked {
				t.Fatalf("accepted risk must lock review, got %s", got)
			}
			if len(detail.RoundtableDecisions) != 1 ||
				!detail.RoundtableDecisions[0].RiskAccepted ||
				detail.RoundtableDecisions[0].Grade != grade {
				t.Fatalf("risk-accepted decision ledger mismatch: %+v", detail.RoundtableDecisions)
			}
		})
	}
}

func TestRoundtableDecisionPersistsImmutableLedgerRecord(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecisionWithReview(
		t, f, "# 五席圆桌审核\ngrade: B\nR1_DURABLE_LEDGER",
	)

	detail, err := f.service.DecideRoundtable(
		f.ownerID, f.projectID, run.Run.ID,
		manju.RoundtableDecisionInput{
			Decision: manju.RoundtableProceed, AcceptQualityRisk: true,
			Note: "DURABLE_DECISION_NOTE",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.RoundtableDecisions) != 1 {
		t.Fatalf("roundtable decision must be returned by run detail: %+v", detail.RoundtableDecisions)
	}
	record := detail.RoundtableDecisions[0]
	if record.ProjectID != f.projectID ||
		record.RunID != run.Run.ID ||
		record.StageID != "stage-roundtable-review" ||
		record.Round != 1 ||
		record.Version != 1 ||
		record.Decision != string(manju.RoundtableProceed) ||
		record.ActorID != f.ownerID ||
		record.Note != "DURABLE_DECISION_NOTE" ||
		record.ReviewArtifactID != "artifact-roundtable-v1" ||
		record.ReviewArtifactHash != "roundtable-v1" ||
		record.Grade != "B" ||
		!record.RiskAccepted ||
		record.CreatedAt.IsZero() {
		t.Fatalf("decision ledger record is incomplete: %+v", record)
	}

	var stored model.ManjuRoundtableDecision
	if err := f.db.First(&stored, "id = ?", record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Decision != string(manju.RoundtableProceed) || stored.ReviewArtifactHash != "roundtable-v1" {
		t.Fatalf("decision row was not persisted immutably: %+v", stored)
	}
	if err := f.db.Model(&model.ManjuScriptStage{}).
		Where("run_id = ? AND key = ?", run.Run.ID, manju.ScriptStageRoundtableReview).
		Update("input_json", `{"decision":"REFINE_OR_REWRITE"}`).Error; err != nil {
		t.Fatal(err)
	}
	reloaded, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.RoundtableDecisions) != 1 ||
		reloaded.RoundtableDecisions[0].Decision != string(manju.RoundtableProceed) {
		t.Fatalf("decision view must come from the ledger, not mutable stage input: %+v", reloaded.RoundtableDecisions)
	}
}

func TestRoundtableProceedCurrentLocksReviewAndCompletesRun(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecision(t, f)

	detail, err := f.service.DecideRoundtable(
		f.ownerID, f.projectID, run.Run.ID,
		manju.RoundtableDecisionInput{Decision: manju.RoundtableProceed, Note: "接受当前版本"},
	)
	if err != nil {
		t.Fatal(err)
	}
	review := findScriptStage(t, detail, manju.ScriptStageRoundtableReview)
	if review.Status != manju.ScriptStageStatusLocked {
		t.Fatalf("PROCEED_CURRENT must lock review, got %+v", review)
	}
	if review.ConfirmedBy != f.ownerID || review.ConfirmedAt == nil {
		t.Fatalf("PROCEED_CURRENT must persist the decision confirmation, got %+v", review)
	}
	if review.ArtifactID != "artifact-roundtable-v1" {
		t.Fatalf("PROCEED_CURRENT must preserve the accepted review artifact, got %+v", review)
	}
	if detail.Run.Status != model.ManjuScriptRunStatusCompleted || detail.Run.CurrentStage != "" {
		t.Fatalf("PROCEED_CURRENT must complete the script chain, got %+v", detail.Run)
	}
}

func TestRoundtableRefineReopensScriptDraftAtNextVersion(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecision(t, f)
	before := findScriptStage(t, run, manju.ScriptStageScriptDraft)

	detail, err := f.service.DecideRoundtable(
		f.ownerID, f.projectID, run.Run.ID,
		manju.RoundtableDecisionInput{Decision: manju.RoundtableRefine, Note: "按圆桌意见精修"},
	)
	if err != nil {
		t.Fatal(err)
	}
	review := findScriptStage(t, detail, manju.ScriptStageRoundtableReview)
	if review.Status != manju.ScriptStageStatusStale {
		t.Fatalf("REFINE_OR_REWRITE must stale review, got %+v", review)
	}
	if review.ArtifactID != "artifact-roundtable-v1" {
		t.Fatalf("REFINE_OR_REWRITE must preserve the review artifact as history, got %+v", review)
	}
	scriptDraft := findScriptStage(t, detail, manju.ScriptStageScriptDraft)
	if scriptDraft.Status != manju.ScriptStageStatusInProgress {
		t.Fatalf("REFINE_OR_REWRITE must reopen script_draft, got %+v", scriptDraft)
	}
	if scriptDraft.Version != before.Version+1 {
		t.Fatalf("script_draft must reopen at the next version: before=%d after=%d", before.Version, scriptDraft.Version)
	}
	if scriptDraft.TaskID != "" {
		t.Fatalf("reopened script_draft must wait for a new task, got %+v", scriptDraft)
	}
	if detail.Run.Status != model.ManjuScriptRunStatusActive || detail.Run.CurrentStage != manju.ScriptStageScriptDraft {
		t.Fatalf("REFINE_OR_REWRITE must make script_draft current, got %+v", detail.Run)
	}
}

func TestRoundtableRefineMaterializesExactlyOneNewScriptVersion(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecision(t, f)
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-roundtable-refine", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.DecideRoundtable(
		f.ownerID, f.projectID, run.Run.ID,
		manju.RoundtableDecisionInput{Decision: manju.RoundtableRefine, Note: "生成新版本"},
	); err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "task-roundtable-refine", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)
	if _, err := f.service.RunScriptStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageScriptDraft,
		manju.ScriptStageRunInput{Instruction: "按圆桌意见精修"},
	); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, task.ID, "# 正式剧本 v2\n林澈停在门内。")

	detail, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	scriptDraft := findScriptStage(t, detail, manju.ScriptStageScriptDraft)
	if scriptDraft.Status != manju.ScriptStageStatusAwaitingConfirmation || scriptDraft.Version != 2 {
		t.Fatalf("refined script must produce one pending v2 artifact, got %+v", scriptDraft)
	}
	if scriptDraft.ArtifactID == "" || scriptDraft.ArtifactID == "artifact-script-v1" {
		t.Fatalf("refined script must reference the new artifact, got %+v", scriptDraft)
	}
	var artifact model.ManjuScriptArtifact
	if err := f.db.First(&artifact, "id = ?", scriptDraft.ArtifactID).Error; err != nil {
		t.Fatal(err)
	}
	if artifact.Version != 2 || !strings.Contains(artifact.Content, "林澈停在门内") {
		t.Fatalf("new script artifact must be v2, got %+v", artifact)
	}
	if _, err := f.service.ConfirmScriptStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageScriptDraft,
	); err != nil {
		t.Fatal(err)
	}
	detail, err = f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	review := findScriptStage(t, detail, manju.ScriptStageRoundtableReview)
	if review.Status != manju.ScriptStageStatusInProgress ||
		review.TaskID != "" ||
		review.ArtifactID != "" ||
		review.CompletedAt != nil {
		t.Fatalf("reopened roundtable must not retain the previous review task or artifact: %+v", review)
	}
}

func TestRoundtableRegenerationRejectedWhileAwaitingDecision(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecision(t, f)

	_, err := f.service.RegenerateScriptArtifact(
		f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
		manju.ScriptStageRunInput{Instruction: "试图绕过决策重生成评审"},
	)
	if !errors.Is(err, manju.ErrScriptStageNotReady) {
		t.Fatalf("roundtable regeneration must be blocked while awaiting decision, got %v", err)
	}
}

func TestScriptRunDetailRepairsStaleCompletedArtifactOnInProgressRoundtable(t *testing.T) {
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecision(t, f)
	task := &model.Task{
		ID: "task-roundtable-stale-artifact", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusSucceeded,
		ResultJSON: `{"mode":"text","text":"# 五席圆桌审核\ngrade: A\n等待用户选择。"}`,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	var reviewArtifact model.ManjuScriptArtifact
	if err := f.db.First(&reviewArtifact, "id = ?", "artifact-roundtable-v1").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&reviewArtifact).Update("task_id", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	startedAt := reviewArtifact.CreatedAt.Add(time.Minute)
	if err := f.db.Model(&model.ManjuScriptStage{}).
		Where("run_id = ? AND key = ?", run.Run.ID, manju.ScriptStageRoundtableReview).
		Updates(map[string]any{
			"status":     manju.ScriptStageStatusInProgress,
			"task_id":    task.ID,
			"started_at": startedAt,
		}).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	review := findScriptStage(t, detail, manju.ScriptStageRoundtableReview)
	if review.Status != manju.ScriptStageStatusInProgress ||
		review.TaskID != "" ||
		review.ArtifactID != "" ||
		!strings.Contains(review.Error, "上一版剧本") {
		t.Fatalf("stale roundtable artifact must reopen the stage instead of blocking it: %+v", review)
	}
}

func TestRoundtableR2PromptIsBlindToPriorReviewContent(t *testing.T) {
	const reviewPhrase = "R1_DISTINCTIVE_CONCLUSION_MUST_NOT_REACH_R2"
	f := newFixture(t)
	run := prepareRoundtableAwaitingDecisionWithReview(
		t, f, "# 五席圆桌审核\ngrade: B\n"+reviewPhrase,
	)
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-roundtable-blind-review", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	scriptTask := &model.Task{
		ID: "task-roundtable-refine-blind-writer", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	reviewTask := &model.Task{
		ID: "task-roundtable-r2-blind-review", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(&[]model.Task{*scriptTask, *reviewTask}).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{scriptTask, reviewTask}}
	f.service.SetScriptTaskCreator(creator.create)

	if _, err := f.service.DecideRoundtable(
		f.ownerID, f.projectID, run.Run.ID,
		manju.RoundtableDecisionInput{Decision: manju.RoundtableRefine},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunScriptStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageScriptDraft,
		manju.ScriptStageRunInput{Instruction: "按 R1 返修清单精修"},
	); err != nil {
		t.Fatal(err)
	}
	if len(creator.requests) != 1 || !strings.Contains(creator.requests[0].Prompt, reviewPhrase) {
		t.Fatalf("script_draft writer must be allowed to consume R1 feedback: %+v", creator.requests)
	}
	succeedManjuTask(t, f, scriptTask.ID, "# 正式剧本 v2\n林澈停在门内。")
	if _, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmScriptStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageScriptDraft,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunScriptStage(
		f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageRoundtableReview,
		manju.ScriptStageRunInput{Instruction: "对 v2 做独立盲审"},
	); err != nil {
		t.Fatal(err)
	}
	if len(creator.requests) != 2 {
		t.Fatalf("expected script writer and R2 review tasks, got %d", len(creator.requests))
	}
	if strings.Contains(creator.requests[1].Prompt, reviewPhrase) {
		t.Fatalf("R2 review prompt leaked prior review conclusion:\n%s", creator.requests[1].Prompt)
	}
}

func TestStepwiseConfirmationLocksCurrentStageAndStartsNext(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeStepwise})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{
		Title: "雨夜旧宅", AuthorizationScope: "本项目改编授权", RangeLabel: "第 1 章", Content: "林澈推开旧宅的门。",
	}); err != nil {
		t.Fatal(err)
	}

	detail, err := f.service.ConfirmScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageSourceRegistration)
	if err != nil {
		t.Fatal(err)
	}
	if got := findScriptStage(t, detail, manju.ScriptStageSourceRegistration).Status; got != manju.ScriptStageStatusLocked {
		t.Fatalf("确认后来源阶段必须锁定，得到 %s", got)
	}
	if got := findScriptStage(t, detail, manju.ScriptStageIPDevelopment).Status; got != manju.ScriptStageStatusInProgress {
		t.Fatalf("确认后下一阶段必须启动，得到 %s", got)
	}
}

func TestSaveNewLockAbandonsScriptRunsFromPreviousRevision(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	firstReceipt, err := f.service.SaveLock(f.ownerID, f.projectID, lock)
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeStepwise})
	if err != nil {
		t.Fatal(err)
	}
	if run.Run.LockRevision != firstReceipt.Revision {
		t.Fatalf("首条剧本运行必须绑定 revision %d: %+v", firstReceipt.Revision, run.Run)
	}

	secondReceipt, err := f.service.SaveLock(f.ownerID, f.projectID, lock)
	if err != nil {
		t.Fatal(err)
	}
	if secondReceipt.Revision <= firstReceipt.Revision {
		t.Fatalf("重新锁定必须生成新 revision: first=%d second=%d", firstReceipt.Revision, secondReceipt.Revision)
	}
	var stored model.ManjuScriptRun
	if err := f.db.First(&stored, "id = ?", run.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.ManjuScriptRunStatusAbandoned || stored.CurrentStage != "" {
		t.Fatalf("新 P0 revision 必须结束旧 revision 的 ACTIVE 剧本运行: %+v", stored)
	}
}

func TestCreateScriptRunAbandonsLegacyActiveRunFromPreviousRevision(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-p1-revision", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	legacy, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeStepwise})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, legacy.Run.ID, manju.ScriptSourceInput{
		Title: "旧单元", AuthorizationScope: "授权", RangeLabel: "第 1 章", Content: "旧单元正文",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	// Simulate legacy data created before SaveLock learned to abandon stale runs.
	if err := f.db.Model(&model.ManjuScriptRun{}).Where("id = ?", legacy.Run.ID).Updates(map[string]any{
		"status": model.ManjuScriptRunStatusActive, "current_stage": manju.ScriptStageIPDevelopment,
	}).Error; err != nil {
		t.Fatal(err)
	}

	current, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeStepwise})
	if err != nil {
		t.Fatalf("创建新 P1 前应自愈旧 revision 的遗留 ACTIVE 运行: %v", err)
	}
	if current.Run.LockRevision != 2 {
		t.Fatalf("新 P1 必须绑定当前 revision 2，得到 %+v", current.Run)
	}
	var stored model.ManjuScriptRun
	if err := f.db.First(&stored, "id = ?", legacy.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.ManjuScriptRunStatusAbandoned || stored.CurrentStage != "" {
		t.Fatalf("遗留旧 revision 运行必须转为 ABANDONED: %+v", stored)
	}
}

func TestConfirmingChangedSourceMarksDownstreamStale(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeStepwise})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{Title: "旧稿", AuthorizationScope: "授权", RangeLabel: "第 1 章", Content: "旧稿正文"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageSourceRegistration); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{Title: "新稿", AuthorizationScope: "授权", RangeLabel: "第 1 章修订版", Content: "新稿正文"}); err != nil {
		t.Fatalf("登记上游修改失败: %v", err)
	}
	detail, err := f.service.ConfirmScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageSourceRegistration)
	if err != nil {
		t.Fatal(err)
	}
	if got := findScriptStage(t, detail, manju.ScriptStageIPDevelopment).Status; got != manju.ScriptStageStatusInProgress {
		t.Fatalf("新来源确认后下一阶段必须重新启动，得到 %s", got)
	}
}

func TestBenchmarkAndPolishBranchesUseDifferentStageFlows(t *testing.T) {
	t.Run("benchmark starts with recon gate", func(t *testing.T) {
		f := newFixture(t)
		lock := validLock()
		lock.Source = manju.SourceBenchmark
		if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
			t.Fatal(err)
		}
		detail, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeStepwise})
		if err != nil {
			t.Fatal(err)
		}
		if detail.Run.Branch != manju.ScriptBranchBenchmark || detail.Stages[0].Key != manju.ScriptStageReconIntelligence {
			t.Fatalf("对标支线必须先走情报卡: %+v", detail)
		}
		if got := findScriptStage(t, detail, manju.ScriptStageSourceRegistration).Status; got != manju.ScriptStageStatusNotStarted {
			t.Fatalf("情报卡未确认前来源登记不能启动，得到 %s", got)
		}
	})

	t.Run("finished adaptation enters polish without script generation stages", func(t *testing.T) {
		f := newFixture(t)
		lock := validLock()
		lock.Source = manju.SourceAdaptation
		if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
			t.Fatal(err)
		}
		source, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
			Title: "待润色剧本", AuthorizationScope: "已获原作者润色授权",
			Content: "第一章 雨夜\n林澈：别开门。",
		})
		if err != nil {
			t.Fatal(err)
		}
		detail, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
			Mode: manju.ScriptModeStepwise, InputKind: manju.ScriptInputFinished,
			SourceDocumentID: source.Document.ID, ChapterIDs: []string{source.Chapters[0].ID},
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, stage := range detail.Stages {
			if stage.Key == manju.ScriptStageIPDevelopment || stage.Key == manju.ScriptStageEpisodePlanning {
				t.Fatalf("润色支线不能出现造剧本阶段: %+v", detail.Stages)
			}
		}
		if got := findScriptStage(t, detail, manju.ScriptStageSourceRegistration).Status; got != manju.ScriptStageStatusLocked {
			t.Fatalf("成型剧本来源登记后必须锁定，得到 %s", got)
		}
		if got := findScriptStage(t, detail, manju.ScriptStagePolishDraft).Status; got != manju.ScriptStageStatusInProgress {
			t.Fatalf("来源锁定后必须进入润色稿阶段，得到 %s", got)
		}
	})
}

func TestRunScriptStageCreatesTextTaskAndWaitsForRealSuccess(t *testing.T) {
	f := newFixture(t)
	detail := prepareAdaptationGeneration(t, f)
	runID := detail.Run.ID

	queued := &model.Task{
		ID: "task-script-1", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text",
		Status: model.TaskStatusQueued, Stage: "等待队列调度", Prompt: "执行 IP 与角色开发",
	}
	if err := f.db.Create(queued).Error; err != nil {
		t.Fatal(err)
	}
	creator := &recordingScriptTaskCreator{task: queued}
	f.service.SetScriptTaskCreator(creator.create)

	running, err := f.service.RunScriptStage(f.ownerID, f.projectID, runID, manju.ScriptStageIPDevelopment, manju.ScriptStageRunInput{
		Instruction: "突出雨夜压迫感与主角失控感",
	})
	if err != nil {
		t.Fatalf("启动阶段文本任务失败: %v", err)
	}
	stage := findScriptStage(t, running, manju.ScriptStageIPDevelopment)
	if stage.TaskID != queued.ID || stage.Status != manju.ScriptStageStatusInProgress {
		t.Fatalf("阶段必须绑定真实文本任务且等待结果: %+v", stage)
	}
	if creator.userID != f.ownerID || creator.request.ProjectID != f.projectID || creator.request.Type != "canvas_text" {
		t.Fatalf("任务必须走 BeefTV 文本任务服务: user=%s request=%+v", creator.userID, creator.request)
	}
	if !strings.Contains(creator.request.Prompt, "高反差大光比") {
		t.Fatalf("P1 必须收到画风卡的可投喂视觉参数: %s", creator.request.Prompt)
	}
	p0Prompt := creator.request.Prompt
	if boundary := strings.Index(p0Prompt, "===== V7.4.26 严格规则执行边界 ====="); boundary >= 0 {
		p0Prompt = p0Prompt[:boundary]
	}
	if strings.Contains(p0Prompt, "徐克") || strings.Contains(p0Prompt, "sd-style-009") || strings.Contains(p0Prompt, "styleCardName") || strings.Contains(p0Prompt, "styleCardId") {
		t.Fatalf("P1 P0 交付参数不得携带画风卡名称或选择层 ID: %s", p0Prompt)
	}
	metadata, _ := creator.request.Input["metadata"].(map[string]any)
	if metadata["manjuRunId"] != runID || metadata["manjuStageKey"] != manju.ScriptStageIPDevelopment {
		t.Fatalf("任务必须携带阶段追踪元数据: %+v", metadata)
	}
	if _, err := f.service.ConfirmScriptStage(f.ownerID, f.projectID, runID, manju.ScriptStageIPDevelopment); !errors.Is(err, manju.ErrScriptStageNotReady) {
		t.Fatalf("任务未成功前不能确认阶段，得到 %v", err)
	}

	if err := f.db.Model(&model.Task{}).Where("id = ?", queued.ID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成",
		"result_json": `{"mode":"text","text":"# IP 与角色开发\n林澈：失忆的档案修复师。"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	completed, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, runID)
	if err != nil {
		t.Fatal(err)
	}
	stage = findScriptStage(t, completed, manju.ScriptStageIPDevelopment)
	if stage.Status != manju.ScriptStageStatusAwaitingConfirmation || stage.ArtifactID == "" {
		t.Fatalf("只有真实成功结果才能生成阶段工件: %+v", stage)
	}
	if len(completed.Artifacts) < 2 || !strings.Contains(completed.Artifacts[len(completed.Artifacts)-1].Content, "失忆的档案修复师") {
		t.Fatalf("阶段工件必须保存模型真实输出: %+v", completed.Artifacts)
	}
}

func TestFailedScriptTaskBlocksStageWithoutArtifact(t *testing.T) {
	f := newFixture(t)
	detail := prepareAdaptationGeneration(t, f)
	runID := detail.Run.ID
	failed := &model.Task{
		ID: "task-script-failed", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text",
		Status: model.TaskStatusQueued, Prompt: "执行全剧开发圣经",
	}
	if err := f.db.Create(failed).Error; err != nil {
		t.Fatal(err)
	}
	creator := &recordingScriptTaskCreator{task: failed}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunScriptStage(f.ownerID, f.projectID, runID, manju.ScriptStageIPDevelopment, manju.ScriptStageRunInput{Instruction: "按项目锁开发"}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.Task{}).Where("id = ?", failed.ID).Updates(map[string]any{
		"status": model.TaskStatusFailed, "stage": "任务失败", "error": "上游模型拒绝请求",
	}).Error; err != nil {
		t.Fatal(err)
	}

	blocked, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, runID)
	if err != nil {
		t.Fatal(err)
	}
	stage := findScriptStage(t, blocked, manju.ScriptStageIPDevelopment)
	if stage.Status != manju.ScriptStageStatusBlocked || !strings.Contains(stage.Error, "拒绝") || stage.ArtifactID != "" {
		t.Fatalf("失败任务必须阻塞且不得生成工件: %+v", stage)
	}
}

func prepareAdaptationGeneration(t *testing.T, f fixture) *manju.ScriptRunDetail {
	t.Helper()
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-script", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
		ImageChannelID: "channel-image", ImageModelKey: "image-pro",
		VideoChannelID: "channel-video", VideoModelKey: "video-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeStepwise})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{
		Title: "雨夜旧宅", AuthorizationScope: "仅本项目改编", RangeLabel: "第 1 章", Content: "林澈推开旧宅的门。",
	}); err != nil {
		t.Fatal(err)
	}
	detail, err := f.service.ConfirmScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageSourceRegistration)
	if err != nil {
		t.Fatal(err)
	}
	return detail
}

type sequentialScriptTaskCreator struct {
	requests []localtask.CreateRequest
	tasks    []*model.Task
	err      error
}

func (f *sequentialScriptTaskCreator) create(_ string, request localtask.CreateRequest) (*model.Task, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	index := len(f.requests) - 1
	if index >= len(f.tasks) {
		return nil, errors.New("缺少顺序任务")
	}
	return f.tasks[index], nil
}
func TestFullPipelineAutoStartsNextStageAfterSourceRegistration(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-full-source", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	queued := &model.Task{
		ID: "task-full-source", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text",
		Status: model.TaskStatusQueued, Prompt: "执行 IP 与角色开发",
	}
	if err := f.db.Create(queued).Error; err != nil {
		t.Fatal(err)
	}
	creator := &recordingScriptTaskCreator{task: queued}
	f.service.SetScriptTaskCreator(creator.create)

	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeFullPipeline})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{
		Title: "雨夜旧宅", AuthorizationScope: "仅本项目改编", RangeLabel: "第 1 章", Content: "林澈推开旧宅的门。",
	})
	if err != nil {
		t.Fatal(err)
	}
	stage := findScriptStage(t, detail, manju.ScriptStageIPDevelopment)
	if stage.Status != manju.ScriptStageStatusInProgress || stage.TaskID != queued.ID {
		t.Fatalf("完整流程在来源锁定后必须真实提交下一阶段，得到 %+v", stage)
	}
	metadata, _ := creator.request.Input["metadata"].(map[string]any)
	if metadata["manjuStageKey"] != manju.ScriptStageIPDevelopment {
		t.Fatalf("自动任务必须携带下一阶段元数据: %+v", metadata)
	}
}

func TestFullPipelineReconciliationAutoStartsFollowingStage(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-full-chain", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	first := &model.Task{
		ID: "task-chain-ip", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text",
		Status: model.TaskStatusQueued, Prompt: "执行 IP 与角色开发",
	}
	second := &model.Task{
		ID: "task-chain-bible", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text",
		Status: model.TaskStatusQueued, Prompt: "执行全剧开发圣经",
	}
	if err := f.db.Create(first).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(second).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{first, second}}
	f.service.SetScriptTaskCreator(creator.create)

	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{Mode: manju.ScriptModeFullPipeline})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RegisterScriptSource(f.ownerID, f.projectID, run.Run.ID, manju.ScriptSourceInput{
		Title: "雨夜旧宅", AuthorizationScope: "仅本项目改编", RangeLabel: "第 1 章", Content: "林澈推开旧宅的门。",
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.Task{}).Where("id = ?", first.ID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成",
		"result_json": `{"mode":"text","text":"# IP 与角色开发\n林澈：失忆的档案修复师。"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findScriptStage(t, detail, manju.ScriptStageIPDevelopment).Status; got != manju.ScriptStageStatusLocked {
		t.Fatalf("成功阶段在完整流程中必须自动锁定，得到 %s", got)
	}
	next := findScriptStage(t, detail, manju.ScriptStageSeriesBible)
	if next.Status != manju.ScriptStageStatusInProgress || next.TaskID != second.ID {
		t.Fatalf("完整流程必须自动提交下一阶段真实任务，得到 %+v", next)
	}
	if len(creator.requests) != 2 {
		t.Fatalf("应依次提交两个真实任务，得到 %d", len(creator.requests))
	}
}
