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

const (
	productionDeliveryContractV1             = "V7.4.26-DELIVERY-V1"
	productionDeliveryContractV2             = "V7.4.26-DELIVERY-V2"
	productionDeliveryComplianceCurrent      = "COMPLIANT"
	productionDeliveryComplianceLegacy       = "LEGACY_COMPLIANT"
	productionDeliveryComplianceUnrecognized = "UNRECOGNIZED"
)

type UserDeliverableView struct {
	ID                     string    `json:"id"`
	ProjectID              string    `json:"projectId"`
	LockRevision           int64     `json:"lockRevision"`
	ProductionRunID        string    `json:"productionRunId"`
	Kind                   string    `json:"kind"`
	FileName               string    `json:"fileName"`
	MimeType               string    `json:"mimeType"`
	Content                string    `json:"content"`
	ContentHash            string    `json:"contentHash"`
	ContentContractVersion string    `json:"contentContractVersion"`
	ComplianceStatus       string    `json:"complianceStatus"`
	CreatedAt              time.Time `json:"createdAt"`
}

type productionDeliveryPlan struct {
	AlreadyReady bool
	ArtifactID   string
	Mode         string
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
	items, err := s.repo.ManjuDeliverablesForProject(projectID)
	if err != nil {
		return nil, err
	}
	return userDeliverableViews(items), nil
}

func (s *Service) ProductionRunDeliverables(actorID, projectID, runID string) ([]UserDeliverableView, error) {
	actorID = strings.TrimSpace(actorID)
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	if _, err := s.repo.ManjuProjectForUser(actorID, projectID); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	run, err := s.repo.ManjuProductionRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrProductionRunNotFound
	}
	items, err := s.repo.ManjuDeliverablesForRun(projectID, runID)
	if err != nil {
		return nil, err
	}
	return userDeliverableViews(items), nil
}

