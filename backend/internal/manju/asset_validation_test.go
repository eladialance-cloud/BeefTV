package manju

import (
	"strings"
	"testing"
)

func TestM0DeliveryRejectsImageNumbers(t *testing.T) {
	err := validateAssetDelivery(AssetModeM0, "> 素材形态：M0（纯提示词）\n\n林澈，见图1。", nil)
	if err == nil || !strings.Contains(err.Error(), "图号") {
		t.Fatalf("M0 出现图号必须被拒绝，得到 %v", err)
	}
}

func TestM1DeliveryRequiresWiringSheet(t *testing.T) {
	err := validateAssetDelivery(AssetModeM1, "> 素材形态：M1（编号占位）\n\n# 资产编号册\n图1 = 林澈\n图2 = 旧宅", nil)
	if err == nil || !strings.Contains(err.Error(), "接线单") {
		t.Fatalf("M1 缺少素材接线单必须被拒绝，得到 %v", err)
	}
}

func TestM1DeliveryDoesNotRequireBeefTVTransportEnvelope(t *testing.T) {
	err := validateAssetDelivery(AssetModeM1, "> 素材形态：M1（编号占位）\n\n# 资产编号册\n图1 = 林澈\n\n## 素材接线单\n| 图号 | 用途 | 只控制什么 | 不控制什么 |\n| 图1 | 角色 | 脸型 | 动作 |", nil)
	if err != nil {
		t.Fatalf("official M1 delivery must pass without a BeefTV transport envelope: %v", err)
	}
}

func TestAssetDeliveryRequiresModeDeclaration(t *testing.T) {
	err := validateAssetDelivery(AssetModeM1, "# 资产编号册\n\n## 素材接线单\n| 图1 | 只控制脸型 | 不控制姿态 |", nil)
	if err == nil || !strings.Contains(err.Error(), "素材形态") {
		t.Fatalf("交付稿首部必须声明素材形态，得到 %v", err)
	}
}

func TestM2DeliveryRequiresAllRegisteredCodes(t *testing.T) {
	err := validateAssetDelivery(
		AssetModeM2,
		"> 素材形态：M2（已入库）\n\n# 资产图册\nCHR-林澈\nSCN-旧宅"+canvasTaskBlockForValidationTest(),
		[]requiredAssetReference{{Code: "CHR-林澈", ResourceID: "res-1"}, {Code: "SCN-旧宅"}},
	)
	if err == nil || !strings.Contains(err.Error(), "SCN-旧宅") {
		t.Fatalf("M2 缺少已登记场景参考图必须被拒绝，得到 %v", err)
	}
}

func canvasTaskBlockForValidationTest() string {
	return "\n\n<!-- MANJU_CANVAS_TASKS_JSON\n" +
		`{"version":1,"tasks":[{"code":"图1","assetCode":"CHR-林澈","title":"角色图","category":"character","prompt":"完整角色四格提示词"}]}` +
		"\n-->"
}
