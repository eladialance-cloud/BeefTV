package manju

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

var ErrProductionDeliverablesNotReady = errors.New("manju production deliverables are not ready")

type UserDeliverableView struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"projectId"`
	LockRevision    int64     `json:"lockRevision"`
	ProductionRunID string    `json:"productionRunId"`
	Kind            string    `json:"kind"`
	FileName        string    `json:"fileName"`
	MimeType        string    `json:"mimeType"`
	Content         string    `json:"content"`
	ContentHash     string    `json:"contentHash"`
	CreatedAt       time.Time `json:"createdAt"`
}

type productionDeliveryPlan struct {
	AlreadyReady bool
	ArtifactID   string
	Rows         []model.ManjuDeliverable
}

type productionDeliveryManifest struct {
	Version         string                           `json:"version"`
	ProjectID       string                           `json:"projectId"`
	ProductionRunID string                           `json:"productionRunId"`
	LockRevision    int64                            `json:"lockRevision"`
	DeliveryMode    string                           `json:"deliveryMode"`
	CanonicalSource string                           `json:"canonicalSource"`
	Files           []productionDeliveryManifestFile `json:"files"`
}

type productionDeliveryManifestFile struct {
	Kind     string `json:"kind"`
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	SHA256   string `json:"sha256"`
}

func (s *Service) UserDeliverables(actorID, projectID string) ([]UserDeliverableView, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	if _, err := s.repo.ManjuProjectForUser(actorID, projectID); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	items, err := s.repo.LatestManjuDeliverables(projectID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		return productionDeliverableKindOrder(items[i].Kind) < productionDeliverableKindOrder(items[j].Kind)
	})
	views := make([]UserDeliverableView, 0, len(items))
	for _, item := range items {
		views = append(views, UserDeliverableView{
			ID:              item.ID,
			ProjectID:       item.ProjectID,
			LockRevision:    item.LockRevision,
			ProductionRunID: item.ProductionRunID,
			Kind:            item.Kind,
			FileName:        item.FileName,
			MimeType:        item.MimeType,
			Content:         item.Content,
			ContentHash:     item.ContentHash,
			CreatedAt:       item.CreatedAt,
		})
	}
	return views, nil
}

func (s *Service) prepareProductionDelivery(actorID, projectID, runID string) (*productionDeliveryPlan, error) {
	access, err := s.repo.ManjuProjectForUser(actorID, projectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !canConfirmProduction(access.Role) {
		return nil, ErrForbidden
	}
	run, err := s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrProductionRunNotFound
	}
	stage, err := s.repo.ManjuProductionStage(runID, ProductionStageQC)
	if err != nil {
		return nil, err
	}
	if stage == nil || strings.TrimSpace(stage.ArtifactID) == "" {
		return nil, ErrProductionDeliverablesNotReady
	}
	receipt, err := s.repo.LatestManjuLockReceipt(projectID)
	if err != nil {
		return nil, err
	}
	if receipt == nil || receipt.Status != model.ManjuLockStatusLocked || receipt.Revision != run.LockRevision {
		return nil, ErrProductionDeliverablesNotReady
	}
	view, err := receiptView(*receipt)
	if err != nil {
		return nil, err
	}
	mode := strings.TrimSpace(view.Snapshot.DeliveryMode)
	if mode != DeliveryMarkdown && mode != DeliveryBoard && mode != DeliveryBoth {
		return nil, ErrProductionDeliverablesNotReady
	}
	completedHealing := run.Status == model.ManjuProductionRunStatusCompleted &&
		stage.Status == model.ManjuProductionStageStatusLocked
	if completedHealing {
		existing, err := s.repo.ManjuDeliverablesForRun(projectID, runID)
		if err != nil {
			return nil, err
		}
		expected := productionDeliveryExpectedKinds(mode)
		if productionDeliveryKindsMatch(existing, expected) {
			return &productionDeliveryPlan{
				AlreadyReady: true,
				ArtifactID:   stage.ArtifactID,
			}, nil
		}
		if len(existing) > 0 {
			return nil, ErrProductionDeliverablesNotReady
		}
	}
	if !completedHealing && (run.Status != model.ManjuProductionRunStatusActive ||
		run.CurrentStage != ProductionStageQC ||
		stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation) {
		return nil, ErrProductionDeliverablesNotReady
	}
	assetRun, err := s.repo.ManjuAssetRun(projectID, run.AssetRunID)
	if err != nil {
		return nil, err
	}
	if assetRun == nil {
		return nil, ErrProductionAssetNotReady
	}
	currentContent, err := productionArtifactContent(s.repo, runID, stage.ArtifactID)
	if err != nil || strings.TrimSpace(currentContent) == "" {
		return nil, ErrProductionDeliverablesNotReady
	}
	markdown, err := buildProductionDeliveryMarkdown(s.repo, projectID, *run, *assetRun, currentContent)
	if err != nil {
		return nil, err
	}
	rows, err := buildProductionDeliveryRows(
		access.Project.Name,
		*run,
		mode,
		markdown,
		func(source string) (string, error) {
			return defaultManjuSkillRuntime.BuildBoard(source)
		},
	)
	if err != nil {
		return nil, err
	}
	return &productionDeliveryPlan{
		ArtifactID: stage.ArtifactID,
		Rows:       rows,
	}, nil
}

func buildProductionDeliveryMarkdown(
	tx *repository.Repository,
	projectID string,
	run model.ManjuProductionRun,
	assetRun model.ManjuAssetRun,
	currentContent string,
) (string, error) {
	scriptRun, err := tx.ManjuScriptRun(projectID, assetRun.ScriptRunID)
	if err != nil || scriptRun == nil {
		return "", ErrProductionAssetNotReady
	}
	scriptArtifacts, err := lockedScriptArtifacts(tx, *scriptRun)
	if err != nil {
		return "", err
	}
	assetArtifacts, err := lockedAssetArtifacts(tx, assetRun)
	if err != nil {
		return "", err
	}
	productionArtifacts, err := lockedProductionArtifacts(tx, run.ID)
	if err != nil {
		return "", err
	}
	previs, err := tx.ManjuProductionPrevis(projectID, run.ID)
	if err != nil {
		return "", err
	}
	var previsArtifact *model.ManjuProductionArtifact
	if previs != nil && strings.TrimSpace(previs.ArtifactID) != "" {
		artifacts, listErr := tx.ManjuProductionArtifacts(run.ID)
		if listErr != nil {
			return "", listErr
		}
		previsArtifact = productionArtifactByID(artifacts, previs.ArtifactID)
		if previsArtifact == nil {
			return "", ErrProductionPrevisRequired
		}
	}
	if previsArtifact == nil {
		return "", ErrProductionPrevisRequired
	}
	return buildProductionContractArchive(scriptArtifacts, assetArtifacts, productionArtifacts, previsArtifact, currentContent), nil
}

func buildProductionDeliveryRows(
	projectName string,
	run model.ManjuProductionRun,
	mode, markdown string,
	buildBoard func(string) (string, error),
) ([]model.ManjuDeliverable, error) {
	if strings.TrimSpace(markdown) == "" {
		return nil, ErrProductionDeliverablesNotReady
	}
	baseName := safeProductionDeliveryBaseName(projectName)
	now := time.Now()
	rows := []model.ManjuDeliverable{
		newProductionDeliverable(
			run,
			model.ManjuDeliverableKindMarkdown,
			baseName+"-P5最终交付.md",
			"text/markdown; charset=utf-8",
			markdown,
			now,
		),
	}
	if mode == DeliveryBoard || mode == DeliveryBoth {
		board, err := buildBoard(markdown)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(board) == "" {
			return nil, errors.New("官方 build_board_lite.py 输出为空")
		}
		rows = append(rows, newProductionDeliverable(
			run,
			model.ManjuDeliverableKindBoard,
			baseName+"-P5分镜看板.html",
			"text/html; charset=utf-8",
			board,
			now,
		))
	}
	manifest := productionDeliveryManifest{
		Version:         manjuSkillVersion,
		ProjectID:       run.ProjectID,
		ProductionRunID: run.ID,
		LockRevision:    run.LockRevision,
		DeliveryMode:    mode,
		CanonicalSource: model.ManjuDeliverableKindMarkdown,
		Files:           make([]productionDeliveryManifestFile, 0, len(rows)),
	}
	for _, row := range rows {
		manifest.Files = append(manifest.Files, productionDeliveryManifestFile{
			Kind: row.Kind, FileName: row.FileName, MimeType: row.MimeType, SHA256: row.ContentHash,
		})
	}
	manifestContent, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	rows = append(rows, newProductionDeliverable(
		run,
		model.ManjuDeliverableKindManifest,
		baseName+"-P5交付清单.json",
		"application/json; charset=utf-8",
		string(manifestContent)+"\n",
		now,
	))
	return rows, nil
}

func newProductionDeliverable(
	run model.ManjuProductionRun,
	kind, fileName, mimeType, content string,
	createdAt time.Time,
) model.ManjuDeliverable {
	sum := sha256.Sum256([]byte(content))
	return model.ManjuDeliverable{
		ID:              productionDeliveryID(run.ID, kind),
		ProjectID:       run.ProjectID,
		LockRevision:    run.LockRevision,
		ProductionRunID: run.ID,
		Kind:            kind,
		FileName:        fileName,
		MimeType:        mimeType,
		Content:         content,
		ContentHash:     hex.EncodeToString(sum[:]),
		CreatedAt:       createdAt,
	}
}

func productionDeliveryID(runID, kind string) string {
	sum := sha256.Sum256([]byte(runID + "\x00" + kind))
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func safeProductionDeliveryBaseName(projectName string) string {
	name := strings.TrimSpace(projectName)
	if name == "" {
		name = "漫剧项目"
	}
	replacer := strings.NewReplacer(
		"<", "-", ">", "-", ":", "-", `"`, "-", "/", "-", `\`, "-", "|", "-", "?", "-", "*", "-",
	)
	name = strings.Map(func(char rune) rune {
		if unicode.IsControl(char) {
			return '-'
		}
		return char
	}, replacer.Replace(name))
	name = strings.Trim(name, " .")
	if name == "" {
		name = "漫剧项目"
	}
	runes := []rune(name)
	if len(runes) > 120 {
		name = string(runes[:120])
	}
	return name
}

