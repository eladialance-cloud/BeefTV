package manju_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fixture struct {
	service   *manju.Service
	db        *gorm.DB
	projectID string
	ownerID   string
	producer  string
	writer    string
	outsider  string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "manju-service.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.User{}, &model.Project{}, &model.CanvasProject{}, &model.ManjuProjectMember{}, &model.ManjuLockReceipt{}, &model.ManjuProjectModelBinding{}, &model.ManjuRuleSnapshot{}, &model.ManjuRuleReadReceipt{}, &model.ManjuModelExchange{}, &model.ManjuArtifactLedger{}, &model.ManjuSourceDocument{}, &model.ManjuSourceChapter{}, &model.ManjuScriptRun{}, &model.ManjuScriptStage{}, &model.ManjuScriptSource{}, &model.ManjuScriptArtifact{}, &model.ManjuRoundtableDecision{}, &model.ManjuAssetRun{}, &model.ManjuAssetStage{}, &model.ManjuAssetArtifact{}, &model.ManjuAssetReference{}, &model.ManjuProductionRun{}, &model.ManjuProductionStage{}, &model.ManjuProductionArtifact{}, &model.ManjuProductionPrevis{}, &model.ManjuProductionImage{}, &model.ManjuProductionVideo{}, &model.ManjuStageDraft{}, &model.ManjuArtifactCorrection{}, &model.ManjuCanvasImport{}, &model.Result{}, &model.Resource{}, &model.ModelChannel{}, &model.ChannelModel{}, &model.Task{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	if err := db.AutoMigrate(&model.ManjuProjectStateSnapshot{}); err != nil {
		t.Fatalf("迁移漫剧状态台账表失败: %v", err)
	}

	f := fixture{
		service:   manju.New(repository.New(db)),
		db:        db,
		projectID: "project-manju-1",
		ownerID:   "user-owner",
		producer:  "user-producer",
		writer:    "user-writer",
		outsider:  "user-outsider",
	}
	f.service.SetSkillValidationEnabledForTest(false)
	users := []model.User{
		{ID: f.ownerID, Username: "owner", DisplayName: "主理人", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: f.producer, Username: "producer", DisplayName: "制片", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: f.writer, Username: "writer", DisplayName: "编剧", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: f.outsider, Username: "outsider", DisplayName: "外部用户", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	project := model.Project{
		ID:       f.projectID,
		UserID:   f.ownerID,
		Name:     "线上漫剧项目",
		Type:     "manju",
		Status:   model.ProjectStatusActive,
		Revision: 1,
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("创建测试项目失败: %v", err)
	}
	return f
}

func validLock() manju.LockInput {
	input := manju.LockInput{
		Track:                 "domestic",
		Source:                "original",
		VideoModel:            "seedance-2.0",
		AspectRatio:           "9:16",
		DeliveryMode:          "both",
		TargetDurationSeconds: 15,
		StyleBase:             "live-action",
		StyleCardID:           "sd-style-009",
		AssetMode:             "M1",
	}
	input.ConfirmedLocks = fullConfirmedLocks(input)
	return input
}

func fullConfirmedLocks(input manju.LockInput) []string {
	confirmed := []string{
		manju.LockTrack,
		manju.LockSource,
		manju.LockVideoModel,
		manju.LockAspectRatio,
		manju.LockDeliveryMode,
		manju.LockStyleCard,
	}
	if input.SeamMode != "" {
		confirmed = append(confirmed, manju.LockSeamMode)
	}
	return confirmed
}

func TestSaveLockRejectsImplicitHardLockValues(t *testing.T) {
	f := newFixture(t)
	input := validLock()
	input.ConfirmedLocks = []string{
		manju.LockTrack,
		manju.LockSource,
		manju.LockVideoModel,
		manju.LockAspectRatio,
	}

	_, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if !errors.Is(err, manju.ErrExplicitLockRequired) {
		t.Fatalf("missing delivery/style confirmations must fail, got %v", err)
	}
}

func TestSaveLockRejectsUnknownExplicitLockKey(t *testing.T) {
	f := newFixture(t)
	input := validLock()
	input.ConfirmedLocks = append(input.ConfirmedLocks, "styleBase")

	_, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if !errors.Is(err, manju.ErrInvalidLock) {
		t.Fatalf("unknown explicit lock keys must be rejected, got %v", err)
	}
}

func TestSaveLockPersistsExplicitSelectionProofInPayloadAndHash(t *testing.T) {
	f := newFixture(t)
	input := validLock()

	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if err != nil {
		t.Fatalf("explicit P0 lock should save: %v", err)
	}
	if got, want := strings.Join(receipt.Snapshot.ConfirmedLocks, ","), strings.Join(input.ConfirmedLocks, ","); got != want {
		t.Fatalf("confirmed lock proof = %q, want %q", got, want)
	}
	var stored model.ManjuLockReceipt
	if err := f.db.First(&stored, "id = ?", receipt.ID).Error; err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stored.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	confirmed, ok := payload["confirmedLocks"].([]any)
	if !ok || len(confirmed) != len(input.ConfirmedLocks) {
		t.Fatalf("payload must persist explicit lock proof: %+v", payload["confirmedLocks"])
	}
	sum := sha256.Sum256([]byte(stored.PayloadJSON))
	if receipt.ContentHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("lock hash must cover the persisted confirmation proof: payload=%s hash=%s", stored.PayloadJSON, receipt.ContentHash)
	}
}

