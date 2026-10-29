package manju_test

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestP2CanvasPackageRejectsM0BecauseItHasNoImagePrompts(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	if err := f.db.Model(&model.ManjuAssetRun{}).Where("id = ?", assetRunID).Update("asset_mode", "M0").Error; err != nil {
		t.Fatal(err)
	}
	artifact := &model.ManjuAssetArtifact{}
	if err := f.db.First(artifact, "run_id = ? AND kind = ?", assetRunID, model.ManjuAssetArtifactM1).Error; err != nil {
		t.Fatal(err)
	}
	artifact.Kind = model.ManjuAssetArtifactM0
	artifact.Content = `> 素材形态：M0（提示词 · 不使用参考图 · 一致性靠文字锁）
# CHR-林澈 外观档案
28~32岁男性档案修复师，瘦高体型，冷白肤色，黑色短发，深灰衬衫与旧呢外套。

# SCN-旧宅 场景档案
民国旧宅门厅，木门掉漆，青砖地面，阴天冷光，纯场景无人。

## 4. M0 文字锁接线摘要
复用 CHR-林澈 与 SCN-旧宅 的文字锁，不新增资产。`
	if err := f.db.Save(artifact).Error; err != nil {
		t.Fatal(err)
	}

	pkg, err := f.service.CanvasPackage(f.ownerID, f.projectID, manju.CanvasPackageStageP2Assets, assetRunID)
	if !errors.Is(err, manju.ErrCanvasPackageUnready) || pkg != nil {
		t.Fatalf("M0 只有文字外观档案，没有出图提示词，不得导入画布生成图片: pkg=%+v err=%v", pkg, err)
	}
}