func userDeliverableViews(items []model.ManjuDeliverable) []UserDeliverableView {
	sort.SliceStable(items, func(i, j int) bool {
		return productionDeliverableKindOrder(items[i].Kind) < productionDeliverableKindOrder(items[j].Kind)
	})
	grouped := make(map[string][]model.ManjuDeliverable)
	for _, item := range items {
		grouped[item.ProductionRunID] = append(grouped[item.ProductionRunID], item)
	}
	type contractState struct {
		Version string
		Status  string
	}
	contracts := make(map[string]contractState, len(grouped))
	for runID, rows := range grouped {
		version, status, ok := classifyProductionDeliveryRows(rows)
		if !ok {
			version = ""
			status = productionDeliveryComplianceUnrecognized
		}
		contracts[runID] = contractState{Version: version, Status: status}
	}
	views := make([]UserDeliverableView, 0, len(items))
	for _, item := range items {
		contract := contracts[item.ProductionRunID]
		views = append(views, UserDeliverableView{
			ID:                     item.ID,
			ProjectID:              item.ProjectID,
			LockRevision:           item.LockRevision,
			ProductionRunID:        item.ProductionRunID,
			Kind:                   item.Kind,
			FileName:               item.FileName,
			MimeType:               item.MimeType,
			Content:                item.Content,
			ContentHash:            item.ContentHash,
			ContentContractVersion: contract.Version,
			ComplianceStatus:       contract.Status,
			CreatedAt:              item.CreatedAt,
		})
	}
	return views
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
		_, status, valid := productionDeliveryContractForMode(mode, existing)
		if valid && status == productionDeliveryComplianceCurrent {
			return &productionDeliveryPlan{
				AlreadyReady: true,
				ArtifactID:   stage.ArtifactID,
				Mode:         mode,
			}, nil
		}
		if len(existing) > 0 && !valid {
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
	sources, err := buildProductionDeliverySources(
		s.repo,
		access.Project.Name,
		projectID,
		*run,
		*assetRun,
		stage.ArtifactID,
		currentContent,
	)
	if err != nil {
		return nil, err
	}
	rows, err := buildProductionDeliveryRows(
		access.Project.Name,
		*run,
		mode,
		sources.Markdown,
		sources.BoardSource,
		sources.PrevisMarkdown,
		func(source string) (string, error) {
			return defaultManjuSkillRuntime.BuildBoard(source)
		},
	)
	if err != nil {
		return nil, err
	}
	return &productionDeliveryPlan{
		ArtifactID: stage.ArtifactID,
		Mode:       mode,
		Rows:       rows,
	}, nil
}

type productionDeliverySources struct {
	Markdown       string
	BoardSource    string
	PrevisMarkdown string
}

func buildProductionDeliverySources(
	tx *repository.Repository,
	projectName string,
	projectID string,
	run model.ManjuProductionRun,
	assetRun model.ManjuAssetRun,
	currentArtifactID string,
	currentContent string,
) (productionDeliverySources, error) {
	scriptRun, err := tx.ManjuScriptRun(projectID, assetRun.ScriptRunID)
	if err != nil || scriptRun == nil {
		return productionDeliverySources{}, ErrProductionAssetNotReady
	}
	scriptArtifacts, err := lockedScriptArtifacts(tx, *scriptRun)
	if err != nil {
		return productionDeliverySources{}, err
	}
	assetArtifacts, err := lockedAssetArtifacts(tx, assetRun)
	if err != nil {
		return productionDeliverySources{}, err
	}
	productionArtifacts, err := lockedProductionArtifacts(tx, run.ID)
	if err != nil {
		return productionDeliverySources{}, err
	}
	previs, err := tx.ManjuProductionPrevis(projectID, run.ID)
	if err != nil {
		return productionDeliverySources{}, err
	}
	var previsArtifact *model.ManjuProductionArtifact
	if previs != nil && strings.TrimSpace(previs.ArtifactID) != "" {
		artifacts, listErr := tx.ManjuProductionArtifacts(run.ID)
		if listErr != nil {
			return productionDeliverySources{}, listErr
		}
		previsArtifact = productionArtifactByID(artifacts, previs.ArtifactID)
		if previsArtifact == nil {
			return productionDeliverySources{}, ErrProductionPrevisRequired
		}
	}
	if previsArtifact == nil || strings.TrimSpace(previsArtifact.Content) == "" {
		return productionDeliverySources{}, ErrProductionPrevisRequired
	}
	assetPrompt := ""
	expectedAssetKind := assetArtifactKind(assetRun.AssetMode, AssetStageCompile)
	for _, artifact := range assetArtifacts {
		if artifact.Kind == expectedAssetKind {
			assetPrompt = artifact.Content
			break
		}
	}
	if strings.TrimSpace(assetPrompt) == "" {
		return productionDeliverySources{}, ErrProductionDeliverablesNotReady
	}
	storyboardPrompt := ""
	for _, artifact := range productionArtifacts {
		if artifact.Kind == model.ManjuProductionArtifactFeed {
			storyboardPrompt = artifact.Content
			break
		}
	}
	if strings.TrimSpace(storyboardPrompt) == "" {
		return productionDeliverySources{}, ErrProductionDeliverablesNotReady
	}
	return productionDeliverySources{
		Markdown:       buildUserFacingDeliveryMarkdown(projectName, assetPrompt, storyboardPrompt),
		BoardSource:    buildProductionBoardArchive(scriptArtifacts, assetArtifacts, productionArtifacts, currentArtifactID, currentContent),
		PrevisMarkdown: previsArtifact.Content,
	}, nil
}

func buildUserFacingDeliveryMarkdown(projectName, assetPrompts, storyboardPrompts string) string {
	var document strings.Builder
	fmt.Fprintf(&document, "# %s · 漫剧交付\n\n", strings.TrimSpace(projectName))
	document.WriteString("## 资产提示词\n\n")
	document.WriteString(strings.TrimSpace(assetPrompts))
	document.WriteString("\n\n## 分镜提示词\n\n")
	document.WriteString(strings.TrimSpace(storyboardPrompts))
	document.WriteString("\n")
	return document.String()
}

func buildProductionBoardArchive(
	scriptArtifacts []model.ManjuScriptArtifact,
	assetArtifacts []model.ManjuAssetArtifact,
	productionArtifacts []model.ManjuProductionArtifact,
	currentArtifactID string,
	currentContent string,
) string {
	var document strings.Builder
	document.WriteString("# VisiDeep AI 漫剧全流程交付归档\n")
	for _, artifact := range scriptArtifacts {
		fmt.Fprintf(&document, "\n# P1 · %s v%d\n\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
	}
	for _, artifact := range assetArtifacts {
		fmt.Fprintf(&document, "\n# P2 · %s v%d\n\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
	}
	currentIncluded := false
	for _, artifact := range productionArtifacts {
		fmt.Fprintf(&document, "\n# 生产阶段 · %s v%d\n\n%s\n", artifact.Kind, artifact.Version, artifact.Content)
		if artifact.ID == currentArtifactID {
			currentIncluded = true
		}
	}
	if !currentIncluded && strings.TrimSpace(currentContent) != "" {
		document.WriteString("\n# P5 当前质检交付\n\n")
		document.WriteString(currentContent)
		document.WriteString("\n")
	}
	return document.String()
}

func buildProductionDeliveryRows(
	projectName string,
	run model.ManjuProductionRun,
	mode, markdown, boardSource, previsMarkdown string,
	buildBoard func(string) (string, error),
) ([]model.ManjuDeliverable, error) {
	if strings.TrimSpace(markdown) == "" {
		return nil, ErrProductionDeliverablesNotReady
	}
	if (mode == DeliveryBoard || mode == DeliveryBoth) && strings.TrimSpace(boardSource) == "" {
		return nil, ErrProductionDeliverablesNotReady
	}
	if strings.TrimSpace(previsMarkdown) == "" {
		return nil, ErrProductionPrevisRequired
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
		board, err := buildBoard(boardSource)
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
	rows = append(rows, newProductionDeliverable(
		run,
		model.ManjuDeliverableKindPrevis,
		baseName+"-预演故事板.md",
		"text/markdown; charset=utf-8",
		previsMarkdown,
		now,
	))
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
		ContractVersion: productionDeliveryContractV2,
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
	case model.ManjuDeliverableKindPrevis:
		return 3
	case model.ManjuDeliverableKindManifest:
		return 4
	default:
		return 99
	}
}

func productionDeliveryCurrentKinds(mode string) []string {
	if mode == DeliveryBoard || mode == DeliveryBoth {
		return []string{
			model.ManjuDeliverableKindMarkdown,
			model.ManjuDeliverableKindBoard,
			model.ManjuDeliverableKindPrevis,
			model.ManjuDeliverableKindManifest,
		}
	}
	return []string{
		model.ManjuDeliverableKindMarkdown,
		model.ManjuDeliverableKindPrevis,
		model.ManjuDeliverableKindManifest,
	}
}

func productionDeliveryLegacyKinds(mode string) []string {
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

func classifyProductionDeliveryRows(items []model.ManjuDeliverable) (string, string, bool) {
	currentSets := [][]string{
		{
			model.ManjuDeliverableKindMarkdown,
			model.ManjuDeliverableKindBoard,
			model.ManjuDeliverableKindPrevis,
			model.ManjuDeliverableKindManifest,
		},
		{
			model.ManjuDeliverableKindMarkdown,
			model.ManjuDeliverableKindPrevis,
			model.ManjuDeliverableKindManifest,
		},
	}
	legacySets := [][]string{
		{
			model.ManjuDeliverableKindMarkdown,
			model.ManjuDeliverableKindBoard,
			model.ManjuDeliverableKindManifest,
		},
		{
			model.ManjuDeliverableKindMarkdown,
			model.ManjuDeliverableKindManifest,
		},
	}
	currentSet := false
	for _, expected := range currentSets {
		if productionDeliveryKindsMatch(items, expected) {
			currentSet = true
			break
		}
	}
	if currentSet {
		for _, item := range items {
			version := strings.TrimSpace(item.ContractVersion)
			if version == "" || version == productionDeliveryContractV1 {
				return productionDeliveryContractV1, productionDeliveryComplianceLegacy, true
			}
			if version != productionDeliveryContractV2 {
				return "", "", false
			}
		}
		return productionDeliveryContractV2, productionDeliveryComplianceCurrent, true
	}
	for _, expected := range legacySets {
		if productionDeliveryKindsMatch(items, expected) {
			return productionDeliveryContractV1, productionDeliveryComplianceLegacy, true
		}
	}
	return "", "", false
}

func productionDeliveryContractForMode(mode string, items []model.ManjuDeliverable) (string, string, bool) {
	currentKinds := productionDeliveryCurrentKinds(mode)
	legacyKinds := productionDeliveryLegacyKinds(mode)
	if !productionDeliveryKindsMatch(items, currentKinds) &&
		!productionDeliveryKindsMatch(items, legacyKinds) {
		return "", "", false
	}
	return classifyProductionDeliveryRows(items)
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
	projectID, runID, mode string,
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
	if len(existing) == 0 {
		return ErrProductionDeliverablesNotReady
	}
	_, status, valid := productionDeliveryContractForMode(mode, existing)
	if !valid {
		return ErrProductionDeliverablesNotReady
	}
	if status == productionDeliveryComplianceCurrent {
		return verifyProductionDeliveryRows(existing, rows)
	}

	currentKinds := productionDeliveryCurrentKinds(mode)
	if !productionDeliveryKindsMatch(rows, currentKinds) {
		return ErrProductionDeliverablesNotReady
	}
	existingKinds := make(map[string]bool, len(existing))
	for _, row := range existing {
		existingKinds[row.Kind] = true
	}
	missing := make([]model.ManjuDeliverable, 0, len(currentKinds))
	for _, row := range rows {
		if !existingKinds[row.Kind] {
			missing = append(missing, row)
		}
	}
	if len(missing) > 0 {
		if err := tx.CreateManjuDeliverables(missing); err != nil {
			return err
		}
		existing, err = tx.ManjuDeliverablesForRun(projectID, runID)
		if err != nil {
			return err
		}
	}
	if !productionDeliveryKindsMatch(existing, currentKinds) {
		return ErrProductionDeliverablesNotReady
	}
	return nil
}

func verifyProductionDeliveryRows(existing, expectedRows []model.ManjuDeliverable) error {
	if len(existing) != len(expectedRows) {
		return ErrProductionDeliverablesNotReady
	}
	expected := make(map[string]model.ManjuDeliverable, len(expectedRows))
	for _, row := range expectedRows {
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