func TestSaveLockRequiresExplicitSeamSelectionOnlyForMultiSegmentTask(t *testing.T) {
	f := newFixture(t)
	input := validLock()
	input.TargetDurationSeconds = 60
	input.SeamMode = "tail-frame-freeze"

	_, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if !errors.Is(err, manju.ErrExplicitLockRequired) {
		t.Fatalf("multi-segment lock without explicit seam confirmation must fail, got %v", err)
	}

	input.ConfirmedLocks = append(input.ConfirmedLocks, manju.LockSeamMode)
	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if err != nil {
		t.Fatalf("explicit seam confirmation should save: %v", err)
	}
	if receipt.SegmentCount != 4 {
		t.Fatalf("segment count = %d, want 4", receipt.SegmentCount)
	}
}

func TestProjectDetailReadsHistoricalLockWithoutConfirmedLocks(t *testing.T) {
	f := newFixture(t)
	payload, err := json.Marshal(map[string]any{
		"track":                   "domestic",
		"source":                  "original",
		"videoModel":              "seedance-2.0",
		"aspectRatio":             "9:16",
		"deliveryMode":            "both",
		"targetDurationSeconds":   15,
		"segmentCount":            1,
		"singleSegmentMaxSeconds": 15,
		"styleBase":               "live-action",
		"styleCardId":             "sd-style-009",
		"assetMode":               "M1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuLockReceipt{
		ID: "historical-lock", ProjectID: f.projectID, Revision: 1, Status: model.ManjuLockStatusLocked,
		PayloadJSON: string(payload), ContentHash: "historical-hash", CreatedBy: f.ownerID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := f.service.ProjectDetail(f.ownerID, f.projectID)
	if err != nil {
		t.Fatalf("historical lock receipt must remain readable: %v", err)
	}
	if detail.Lock == nil || len(detail.Lock.Snapshot.ConfirmedLocks) != 0 {
		t.Fatalf("historical receipt must not invent explicit confirmations: %+v", detail.Lock)
	}
}

func TestSaveLockRejectsCardOutsideV7426Catalog(t *testing.T) {
	f := newFixture(t)
	input := validLock()
	input.StyleCardID = "style-cold-realism"

	_, err := f.service.SaveLock(f.ownerID, f.projectID, input)

	if !errors.Is(err, manju.ErrInvalidLock) {
		t.Fatalf("目录外画风卡必须被拒绝，得到 %v", err)
	}
}

func TestSaveLockRejectsStyleBaseConflict(t *testing.T) {
	f := newFixture(t)
	input := validLock()
	input.StyleBase = "live-action"
	input.StyleCardID = "sd-style-064"

	_, err := f.service.SaveLock(f.ownerID, f.projectID, input)

	if !errors.Is(err, manju.ErrInvalidLock) {
		t.Fatalf("动画卡不能配真人实拍基底，得到 %v", err)
	}
}

func TestSaveLockStoresResolvedStyleMetadata(t *testing.T) {
	f := newFixture(t)

	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, validLock())
	if err != nil {
		t.Fatalf("保存目录内画风卡失败: %v", err)
	}

	payload, err := json.Marshal(receipt.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["styleCardName"] != "徐克" || snapshot["styleFamily"] != "T1" || snapshot["styleCatalogVersion"] != "V7.4.26" {
		t.Fatalf("P0 回执必须冻结画风卡解析结果: %+v", snapshot)
	}
	if snapshot["styleFightRating"] != float64(3) || snapshot["styleEffectRating"] != float64(3) {
		t.Fatalf("打斗/特效星级错误: %+v", snapshot)
	}
	if snapshot["stylePromptParameters"] == "" {
		t.Fatalf("画风卡必须冻结可投喂视觉参数: %+v", snapshot)
	}
	var state model.ManjuProjectStateSnapshot
	if err := f.db.First(&state, "project_id = ? AND lock_revision = ?", f.projectID, receipt.Revision).Error; err != nil {
		t.Fatalf("P0 必须初始化官方 project_state.json: %v", err)
	}
	if state.StageKey != "p0:lock" || state.SchemaVersion != "7.4.23" || state.ContentHash == "" {
		t.Fatalf("P0 状态台账快照不正确: %+v", state)
	}
	var statePayload map[string]any
	if err := json.Unmarshal([]byte(state.StateJSON), &statePayload); err != nil {
		t.Fatal(err)
	}
	locks, _ := statePayload["locks"].(map[string]any)
	if locks["aspect"] != receipt.Snapshot.AspectRatio || locks["asset_mode"] != receipt.Snapshot.AssetMode {
		t.Fatalf("project_state 未继承 P0 锁: %+v", locks)
	}
	status, err := f.service.RuleLibraryStatus(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Version != "V7.4.26" || status.FileCount != 170 || status.ProjectState == nil || status.ProjectState.StageKey != "p0:lock" {
		t.Fatalf("规则库状态不完整: %+v", status)
	}
}

func TestSaveLockMigratesMixedCustomStyleIntoSeparateP0Fields(t *testing.T) {
	f := newFixture(t)
	input := validLock()
	input.StyleCardID = "custom"
	input.StyleCustomDescription = strings.Join([]string{
		"北宋末年历史战争片，电影级质感，2.39:1变形宽银幕画幅，低饱和青灰冷色调",
		"【手持类】handheld camera / 手持摄影，3-5度倾斜角",
		"现代元素，鲜艳色彩，干净崭新的服装，棚拍感",
	}, "\n")

	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if err != nil {
		t.Fatalf("旧混合画风必须自动迁移，不得要求用户重填: %v", err)
	}
	if strings.Contains(receipt.Snapshot.StyleVisualPrompt, "手持") ||
		strings.Contains(receipt.Snapshot.StyleVisualPrompt, "现代元素") ||
		strings.Contains(receipt.Snapshot.StyleVisualPrompt, "2.39:1") {
		t.Fatalf("正向画风仍混入运镜/禁项/旧画幅: %q", receipt.Snapshot.StyleVisualPrompt)
	}
	if !strings.Contains(receipt.Snapshot.StyleMotionParameters, "手持摄影") {
		t.Fatalf("运镜参数未独立保存: %q", receipt.Snapshot.StyleMotionParameters)
	}
	if strings.Join(receipt.Snapshot.StyleForbiddenItems, "；") == "" {
		t.Fatalf("禁用项未独立保存: %+v", receipt.Snapshot.StyleForbiddenItems)
	}

	var state model.ManjuProjectStateSnapshot
	if err := f.db.First(&state, "project_id = ? AND lock_revision = ?", f.projectID, receipt.Revision).Error; err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(state.StateJSON), &payload); err != nil {
		t.Fatal(err)
	}
	worldAnchor, _ := payload["world_anchor"].(map[string]any)
	forbidden, _ := worldAnchor["forbidden_elements"].([]any)
	if len(forbidden) < 2 {
		t.Fatalf("P0 world_anchor 未继承独立禁用项: %+v", worldAnchor)
	}
}

func TestSaveLockFreezesStyleReferenceAndStrongPrompt(t *testing.T) {
	f := newFixture(t)
	resource := &model.Resource{
		ID: "style-reference-image", UserID: f.ownerID, Kind: "image", Status: model.ResourceStatusReady,
		Provider: "local", ObjectKey: "style-reference.png", MimeType: "image/png",
	}
	if err := f.db.Create(resource).Error; err != nil {
		t.Fatal(err)
	}
	input := validLock()
	input.StyleCardID = "sd-style-012"
	input.StyleReferenceResourceID = resource.ID
	input.StyleReferenceAnalysis = "低调偏暗 · 高反差 · 中等饱和 · 偏暖"
	input.StyleReferencePalette = "#C88A52 / #1D2433"

	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Snapshot.StyleReferenceStorageKey != "resource:"+resource.ID {
		t.Fatalf("style reference storage key = %q", receipt.Snapshot.StyleReferenceStorageKey)
	}
	for _, required := range []string{
		"【STYLE｜视觉风格 · 全片锁死】",
		"光比 [1:3, 1:6]",
		"参考图风格分析：低调偏暗 · 高反差 · 中等饱和 · 偏暖",
		"只迁移光线、色彩、材质、镜头与后期质感",
	} {
		if !strings.Contains(receipt.Snapshot.StyleImagePrompt, required) {
			t.Fatalf("P0 must freeze strong image style prompt missing %q:\n%s", required, receipt.Snapshot.StyleImagePrompt)
		}
	}
}

func TestSaveLockDoesNotTruncateStyleReferenceInputs(t *testing.T) {
	f := newFixture(t)
	resource := &model.Resource{
		ID: "style-reference-full", UserID: f.ownerID, Kind: "image", Status: model.ResourceStatusReady,
		Provider: "local", ObjectKey: "style-reference-full.png", MimeType: "image/png",
	}
	if err := f.db.Create(resource).Error; err != nil {
		t.Fatal(err)
	}
	input := validLock()
	input.StyleReferenceResourceID = resource.ID
	input.StyleReferenceAnalysis = strings.Repeat("参考图逐项分析；", 900) + "ANALYSIS_TAIL"
	input.StyleReferencePalette = strings.Repeat("#112233 / ", 300) + "PALETTE_TAIL"

	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Snapshot.StyleReferenceAnalysis != input.StyleReferenceAnalysis {
		t.Fatal("SaveLock truncated the complete style reference analysis")
	}
	if receipt.Snapshot.StyleReferencePalette != input.StyleReferencePalette {
		t.Fatal("SaveLock truncated the complete style reference palette")
	}
	if !strings.Contains(receipt.Snapshot.StyleImagePrompt, "ANALYSIS_TAIL") {
		t.Fatal("frozen style prompt is missing the full analysis tail")
	}
}

func TestProjectDetailRejectsOutsider(t *testing.T) {
	f := newFixture(t)

	_, err := f.service.ProjectDetail(f.outsider, f.projectID)

	if !errors.Is(err, manju.ErrNotFound) {
		t.Fatalf("外部用户读取项目必须返回不可见，得到 %v", err)
	}
}

func TestOwnerInvitesActiveProducer(t *testing.T) {
	f := newFixture(t)

	member, err := f.service.InviteMember(f.ownerID, f.projectID, manju.MemberInput{
		UserID: f.producer,
		Role:   model.ManjuMemberRoleProducer,
	})
	if err != nil {
		t.Fatalf("主理人邀请制片失败: %v", err)
	}
	if member.Role != model.ManjuMemberRoleProducer || member.Status != model.ManjuMemberStatusActive {
		t.Fatalf("邀请结果错误: %+v", member)
	}

	detail, err := f.service.ProjectDetail(f.producer, f.projectID)
	if err != nil {
		t.Fatalf("制片读取项目失败: %v", err)
	}
	if detail.AccessRole != model.ManjuMemberRoleProducer {
		t.Fatalf("制片权限应为 producer，得到 %s", detail.AccessRole)
	}
}

func TestProducerCanSaveP0ButWriterCannot(t *testing.T) {
	f := newFixture(t)
	if _, err := f.service.InviteMember(f.ownerID, f.projectID, manju.MemberInput{UserID: f.producer, Role: model.ManjuMemberRoleProducer}); err != nil {
		t.Fatalf("邀请制片失败: %v", err)
	}
	if _, err := f.service.InviteMember(f.ownerID, f.projectID, manju.MemberInput{UserID: f.writer, Role: model.ManjuMemberRoleWriter}); err != nil {
		t.Fatalf("邀请编剧失败: %v", err)
	}

	if _, err := f.service.SaveLock(f.producer, f.projectID, validLock()); err != nil {
		t.Fatalf("制片应可保存 P0: %v", err)
	}
	if _, err := f.service.SaveLock(f.writer, f.projectID, validLock()); !errors.Is(err, manju.ErrForbidden) {
		t.Fatalf("编剧保存 P0 必须被拒绝，得到 %v", err)
	}
}

func TestSavingP0CreatesNewRevisionAndSupersedesOld(t *testing.T) {
	f := newFixture(t)
	first, err := f.service.SaveLock(f.ownerID, f.projectID, validLock())
	if err != nil {
		t.Fatalf("首次保存 P0 失败: %v", err)
	}
	secondInput := validLock()
	secondInput.VideoModel = "seedance-2.5"
	secondInput.TargetDurationSeconds = 30
	second, err := f.service.SaveLock(f.ownerID, f.projectID, secondInput)
	if err != nil {
		t.Fatalf("第二次保存 P0 失败: %v", err)
	}
	if first.Revision != 1 || second.Revision != 2 {
		t.Fatalf("revision 必须递增，得到 %d -> %d", first.Revision, second.Revision)
	}

	var old model.ManjuLockReceipt
	if err := f.db.First(&old, "id = ?", first.ID).Error; err != nil {
		t.Fatalf("读取旧锁失败: %v", err)
	}
	if old.Status != model.ManjuLockStatusSuperseded {
		t.Fatalf("旧锁必须保留并标记 superseded，得到 %s", old.Status)
	}
	if second.Status != model.ManjuLockStatusLocked {
		t.Fatalf("新锁必须为 locked，得到 %s", second.Status)
	}
}

func TestDSeamIsOnlyRequiredForMultiSegmentTask(t *testing.T) {
	f := newFixture(t)
	sixtySeconds := validLock()
	sixtySeconds.TargetDurationSeconds = 60
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, sixtySeconds); !errors.Is(err, manju.ErrSeamRequired) {
		t.Fatalf("60 秒 Seedance 2.0 必须要求 D 锁，得到 %v", err)
	}

	sixtySeconds.SeamMode = "tail-frame-freeze"
	sixtySeconds.ConfirmedLocks = append(sixtySeconds.ConfirmedLocks, manju.LockSeamMode)
	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, sixtySeconds)
	if err != nil {
		t.Fatalf("补齐 D 锁后应可保存: %v", err)
	}
	if receipt.SegmentCount != 4 {
		t.Fatalf("60 秒 / 15 秒应推导 4 段，得到 %d", receipt.SegmentCount)
	}

	detail, err := f.service.ProjectDetail(f.ownerID, f.projectID)
	if err != nil {
		t.Fatalf("读取项目失败: %v", err)
	}
	if detail.Lock == nil || detail.Lock.CreatedAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("项目必须返回服务端保存的当前锁回执: %+v", detail.Lock)
	}
}

