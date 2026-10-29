package manju

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestInheritedAssetBaselinePromptOnlyIncludesStructuredArtifacts(t *testing.T) {
	prompt := inheritedAssetBaselinePrompt([]model.ManjuAssetArtifact{
		{Kind: model.ManjuAssetArtifactExtraction, Version: 2, Content: "ENTITY_BASELINE"},
		{Kind: model.ManjuAssetArtifactManifest, Version: 3, Content: "MANIFEST_BASELINE"},
		{Kind: model.ManjuAssetArtifactM1, Version: 4, Content: "#### 图1 · 旧编译资产"},
	})

	for _, required := range []string{"ENTITY_BASELINE", "MANIFEST_BASELINE", "继承资产基线"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("inherited baseline is missing %q:\n%s", required, prompt)
		}
	}
	if strings.Contains(prompt, "#### 图1") || strings.Contains(prompt, model.ManjuAssetArtifactM1) {
		t.Fatalf("inherited baseline must exclude compiled delivery artifacts:\n%s", prompt)
	}
}
