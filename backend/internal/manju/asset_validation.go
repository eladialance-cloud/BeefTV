package manju

import (
	"fmt"
	"regexp"
	"strings"
)

type requiredAssetReference struct {
	Code       string
	ResourceID string
}

var imageNumberPattern = regexp.MustCompile(`(?i)(?:图|图片|参考图)\s*[0-9０-９]+|image\s*[0-9]+`)

func validateAssetDelivery(mode, content string, references []requiredAssetReference) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("资产交付稿为空")
	}
	declaration := "素材形态：" + strings.ToUpper(strings.TrimSpace(mode))
	firstLines := content
	if newline := strings.IndexByte(firstLines, '\n'); newline >= 0 {
		firstLines = firstLines[:newline]
	}
	if !strings.Contains(firstLines, declaration) {
		return fmt.Errorf("资产交付稿首行必须声明%s", declaration)
	}

	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case AssetModeM0:
		if imageNumberPattern.MatchString(content) || strings.Contains(content, "资产图册") {
			return fmt.Errorf("M0 外观档案不得出现图号、参考图槽位或《资产图册》命名")
		}
	case AssetModeM1:
		if !strings.Contains(content, "素材接线单") {
			return fmt.Errorf("M1 交付必须包含《素材接线单》")
		}
		if !imageNumberPattern.MatchString(content) {
			return fmt.Errorf("M1 交付必须显式登记图号")
		}
		if !strings.Contains(content, "只控制") || !strings.Contains(content, "不控制") {
			return fmt.Errorf("M1 素材接线单必须逐号写明只控制什么、不控制什么")
		}
	case AssetModeM2:
		if !strings.Contains(content, "资产图册") {
			return fmt.Errorf("M2 交付必须使用《资产图册》")
		}
		registered := make(map[string]bool, len(references))
		for _, reference := range references {
			if strings.TrimSpace(reference.ResourceID) != "" {
				registered[strings.TrimSpace(reference.Code)] = true
			}
		}
		for _, reference := range references {
			code := strings.TrimSpace(reference.Code)
			if code == "" {
				continue
			}
			if !registered[code] {
				return fmt.Errorf("M2 资产 %s 尚未登记资产参考图", code)
			}
			if !strings.Contains(content, code) {
				return fmt.Errorf("M2 资产图册缺少资产编码 %s", code)
			}
		}
	default:
		return fmt.Errorf("不支持的素材形态 %s", mode)
	}
	return nil
}