func TestReinviteUpdatesExistingMemberAndCanBeRemoved(t *testing.T) {
	f := newFixture(t)
	first, err := f.service.InviteMember(f.ownerID, f.projectID, manju.MemberInput{UserID: f.producer, Role: model.ManjuMemberRoleProducer})
	if err != nil {
		t.Fatalf("首次邀请失败: %v", err)
	}
	second, err := f.service.InviteMember(f.ownerID, f.projectID, manju.MemberInput{UserID: f.producer, Role: model.ManjuMemberRoleReviewer})
	if err != nil {
		t.Fatalf("重复邀请失败: %v", err)
	}
	if second.ID != first.ID || second.Role != model.ManjuMemberRoleReviewer {
		t.Fatalf("重复邀请必须更新原成员行，首次=%+v 第二次=%+v", first, second)
	}
	if err := f.service.RemoveMember(f.ownerID, f.projectID, second.ID); err != nil {
		t.Fatalf("按重复邀请返回的成员 ID 移除失败: %v", err)
	}
}
func TestListProjectsOnlyReturnsManjuProjects(t *testing.T) {
	f := newFixture(t)
	if err := f.db.Create(&model.Project{
		ID: "project-canvas-1", UserID: f.ownerID, Name: "普通画布项目", Type: "canvas", Status: model.ProjectStatusActive, Revision: 1,
	}).Error; err != nil {
		t.Fatalf("创建普通画布项目失败: %v", err)
	}

	items, err := f.service.ListProjects(f.ownerID)
	if err != nil {
		t.Fatalf("读取漫剧项目列表失败: %v", err)
	}
	if len(items) != 1 || items[0].ID != f.projectID || items[0].Type != "manju" {
		t.Fatalf("漫剧列表只能返回 manju 项目，得到 %+v", items)
	}
}
func TestNonManjuProjectCannotBeReadOrLockedThroughManjuAPI(t *testing.T) {
	f := newFixture(t)
	if err := f.db.Create(&model.Project{
		ID: "project-canvas-private", UserID: f.ownerID, Name: "普通画布项目", Type: "canvas", Status: model.ProjectStatusActive, Revision: 1,
	}).Error; err != nil {
		t.Fatalf("创建普通画布项目失败: %v", err)
	}

	if _, err := f.service.ProjectDetail(f.ownerID, "project-canvas-private"); !errors.Is(err, manju.ErrNotFound) {
		t.Fatalf("漫剧详情不能读取普通画布项目，得到 %v", err)
	}
	if _, err := f.service.SaveLock(f.ownerID, "project-canvas-private", validLock()); !errors.Is(err, manju.ErrNotFound) {
		t.Fatalf("P0 不能锁定普通画布项目，得到 %v", err)
	}
}
func seedEnabledModels(t *testing.T, db *gorm.DB) {
	t.Helper()
	channel := model.ModelChannel{ID: "channel-main", Scope: model.ChannelScopeSystem, Enabled: true, Name: "画布主渠道", APIFormat: "openai"}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("创建测试模型渠道失败: %v", err)
	}
	models := []model.ChannelModel{
		{ID: "cm-text", ChannelID: channel.ID, ModelKey: "text-pro", DisplayName: "文本 Pro", Capability: "text", Enabled: true},
		{ID: "cm-image", ChannelID: channel.ID, ModelKey: "image-pro", DisplayName: "图片 Pro", Capability: "image", Enabled: true},
		{ID: "cm-video", ChannelID: channel.ID, ModelKey: "video-pro", DisplayName: "视频 Pro", Capability: "video", Enabled: true},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("创建测试模型失败: %v", err)
	}
}

