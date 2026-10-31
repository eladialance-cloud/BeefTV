package manju

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestAssetBlockContractPinsOfficialChecker(t *testing.T) {
	contract, err := loadAssetBlockContract()
	if err != nil {
		t.Fatal(err)
	}
	if contract.RulesVersion != manjuSkillVersion {
		t.Fatalf("contract rules version mismatch: %s", contract.RulesVersion)
	}
	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(contract.Source.Path)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := fmt.Sprintf("%x", sum); got != contract.Source.SHA256 {
		t.Fatalf("contract is stale for official checker: got=%s want=%s", got, contract.Source.SHA256)
	}
}

func TestAssetBlockContractWitnessPassesOfficialChecker(t *testing.T) {
	contract, err := loadAssetBlockContract()
	if err != nil {
		t.Fatal(err)
	}
	witness := buildAssetBlockContractWitness(contract)
	result, err := defaultManjuSkillRuntime.ValidateAssetBlocks(witness)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("contract witness must pass the official checker: %+v\n%s", result, witness)
	}
}

func TestAssetBlockCheckerRecognizesGroupAndComplexSceneTypes(t *testing.T) {
	contract, err := loadAssetBlockContract()
	if err != nil {
		t.Fatal(err)
	}
	group, ok := contract.Kinds["groupAsset"]
	if !ok {
		t.Fatal("asset contract is missing groupAsset")
	}
	witness := strings.Join([]string{
		"#### 图1 · 测试角色（角色四格）",
		"16:9 角色四格资产板。",
		strings.Join(contract.Kinds["character"].All, "；"),
		"",
		"#### 图2 · 测试群像（群像资产卡）",
		"16:9，全图共 3 格横向并排。",
		"【群像资产 GRP-01】测试群像",
		strings.Join(group.All, "；"),
		"B 级群体格内不刻画个体五官。",
		"本群像卡不采用「全图共 4 格」规则，也不包含 fifth panel。",
		"",
		"#### 图3 · 测试战场（战斗站位版）",
		"16:9 复杂场景调度图；战斗站位版；标注机位、站位、轴线与禁入画区。",
	}, "\n")
	result, err := defaultManjuSkillRuntime.ValidateAssetBlocks(witness)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("group and complex scene witness must pass: %+v\n%s", result, witness)
	}
	for _, expected := range []string{"图2   群像", "图3   复杂场景"} {
		if !strings.Contains(result.Output, expected) {
			t.Fatalf("asset type was not classified as %q:\n%s", expected, result.Output)
		}
	}
	if strings.Contains(result.Output, "A7 图2") || strings.Contains(result.Output, "A7 图3") {
		t.Fatalf("group/complex scene must not fall back to unknown A7:\n%s", result.Output)
	}
}

func TestAssetBlockCheckerStillRejectsRoleWithoutVoiceQuickLabel(t *testing.T) {
	contract, err := loadAssetBlockContract()
	if err != nil {
		t.Fatal(err)
	}
	roleFields := make([]string, 0, len(contract.Kinds["character"].All))
	for _, field := range contract.Kinds["character"].All {
		if field != "≤8 词音色速览标签" {
			roleFields = append(roleFields, field)
		}
	}
	witness := strings.Join([]string{
		"#### 图1 · 测试角色（角色四格）",
		"16:9 角色四格资产板。",
		strings.Join(roleFields, "；"),
	}, "\n")
	result, err := defaultManjuSkillRuntime.ValidateAssetBlocks(witness)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Output, "≤8 词音色速览标签") {
		t.Fatalf("role missing the exact quick-label field must still fail: %+v\n%s", result, witness)
	}
}

func TestAssetCompilePromptUsesOfficialContextWithoutPlatformHeadingOverride(t *testing.T) {
	prompt := buildAssetStagePrompt(
		"测试项目",
		model.ManjuAssetRun{AssetMode: AssetModeM1Value},
		model.ManjuAssetStage{Key: AssetStageCompile, Title: "资产交付编译"},
		model.ManjuScriptRun{},
		nil,
		nil,
		nil,
		nil,
		"",
		"",
	)
	for _, forbidden := range []string{
		"【资产块标题与机检硬格式】",
		"#### 图N · 名称（角色四格）",
		"资产编码：CHR-…",
		"覆盖前文任何章节编号式标题要求",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("asset compile prompt must not override official rules with platform-only format %q:\n%s", forbidden, prompt)
		}
	}
}

func buildAssetBlockContractWitness(contract AssetBlockContract) string {
	return strings.Join([]string{
		"#### 图1 · 测试角色（角色四格）",
		"16:9 角色四格资产板。",
		strings.Join(contract.Kinds["character"].All, "；"),
		"",
		"#### 图2 · 测试场景（场景母版）",
		"16:9；" + strings.Join(contract.Kinds["sceneMaster"].All, "；"),
		"",
		"#### 图3 · 测试场景（场景九宫格）",
		"16:9；" + strings.Join(contract.Kinds["sceneGrid"].All, "；"),
		"【参考场景锁定模式 · 模式A（仅允许摄影机移动）】",
		"",
		"#### 图4 · 测试道具（道具设定图）",
		"16:9；" + strings.Join(contract.Kinds["prop"].All, "；"),
	}, "\n")
}
