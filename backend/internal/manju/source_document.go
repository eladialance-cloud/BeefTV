package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"github.com/google/uuid"
)

var novelChapterHeading = regexp.MustCompile(`^(第[0-9一二三四五六七八九十百千万零〇两]+[章节卷回部集篇].*|(?i:chapter)\s+\d+.*)$`)

type parsedNovelChapter struct {
	Title   string
	Content string
}

type SourceDocumentInput struct {
	Title              string `json:"title"`
	AuthorizationScope string `json:"authorizationScope"`
	FileName           string `json:"fileName"`
	Content            string `json:"content"`
}

type SourceDocumentView struct {
	ID                 string    `json:"id"`
	ProjectID          string    `json:"projectId"`
	Title              string    `json:"title"`
	AuthorizationScope string    `json:"authorizationScope"`
	FileName           string    `json:"fileName"`
	ContentHash        string    `json:"contentHash"`
	CharacterCount     int64     `json:"characterCount"`
	ChapterCount       int       `json:"chapterCount"`
	CreatedBy          string    `json:"createdBy"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type SourceChapterView struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"projectId"`
	DocumentID     string    `json:"documentId"`
	Position       int       `json:"position"`
	Title          string    `json:"title"`
	RangeLabel     string    `json:"rangeLabel"`
	ContentHash    string    `json:"contentHash"`
	CharacterCount int64     `json:"characterCount"`
	CreatedAt      time.Time `json:"createdAt"`
}

type SourceDocumentDetail struct {
	Document SourceDocumentView  `json:"document"`
	Chapters []SourceChapterView `json:"chapters"`
}

func splitNovelChapters(content string) []parsedNovelChapter {
	content = strings.ReplaceAll(strings.TrimSpace(content), "\r\n", "\n")
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	result := make([]parsedNovelChapter, 0)
	var title string
	var body []string
	sawHeading := false
	flush := func() {
		text := strings.TrimSpace(strings.Join(body, "\n"))
		if text == "" {
			return
		}
		if title == "" {
			if sawHeading {
				title = "前言"
			} else {
				title = "全文"
			}
		}
		result = append(result, parsedNovelChapter{Title: title, Content: text})
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if novelChapterHeading.MatchString(heading) {
			sawHeading = true
			flush()
			title = heading
			body = nil
			continue
		}
		body = append(body, raw)
	}
	flush()
	if len(result) == 0 {
		return []parsedNovelChapter{{Title: "全文", Content: content}}
	}
	return result
}

func contentHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func sourceDocumentView(document model.ManjuSourceDocument) SourceDocumentView {
	return SourceDocumentView{
		ID: document.ID, ProjectID: document.ProjectID, Title: document.Title,
		AuthorizationScope: document.AuthorizationScope, FileName: document.FileName,
		ContentHash: document.ContentHash, CharacterCount: document.CharacterCount,
		ChapterCount: document.ChapterCount, CreatedBy: document.CreatedBy,
		CreatedAt: document.CreatedAt, UpdatedAt: document.UpdatedAt,
	}
}

func sourceChapterView(chapter model.ManjuSourceChapter) SourceChapterView {
	return SourceChapterView{
		ID: chapter.ID, ProjectID: chapter.ProjectID, DocumentID: chapter.DocumentID,
		Position: chapter.Position, Title: chapter.Title, RangeLabel: chapter.RangeLabel,
		ContentHash: chapter.ContentHash, CharacterCount: chapter.CharacterCount, CreatedAt: chapter.CreatedAt,
	}
}

func newSourceDocumentRecords(projectID, actorID string, input SourceDocumentInput, now time.Time) (*model.ManjuSourceDocument, []model.ManjuSourceChapter, []parsedNovelChapter) {
	parsed := splitNovelChapters(input.Content)
	documentID := uuid.NewString()
	document := &model.ManjuSourceDocument{
		ID: documentID, ProjectID: projectID, Title: input.Title,
		AuthorizationScope: input.AuthorizationScope, FileName: input.FileName,
		ContentHash: contentHash(input.Content), CharacterCount: int64(len([]rune(input.Content))),
		ChapterCount: len(parsed), CreatedBy: actorID, CreatedAt: now, UpdatedAt: now,
	}
	chapters := make([]model.ManjuSourceChapter, 0, len(parsed))
	for index, chapter := range parsed {
		chapters = append(chapters, model.ManjuSourceChapter{
			ID: uuid.NewString(), ProjectID: projectID, DocumentID: documentID,
			Position: index + 1, Title: chapter.Title, RangeLabel: chapter.Title,
			ContentText: chapter.Content, ContentHash: contentHash(chapter.Content),
			CharacterCount: int64(len([]rune(chapter.Content))), CreatedAt: now,
		})
	}
	return document, chapters, parsed
}