func TestProjectModelBindingsRequireEnabledModelsWithMatchingCapability(t *testing.T) {
	f := newFixture(t)
	seedEnabledModels(t, f.db)
	input := manju.ModelBindingInput{
		Text:  manju.ModelSelection{ChannelID: "channel-main", ModelKey: "text-pro"},
		Image: manju.ModelSelection{ChannelID: "channel-main", ModelKey: "image-pro"},
		Video: manju.ModelSelection{ChannelID: "channel-main", ModelKey: "video-pro"},
	}

	binding, err := f.service.SaveModels(f.ownerID, f.projectID, input)
	if err != nil {
		t.Fatalf("主理人保存已启用模型失败: %v", err)
	}
	if binding.Text.ModelKey != "text-pro" || binding.Image.ModelKey != "image-pro" || binding.Video.ModelKey != "video-pro" {
		t.Fatalf("模型绑定保存结果错误: %+v", binding)
	}

	detail, err := f.service.ProjectDetail(f.ownerID, f.projectID)
	if err != nil {
		t.Fatalf("读取模型绑定失败: %v", err)
	}
	if detail.Models == nil || detail.Models.Video.ModelKey != "video-pro" {
		t.Fatalf("项目详情必须返回已保存模型绑定: %+v", detail.Models)
	}

	wrongCapability := input
	wrongCapability.Text = manju.ModelSelection{ChannelID: "channel-main", ModelKey: "image-pro"}
	if _, err := f.service.SaveModels(f.ownerID, f.projectID, wrongCapability); !errors.Is(err, manju.ErrInvalidModelBinding) {
		t.Fatalf("文本槽不能绑定图片模型，得到 %v", err)
	}

	disabled := model.ChannelModel{ID: "cm-disabled", ChannelID: "channel-main", ModelKey: "text-disabled", Capability: "text", Enabled: false}
	if err := f.db.Create(&disabled).Error; err != nil {
		t.Fatalf("创建停用模型失败: %v", err)
	}
	disabledInput := input
	disabledInput.Text = manju.ModelSelection{ChannelID: "channel-main", ModelKey: "text-disabled"}
	if _, err := f.service.SaveModels(f.ownerID, f.projectID, disabledInput); !errors.Is(err, manju.ErrInvalidModelBinding) {
		t.Fatalf("停用模型不能被绑定，得到 %v", err)
	}
}

