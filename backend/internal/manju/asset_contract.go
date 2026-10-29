package manju

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed output_contracts/asset-block-v1.json
var assetBlockContractJSON []byte

var (
	assetBlockContractOnce sync.Once
	assetBlockContractData AssetBlockContract
	assetBlockContractErr  error

	ErrAssetBlockContract = errors.New("P2 资产块不符合共享输出契约")
)

type AssetBlockContract struct {
	SchemaVersion string                            `json:"schemaVersion"`
	RulesVersion  string                            `json:"rulesVersion"`
	Source        AssetBlockContractSource          `json:"source"`
	Block         AssetBlockContractBlock           `json:"block"`
	Kinds         map[string]AssetBlockKindContract `json:"kinds"`
}

type AssetBlockContractSource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type AssetBlockContractBlock struct {
	HeadingTemplate          string   `json:"headingTemplate"`
	HeadingLevels            []int    `json:"headingLevels"`
	HeadingPrefix            string   `json:"headingPrefix"`
	Examples                 []string `json:"examples"`
	AspectRatio              string   `json:"aspectRatio"`
	SelfContained            bool     `json:"selfContained"`
	ForbiddenCrossReferences []string `json:"forbiddenCrossReferences"`
	MinBlocks                int      `json:"minBlocks"`
}

type AssetBlockKindContract struct {
	Label        string   `json:"label"`
	All          []string `json:"all"`
	Any          []string `json:"any,omitempty"`
	Forbidden    []string `json:"forbidden,omitempty"`
	PromptFields []string `json:"promptFields,omitempty"`
	FieldAliases []string `json:"fieldAliases,omitempty"`
}

func loadAssetBlockContract() (AssetBlockContract, error) {
	assetBlockContractOnce.Do(func() {
		assetBlockContractErr = json.Unmarshal(assetBlockContractJSON, &assetBlockContractData)
		if assetBlockContractErr == nil && assetBlockContractData.SchemaVersion == "" {
			assetBlockContractErr = errors.New("P2 资产块共享输出契约为空")
		}
	})
	return assetBlockContractData, assetBlockContractErr
}

func mustLoadAssetBlockContract() AssetBlockContract {
	contract, err := loadAssetBlockContract()
	if err != nil {
		panic(err)
	}
	return contract
}

func assetBlockPromptContract(contract AssetBlockContract) string {
	var out strings.Builder
	out.WriteString("\n\n【资产块标题与机检硬格式】\n")
	fmt.Fprintf(&out, "每个资产块都必须单独使用 Markdown 标题，标准格式为：\n%s\n", contract.Block.HeadingTemplate)
	out.WriteString("上述标题是占位模板，不是真实资产。不得把示例原样写入正文，必须替换为本次资产清单中的真实图号和资产名称。\n")
	fmt.Fprintf(
		&out,
		"标题必须是 Markdown %d~%d 级标题，且以“%s”开头。不能只用表格、代码块或 JSON 代替资产块标题。\n",
		contract.Block.HeadingLevels[0],
		contract.Block.HeadingLevels[1],
		contract.Block.HeadingPrefix,
	)
	if contract.Block.SelfContained {
		fmt.Fprintf(
			&out,
			"每个资产块必须自包含并写明 %s，禁止“%s”等跨条引用。\n",
			contract.Block.AspectRatio,
			strings.Join(contract.Block.ForbiddenCrossReferences, "、"),
		)
	}
	for _, key := range []string{"character", "sceneMaster", "sceneGrid", "prop", "groupAsset", "complexScene"} {
		kind, ok := contract.Kinds[key]
		if !ok {
			continue
		}
		fmt.Fprintf(&out, "%s必须包含：%s。", kind.Label, strings.Join(kind.All, "、"))
		if len(kind.Any) > 0 {
			fmt.Fprintf(&out, "需二选一的字段必须原文写入：%s。", strings.Join(kind.Any, "或"))
		}
		for _, field := range kind.PromptFields {
			out.WriteString(field)
		}
		if len(kind.Forbidden) > 0 {
			fmt.Fprintf(&out, "禁止使用“%s”旧称。", strings.Join(kind.Forbidden, "、"))
		}
		out.WriteString("\n")
	}
	out.WriteString(assetBlockSkeletonContract(contract))
	return strings.TrimRight(out.String(), "\n")
}

func assetBlockSkeletonContract(contract AssetBlockContract) string {
	character := contract.Kinds["character"]
	sceneGrid := contract.Kinds["sceneGrid"]
	prop := contract.Kinds["prop"]
	group := contract.Kinds["groupAsset"]
	return fmt.Sprintf(`

【角色四格块固定骨架】
#### 图N · 名称（角色四格）
16:9；%s。

【场景九宫格块固定骨架】
#### 图N · 名称（场景九宫格）
16:9。
必须逐字包含：%s。
每个九宫格块必须独立完整复述全部字段，不得引用前文。
资产块正文禁止出现：按上述、同上、见母版、沿用上文、开关＝关、后续集数延续。
官方规则文档中的操作说明、开关状态、后续集数处理不得写入资产块正文。

【群像资产卡固定骨架】
#### 图N · 名称（群像资产卡）
16:9。
【群像资产 GRP-N】群体名
全图共 3 格，3 格横向并排。
必须逐字包含：%s。

【道具设定图固定骨架】
#### 图N · 名称（道具设定图）
16:9。
道具设定图。
必须逐字包含：%s。

固定字段名必须逐字保留，不得改写、合并或省略。`,
		strings.Join(character.All, "；"),
		strings.Join(sceneGrid.All, "；"),
		strings.Join(group.All, "；"),
		strings.Join(prop.All, "；"),
	)
}

func (contract AssetBlockContract) validateHeadingStructure(content string) error {
	if len(contract.Block.HeadingLevels) != 2 {
		return fmt.Errorf("%w：标题层级配置无效", ErrAssetBlockContract)
	}
	pattern := regexp.MustCompile(fmt.Sprintf(
		`(?m)^#{%d,%d}\s*\*{0,2}\s*图\s*[0-9]+\s*\*{0,2}\s*[·:：\-—]?\s*.*$`,
		contract.Block.HeadingLevels[0],
		contract.Block.HeadingLevels[1],
	))
	if pattern.MatchString(content) {
		return nil
	}
	return fmt.Errorf(
		"%w：未识别到资产块标题。必须使用 `%s`，标题层级为 Markdown %d~%d 级，且以“%s”开头。",
		ErrAssetBlockContract,
		contract.Block.HeadingTemplate,
		contract.Block.HeadingLevels[0],
		contract.Block.HeadingLevels[1],
		contract.Block.HeadingPrefix,
	)
}