func productionDeliverableKindOrder(kind string) int {
	switch kind {
	case model.ManjuDeliverableKindMarkdown:
		return 1
	case model.ManjuDeliverableKindBoard:
		return 2
	case model.ManjuDeliverableKindManifest:
		return 3
	default:
		return 99
	}
}

func productionDeliveryExpectedKinds(mode string) []string {
	if mode == DeliveryBoard || mode == DeliveryBoth {
		return []string{
			model.ManjuDeliverableKindMarkdown,
			model.ManjuDeliverableKindBoard,
			model.ManjuDeliverableKindManifest,
		}
	}
	return []string{
		model.ManjuDeliverableKindMarkdown,
		model.ManjuDeliverableKindManifest,
	}
}

func productionDeliveryKindsMatch(items []model.ManjuDeliverable, expected []string) bool {
	if len(items) != len(expected) {
		return false
	}
	found := make(map[string]bool, len(items))
	for _, item := range items {
		if found[item.Kind] {
			return false
		}
		found[item.Kind] = true
	}
	for _, kind := range expected {
		if !found[kind] {
			return false
		}
	}
	return true
}

func persistProductionDeliveryRows(
	tx *repository.Repository,
	projectID, runID string,
	rows []model.ManjuDeliverable,
) error {
	existing, err := tx.ManjuDeliverablesForRun(projectID, runID)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		if err := tx.CreateManjuDeliverables(rows); err != nil {
			return err
		}
		existing, err = tx.ManjuDeliverablesForRun(projectID, runID)
		if err != nil {
			return err
		}
	}
	if len(existing) != len(rows) {
		return ErrProductionDeliverablesNotReady
	}
	expected := make(map[string]model.ManjuDeliverable, len(rows))
	for _, row := range rows {
		expected[row.Kind] = row
	}
	for _, row := range existing {
		want, ok := expected[row.Kind]
		if !ok ||
			row.FileName != want.FileName ||
			row.MimeType != want.MimeType ||
			row.ContentHash != want.ContentHash {
			return ErrProductionDeliverablesNotReady
		}
	}
	return nil
}