func TestProjectModelBindingsAllowSavingConfiguredCapabilitySubset(t *testing.T) {
	f := newFixture(t)
	seedEnabledModels(t, f.db)

	binding, err := f.service.SaveModels(f.ownerID, f.projectID, manju.ModelBindingInput{
		Text: manju.ModelSelection{ChannelID: "channel-main", ModelKey: "text-pro"},
	})
	if err != nil {
		t.Fatalf("只配置文本模型时也应允许保存: %v", err)
	}
	if binding.Text.ModelKey != "text-pro" || binding.Image.ModelKey != "" || binding.Video.ModelKey != "" {
		t.Fatalf("保存后的能力子集错误: %+v", binding)
	}

	if _, err := f.service.SaveModels(f.ownerID, f.projectID, manju.ModelBindingInput{
		Text: manju.ModelSelection{ChannelID: "channel-main"},
	}); !errors.Is(err, manju.ErrInvalidModelBinding) {
		t.Fatalf("只填写渠道而不填写模型标识时必须拒绝，得到 %v", err)
	}
}

func TestWriterCannotChangeProjectModelBindings(t *testing.T) {
	f := newFixture(t)
	seedEnabledModels(t, f.db)
	if _, err := f.service.InviteMember(f.ownerID, f.projectID, manju.MemberInput{UserID: f.writer, Role: model.ManjuMemberRoleWriter}); err != nil {
		t.Fatalf("邀请编剧失败: %v", err)
	}
	input := manju.ModelBindingInput{
		Text:  manju.ModelSelection{ChannelID: "channel-main", ModelKey: "text-pro"},
		Image: manju.ModelSelection{ChannelID: "channel-main", ModelKey: "image-pro"},
		Video: manju.ModelSelection{ChannelID: "channel-main", ModelKey: "video-pro"},
	}
	if _, err := f.service.SaveModels(f.writer, f.projectID, input); !errors.Is(err, manju.ErrForbidden) {
		t.Fatalf("编剧不能改项目模型绑定，得到 %v", err)
	}
}

