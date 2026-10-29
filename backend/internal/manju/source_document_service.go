package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

type SourceLibraryState struct {
	ContentHash   string `json:"contentHash"`
	DocumentCount int    `json:"documentCount"`
	ChapterCount  int    `json:"chapterCount"`
}

func (s *Service) CreateSourceDocument(actorID, projectID string, input SourceDocumentInput) (*SourceDocumentDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	input.Title = strings.TrimSpace(input.Title)
	input.AuthorizationScope = strings.TrimSpace(input.AuthorizationScope)
	input.FileName = strings.TrimSpace(input.FileName)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.AuthorizationScope == "" || input.Content == "" {
		return nil, ErrInvalidScriptSource
	}
	var createdID string
	err := s.repo.Transaction(func(tx *repository.Repository) error {
		access, err := tx.ManjuProjectForUser(actorID, projectID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !canEditScript(access.Role) {
			return ErrForbidden
		}
		now := time.Now()
		document, chapters, _ := newSourceDocumentRecords(projectID, actorID, input, now)
		if err := tx.CreateManjuSourceDocument(document, chapters); err != nil {
			return err
		}
		createdID = document.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.SourceDocumentDetail(actorID, projectID, createdID)
}

func (s *Service) SourceDocuments(actorID, projectID string) ([]SourceDocumentView, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(access.Role) == "" {
		return nil, ErrForbidden
	}
	documents, err := s.repo.ManjuSourceDocuments(projectID)
	if err != nil {
		return nil, err
	}
	views := make([]SourceDocumentView, 0, len(documents))
	for _, document := range documents {
		views = append(views, sourceDocumentView(document))
	}
	return views, nil
}

func (s *Service) SourceDocumentDetail(actorID, projectID, documentID string) (*SourceDocumentDetail, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	documentID = strings.TrimSpace(documentID)
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(access.Role) == "" {
		return nil, ErrForbidden
	}
	document, err := s.repo.ManjuSourceDocument(projectID, documentID)
	if err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ErrScriptSourceNotFound
	}
	chapters, err := s.repo.ManjuSourceChapters(documentID)
	if err != nil {
		return nil, err
	}
	chapterViews := make([]SourceChapterView, 0, len(chapters))
	for _, chapter := range chapters {
		chapterViews = append(chapterViews, sourceChapterView(chapter))
	}
	return &SourceDocumentDetail{Document: sourceDocumentView(*document), Chapters: chapterViews}, nil
}

func (s *Service) SourceLibraryState(actorID, projectID string) (*SourceLibraryState, error) {
	documents, err := s.SourceDocuments(actorID, projectID)
	if err != nil {
		return nil, err
	}
	chapters := make([]SourceChapterView, 0)
	for _, document := range documents {
		detail, err := s.SourceDocumentDetail(actorID, projectID, document.ID)
		if err != nil {
			return nil, err
		}
		chapters = append(chapters, detail.Chapters...)
	}
	return sourceLibraryState(documents, chapters), nil
}

func sourceLibraryState(documents []SourceDocumentView, chapters []SourceChapterView) *SourceLibraryState {
	lines := make([]string, 0, len(chapters))
	for _, chapter := range chapters {
		lines = append(lines, chapter.DocumentID+"|"+chapter.ID+"|"+chapter.ContentHash)
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return &SourceLibraryState{
		ContentHash: hex.EncodeToString(sum[:]), DocumentCount: len(documents), ChapterCount: len(chapters),
	}
}
