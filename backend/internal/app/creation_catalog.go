package app

import (
	"encoding/json"
	"errors"
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	CreationCatalogKindWorkflow = "workflow"
	CreationCatalogKindPrompt   = "prompt"
	CreationCatalogKindStyle    = "style"
)

type CreationCatalogRequest struct {
	Kind         string         `json:"kind"`
	ItemKey      string         `json:"itemKey"`
	Name         string         `json:"name"`
	Scene        string         `json:"scene"`
	Group        string         `json:"group"`
	Description  string         `json:"description"`
	Content      string         `json:"content"`
	Variables    string         `json:"variables"`
	CanvasType   string         `json:"canvasType"`
	DefaultModel string         `json:"defaultModel"`
	CoverURL     string         `json:"coverUrl"`
	Payload      map[string]any `json:"payload"`
	SortOrder    int            `json:"sortOrder"`
	Enabled      *bool          `json:"enabled"`
}

type CreationCatalogPage struct {
	Items []model.CreationCatalogItem `json:"items"`
	Total int64                       `json:"total"`
	Page  int                         `json:"page"`
	Limit int                         `json:"pageSize"`
}

func (s *Service) AdminCreationCatalogPage(actor *model.User, kind string, query AdminListQuery) (*CreationCatalogPage, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	kind, err := normalizeCreationCatalogKind(kind)
	if err != nil {
		return nil, err
	}
	page, limit := normalizeAdminPage(query.Page, query.Limit)
	items, total, err := s.repo.AdminCreationCatalogItems(kind, query.Keyword, query.Status, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.CreationCatalogItem{}
	}
	return &CreationCatalogPage{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func (s *Service) CreateCreationCatalogItem(actor *model.User, req CreationCatalogRequest) (*model.CreationCatalogItem, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	item, err := creationCatalogItemFromRequest(nil, req)
	if err != nil {
		return nil, err
	}
	item.ID = newID()
	item.CreatedBy = actor.ID
	if err := s.repo.CreateCreationCatalogItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateCreationCatalogItem(actor *model.User, id string, req CreationCatalogRequest) (*model.CreationCatalogItem, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	existing, err := s.repo.CreationCatalogItem(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, BadAuthRequest("创作模板不存在或已删除")
		}
		return nil, err
	}
	if strings.TrimSpace(req.Kind) != "" && strings.TrimSpace(req.Kind) != existing.Kind {
		return nil, BadAuthRequest("不能修改模板类型")
	}
	next, err := creationCatalogItemFromRequest(existing, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveCreationCatalogItem(next); err != nil {
		return nil, err
	}
	return next, nil
}

func (s *Service) DeleteCreationCatalogItem(actor *model.User, id string) error {
	if err := s.RequireAdmin(actor); err != nil {
		return err
	}
	if err := s.repo.DeleteCreationCatalogItem(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BadAuthRequest("创作模板不存在或已删除")
		}
		return err
	}
	return nil
}

func (s *Service) PublicCreationCatalog(kind string, scene string) ([]model.CreationCatalogItem, error) {
	kind, err := normalizeCreationCatalogKind(kind)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.PublicCreationCatalogItems(kind, scene)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.CreationCatalogItem{}
	}
	return items, nil
}

func creationCatalogItemFromRequest(existing *model.CreationCatalogItem, req CreationCatalogRequest) (*model.CreationCatalogItem, error) {
	item := &model.CreationCatalogItem{Enabled: true}
	if existing != nil {
		*item = *existing
	}
	kindSource := req.Kind
	if existing != nil {
		kindSource = existing.Kind
	}
	kind, err := normalizeCreationCatalogKind(kindSource)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, BadAuthRequest("请填写模板名称")
	}
	scene := strings.TrimSpace(req.Scene)
	if scene == "" {
		scene = "general"
	}
	switch scene {
	case "short_drama", "ecommerce", "general":
	default:
		return nil, BadAuthRequest("适用场景不合法")
	}
	item.Kind = kind
	item.ItemKey = strings.TrimSpace(req.ItemKey)
	if item.ItemKey == "" {
		item.ItemKey = newID()
	}
	item.Name = name
	item.Scene = scene
	item.Group = strings.TrimSpace(req.Group)
	item.Description = strings.TrimSpace(req.Description)
	item.Content = strings.TrimSpace(req.Content)
	item.Variables = strings.TrimSpace(req.Variables)
	item.CanvasType = strings.TrimSpace(req.CanvasType)
	item.DefaultModel = strings.TrimSpace(req.DefaultModel)
	item.CoverURL = strings.TrimSpace(req.CoverURL)
	item.SortOrder = req.SortOrder
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	switch kind {
	case CreationCatalogKindWorkflow:
		if item.CanvasType == "" {
			item.CanvasType = "node"
		}
		if item.CanvasType != "node" && item.CanvasType != "batch" {
			return nil, BadAuthRequest("画布类型不合法")
		}
	case CreationCatalogKindPrompt:
		if item.Content == "" {
			return nil, BadAuthRequest("请填写提示词正文")
		}
	case CreationCatalogKindStyle:
		if item.Content == "" {
			return nil, BadAuthRequest("请填写画风提示词前缀")
		}
	}
	payload := req.Payload
	if existing != nil && payload == nil {
		payload = map[string]any{}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, BadAuthRequest("模板定义 JSON 格式错误")
	}
	item.PayloadJSON = string(encoded)
	return item, nil
}

func normalizeCreationCatalogKind(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case CreationCatalogKindWorkflow:
		return CreationCatalogKindWorkflow, nil
	case CreationCatalogKindPrompt:
		return CreationCatalogKindPrompt, nil
	case CreationCatalogKindStyle:
		return CreationCatalogKindStyle, nil
	default:
		return "", BadAuthRequest("创作模板类型不合法")
	}
}