// deletion regression tests are added below.
func TestOwnerCanDeleteManjuProjectAndRelatedRecords(t *testing.T) {
	f := newFixture(t)
	seedManjuDeleteRecords(t, f)

	if err := f.service.DeleteProject(f.ownerID, f.projectID); err != nil {
		t.Fatalf("主理人删除漫剧项目失败: %v", err)
	}

	assertNoManjuProjectRecords(t, f.db, f.projectID)

	var taskCount int64
	if err := f.db.Model(&model.Task{}).Where("project_id = ?", f.projectID).Count(&taskCount).Error; err != nil {
		t.Fatal(err)
	}
	if taskCount != 1 {
		t.Fatalf("已完成任务历史必须保留，count=%d", taskCount)
	}
}

func TestOnlyOwnerCanDeleteManjuProject(t *testing.T) {
	f := newFixture(t)
	if _, err := f.service.InviteMember(f.ownerID, f.projectID, manju.MemberInput{UserID: f.writer, Role: model.ManjuMemberRoleWriter}); err != nil {
		t.Fatalf("邀请编剧失败: %v", err)
	}

	if err := f.service.DeleteProject(f.writer, f.projectID); !errors.Is(err, manju.ErrForbidden) {
		t.Fatalf("非主理人删除必须被拒绝，得到 %v", err)
	}
	if err := f.service.DeleteProject(f.outsider, f.projectID); !errors.Is(err, manju.ErrNotFound) {
		t.Fatalf("外部用户删除必须隐藏项目，得到 %v", err)
	}
}

