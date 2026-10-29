package manju

import (
	"strings"
	"testing"
)

func TestSplitNovelChaptersUsesNaturalChapterHeadings(t *testing.T) {
	content := `第一章 雨夜
林澈推开旧宅的门。

第二章 旧照片
照片背面写着一个日期。

第三章 追踪
走廊尽头传来脚步声。`

	chapters := splitNovelChapters(content)
	if len(chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d: %+v", len(chapters), chapters)
	}
	if chapters[0].Title != "第一章 雨夜" || !strings.Contains(chapters[0].Content, "旧宅") {
		t.Fatalf("first chapter not parsed: %+v", chapters[0])
	}
	if chapters[2].Title != "第三章 追踪" || !strings.Contains(chapters[2].Content, "脚步声") {
		t.Fatalf("last chapter not parsed: %+v", chapters[2])
	}
}

func TestSplitNovelChaptersFallsBackToOneChapter(t *testing.T) {
	chapters := splitNovelChapters("没有章节标题的一段完整小说正文。")
	if len(chapters) != 1 {
		t.Fatalf("expected one fallback chapter, got %d", len(chapters))
	}
	if chapters[0].Title != "全文" || !strings.Contains(chapters[0].Content, "完整小说") {
		t.Fatalf("fallback chapter not preserved: %+v", chapters[0])
	}
}
