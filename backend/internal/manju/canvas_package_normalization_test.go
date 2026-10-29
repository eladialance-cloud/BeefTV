package manju

import (
	"strings"
	"testing"
)

func TestNormalizeCanvasImportReferenceCodeCanonicalizesLegacyCNH(t *testing.T) {
	if got := normalizeCanvasImportReferenceCode("CNH-林澈"); got != "CHR-林澈" {
		t.Fatalf("legacy CNH asset code = %q, want %q", got, "CHR-林澈")
	}
}

func TestPlotGridPromptPreservesCompleteLockedStoryboardTable(t *testing.T) {
	content := "# P3\n\n## 分镜总表\n" +
		strings.Repeat("| 段1 | 镜1 | 中景 | 林澈推门 |\n", 600) +
		"STORYBOARD_TAIL_MARKER\n\n## 故事板表\n"
	prompt := buildPlotGridCanvasPrompt(content, "场1", "四格连续叙事", "4格(2×2)", LockSnapshot{AspectRatio: "9:16"})
	if !strings.Contains(prompt, "STORYBOARD_TAIL_MARKER") {
		t.Fatal("plot-grid prompt truncated the locked P3 storyboard table")
	}
}