func TestDeleteManjuProjectRejectsActiveTasks(t *testing.T) {
	f := newFixture(t)
	if err := f.db.Create(&model.Task{ID: "task-active-delete", UserID: f.producer, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusRunning}).Error; err != nil {
		t.Fatalf("创建运行中任务失败: %v", err)
	}

	if err := f.service.DeleteProject(f.ownerID, f.projectID); !errors.Is(err, manju.ErrHasActiveTasks) {
		t.Fatalf("运行中任务必须阻止删除，得到 %v", err)
	}

	var count int64
	if err := f.db.Model(&model.Project{}).Where("id = ?", f.projectID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("删除被拒绝后项目必须保留，count=%d", count)
	}
}
func seedManjuDeleteRecords(t *testing.T, f fixture) {
	t.Helper()
	records := []any{
		&model.ManjuProjectMember{ID: "delete-member", ProjectID: f.projectID, UserID: f.producer, Role: model.ManjuMemberRoleProducer, Status: model.ManjuMemberStatusActive},
		&model.ManjuLockReceipt{ID: "delete-lock", ProjectID: f.projectID, Revision: 1, Status: model.ManjuLockStatusLocked, PayloadJSON: "{}", ContentHash: "hash", CreatedBy: f.ownerID},
		&model.ManjuProjectModelBinding{ID: "delete-binding", ProjectID: f.projectID, TextChannelID: "channel", TextModelKey: "text"},
		&model.ManjuScriptRun{ID: "delete-script-run", ProjectID: f.projectID, Branch: model.ManjuScriptBranchOriginal, Mode: model.ManjuScriptModeStepwise, Status: model.ManjuScriptRunStatusActive, CurrentStage: model.ManjuScriptStageSourceRegistration, CreatedBy: f.ownerID},
		&model.ManjuScriptStage{ID: "delete-script-stage", RunID: "delete-script-run", Key: model.ManjuScriptStageSourceRegistration, Title: "来源登记", Status: model.ManjuScriptStageStatusNotStarted},
		&model.ManjuScriptSource{ID: "delete-script-source", ProjectID: f.projectID, RunID: "delete-script-run", Branch: model.ManjuScriptBranchOriginal, Title: "来源", ContentHash: "hash", CreatedBy: f.ownerID},
		&model.ManjuScriptArtifact{ID: "delete-script-artifact", ProjectID: f.projectID, RunID: "delete-script-run", StageID: "delete-script-stage", Kind: "SOURCE_MANIFEST.json", Content: "{}", ContentHash: "hash", CreatedBy: f.ownerID},
		&model.ManjuAssetRun{ID: "delete-asset-run", ProjectID: f.projectID, ScriptRunID: "delete-script-run", AssetMode: model.ManjuAssetModeM1, Status: model.ManjuAssetRunStatusActive, CreatedBy: f.ownerID},
		&model.ManjuAssetStage{ID: "delete-asset-stage", RunID: "delete-asset-run", Key: model.ManjuAssetStageExtraction, Title: "实体提取", Status: model.ManjuAssetStageStatusNotStarted},
		&model.ManjuAssetArtifact{ID: "delete-asset-artifact", ProjectID: f.projectID, RunID: "delete-asset-run", StageID: "delete-asset-stage", Kind: "ENTITY_INVENTORY.md", Content: "x", ContentHash: "hash", CreatedBy: f.ownerID},
		&model.ManjuAssetReference{ID: "delete-asset-reference", ProjectID: f.projectID, RunID: "delete-asset-run", Code: "C01", AssetType: "character", ResourceID: "resource-1", Status: model.ManjuAssetReferenceStatusReady, CreatedBy: f.ownerID},
		&model.ManjuProductionRun{ID: "delete-production-run", ProjectID: f.projectID, AssetRunID: "delete-asset-run", Status: model.ManjuProductionRunStatusActive, CreatedBy: f.ownerID},
		&model.ManjuProductionStage{ID: "delete-production-stage", RunID: "delete-production-run", Key: model.ManjuProductionStageStoryboard, Title: "导演故事板", Status: model.ManjuProductionStageStatusNotStarted},
		&model.ManjuProductionArtifact{ID: "delete-production-artifact", ProjectID: f.projectID, RunID: "delete-production-run", StageID: "delete-production-stage", Kind: "DIRECTOR_STORYBOARD.md", Content: "x", ContentHash: "hash", CreatedBy: f.ownerID},
		&model.ManjuProductionImage{ID: "delete-production-image", ProjectID: f.projectID, RunID: "delete-production-run", StageID: "delete-production-stage", Position: 1, SegmentCode: "S01", Prompt: "x", Status: model.ManjuProductionImageStatusReady, CreatedBy: f.ownerID},
		&model.ManjuProductionVideo{ID: "delete-production-video", ProjectID: f.projectID, RunID: "delete-production-run", StageID: "delete-production-stage", Position: 1, SegmentCode: "S01", Prompt: "x", Status: model.ManjuProductionVideoStatusReady, CreatedBy: f.ownerID},
		&model.Task{ID: "task-completed-delete", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusSucceeded},
	}
	for _, record := range records {
		if err := f.db.Create(record).Error; err != nil {
			t.Fatalf("写入删除测试记录失败: %v", err)
		}
	}
}

