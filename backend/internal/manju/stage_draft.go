package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/google/uuid"
)

type ManjuStageDraftView struct {
	ID             string    `json:"id"`
	Version        int       `json:"version"`
	Content        string    `json:"content"`
	ContentHash    string    `json:"contentHash"`
	ValidationJSON string    `json:"validationJson"`
	Source         string    `json:"source"`
	CreatedBy      string    `json:"createdBy"`
	CreatedAt      time.Time `json:"createdAt"`
}

func stageDraftView(item *model.ManjuStageDraft) *ManjuStageDraftView {
	if item == nil {
		return nil
	}
	return &ManjuStageDraftView{
		ID: item.ID, Version: item.Version, Content: item.Content,
		ContentHash: item.ContentHash, ValidationJSON: item.ValidationJSON,
		Source: item.Source, CreatedBy: item.CreatedBy, CreatedAt: item.CreatedAt,
	}
}

func saveFailedManjuStageDraft(
	tx *repository.Repository,
	projectID, phase, runID, stageID, stageKey, content, source, validationJSON, actorID string,
) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	version, err := tx.NextManjuStageDraftVersion(runID, stageID)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(content))
	if strings.TrimSpace(validationJSON) == "" {
		validationJSON = "{}"
	}
	if !json.Valid([]byte(validationJSON)) {
		encoded, _ := json.Marshal(map[string]string{"error": validationJSON})
		validationJSON = string(encoded)
	}
	return tx.CreateManjuStageDraft(&model.ManjuStageDraft{
		ID:             uuid.NewString(),
		ProjectID:      projectID,
		Phase:          phase,
		RunID:          runID,
		StageID:        stageID,
		Version:        version,
		Content:        content,
		ContentHash:    hex.EncodeToString(sum[:]),
		ValidationJSON: validationJSON,
		Source:         source,
		CreatedBy:      actorID,
		CreatedAt:      time.Now(),
	})
}

func validationJSONForError(message string) string {
	encoded, _ := json.Marshal(map[string]any{
		"issues": []string{strings.TrimSpace(message)},
	})
	return string(encoded)
}

func validationJSONForResult(result ManjuSkillValidationResult, fallback string) string {
	payload := map[string]any{
		"passed":    result.Passed,
		"exit_code": result.ExitCode,
		"issues":    result.Issues,
		"warnings":  result.Warnings,
		"output":    result.Output,
	}
	if len(result.Issues) == 0 && strings.TrimSpace(fallback) != "" {
		payload["issues"] = []string{strings.TrimSpace(fallback)}
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}
