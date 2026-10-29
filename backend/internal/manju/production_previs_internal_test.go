package manju

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestProductionContractArchiveIncludesP45(t *testing.T) {
	archive := buildProductionContractArchive(
		nil,
		nil,
		nil,
		&model.ManjuProductionArtifact{
			Kind:    model.ManjuProductionArtifactPrevis,
			Version: 1,
			Content: "# 线上漫剧项目 · 预演故事板\n\n## 段 1\n**本段要验什么**：站位\n",
		},
		"# P5 当前质检交付\n",
	)
	if !strings.Contains(archive, "# P4.5 · PREVIS_STORYBOARD.md v1") {
		t.Fatalf("P5 契约归档必须包含 P4.5 工件:\n%s", archive)
	}
}