func assertNoManjuProjectRecords(t *testing.T, db *gorm.DB, projectID string) {
	t.Helper()
	checks := []struct {
		name  string
		model any
		where string
		value string
	}{
		{name: "project", model: &model.Project{}, where: "id = ?", value: projectID},
		{name: "member", model: &model.ManjuProjectMember{}, where: "project_id = ?", value: projectID},
		{name: "lock", model: &model.ManjuLockReceipt{}, where: "project_id = ?", value: projectID},
		{name: "binding", model: &model.ManjuProjectModelBinding{}, where: "project_id = ?", value: projectID},
		{name: "script run", model: &model.ManjuScriptRun{}, where: "project_id = ?", value: projectID},
		{name: "script stage", model: &model.ManjuScriptStage{}, where: "run_id = ?", value: "delete-script-run"},
		{name: "script source", model: &model.ManjuScriptSource{}, where: "project_id = ?", value: projectID},
		{name: "script artifact", model: &model.ManjuScriptArtifact{}, where: "project_id = ?", value: projectID},
		{name: "asset run", model: &model.ManjuAssetRun{}, where: "project_id = ?", value: projectID},
		{name: "asset stage", model: &model.ManjuAssetStage{}, where: "run_id = ?", value: "delete-asset-run"},
		{name: "asset artifact", model: &model.ManjuAssetArtifact{}, where: "project_id = ?", value: projectID},
		{name: "asset reference", model: &model.ManjuAssetReference{}, where: "project_id = ?", value: projectID},
		{name: "production run", model: &model.ManjuProductionRun{}, where: "project_id = ?", value: projectID},
		{name: "production stage", model: &model.ManjuProductionStage{}, where: "run_id = ?", value: "delete-production-run"},
		{name: "production artifact", model: &model.ManjuProductionArtifact{}, where: "project_id = ?", value: projectID},
		{name: "production image", model: &model.ManjuProductionImage{}, where: "project_id = ?", value: projectID},
		{name: "production video", model: &model.ManjuProductionVideo{}, where: "project_id = ?", value: projectID},
	}
	for _, check := range checks {
		var count int64
		if err := db.Model(check.model).Where(check.where, check.value).Count(&count).Error; err != nil {
			t.Fatalf("统计 %s 失败: %v", check.name, err)
		}
		if count != 0 {
			t.Fatalf("%s = %d, want 0", check.name, count)
		}
	}
}
