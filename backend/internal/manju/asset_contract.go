package manju

import (
	_ "embed"
	"encoding/json"
	"errors"
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
