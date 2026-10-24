package hostedapi

import (
	"strings"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type CreationCatalogService interface {
	AdminCreationCatalogPage(actor *model.User, kind string, query app.AdminListQuery) (*app.CreationCatalogPage, error)
	CreateCreationCatalogItem(actor *model.User, req app.CreationCatalogRequest) (*model.CreationCatalogItem, error)
	UpdateCreationCatalogItem(actor *model.User, id string, req app.CreationCatalogRequest) (*model.CreationCatalogItem, error)
	DeleteCreationCatalogItem(actor *model.User, id string) error
	PublicCreationCatalog(kind string, scene string) ([]model.CreationCatalogItem, error)
	PreviewCreationPrompt(templateID string, nodeRef string, dynastyID string, variables map[string]any) (*app.CreationPromptPreview, error)
}

type publicCreationTemplate struct {
	ID           string         `json:"id"`
	ItemKey      string         `json:"itemKey"`
	Name         string         `json:"name"`
	Scene        string         `json:"scene"`
	Description  string         `json:"description"`
	CanvasType   string         `json:"canvasType"`
	DefaultModel string         `json:"defaultModel"`
	CoverURL     string         `json:"coverUrl"`
	Payload      map[string]any `json:"payload"`
	SortOrder    int            `json:"sortOrder"`
	UpdatedAt    string         `json:"updatedAt"`
}

// RegisterCreationCatalogPublicRoutes exposes enabled templates without
// leaking creator identity, timestamps unrelated to cache invalidation, or the
// raw JSON string used by the admin form.
func RegisterCreationCatalogPublicRoutes(r *gin.RouterGroup, svc CreationCatalogService) {
	r.GET("/creation-templates", func(c *gin.Context) {
		kind := strings.TrimSpace(c.Query("kind"))
		if kind == "" {
			kind = app.CreationCatalogKindWorkflow
		}
		items, err := svc.PublicCreationCatalog(kind, strings.TrimSpace(c.Query("scene")))
		if err != nil {
			writeError(c, err)
			return
		}
		result := make([]publicCreationTemplate, 0, len(items))
		for _, item := range items {
			payload, ok := publicCreationTemplatePayload(item.PayloadJSON)
			if !ok {
				continue
			}
			result = append(result, publicCreationTemplate{
				ID:           item.ID,
				ItemKey:      item.ItemKey,
				Name:         item.Name,
				Scene:        item.Scene,
				Description:  item.Description,
				CanvasType:   item.CanvasType,
				DefaultModel: item.DefaultModel,
				CoverURL:     item.CoverURL,
				Payload:      payload,
				SortOrder:    item.SortOrder,
				UpdatedAt:    item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			})
		}
		writeOK(c, gin.H{"items": result})
	})

	r.POST("/creation-templates/:id/prompt-preview", func(c *gin.Context) {
		var input struct {
			NodeRef   string         `json:"nodeRef"`
			DynastyID string         `json:"dynastyId"`
			Variables map[string]any `json:"variables"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("提示词预览参数格式错误"))
			return
		}
		result, err := svc.PreviewCreationPrompt(
			strings.TrimSpace(c.Param("id")),
			strings.TrimSpace(input.NodeRef),
			strings.TrimSpace(input.DynastyID),
			input.Variables,
		)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, result)
	})
}

func RegisterCreationCatalogAdminRoutes(r *gin.RouterGroup, svc CreationCatalogService, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin/creation-catalog")
	actor := func(c *gin.Context) (*model.User, bool) {
		user, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		if err := requireAdmin(user); err != nil {
			writeError(c, err)
			return nil, false
		}
		return user, true
	}

	group.GET("", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		page, err := queryInt(c, "page", 1)
		if err != nil {
			writeError(c, err)
			return
		}
		limit, err := queryInt(c, "pageSize", 50)
		if err != nil {
			writeError(c, err)
			return
		}
		result, err := svc.AdminCreationCatalogPage(user, strings.TrimSpace(c.Query("kind")), app.AdminListQuery{
			Keyword: strings.TrimSpace(c.Query("keyword")),
			Status:  strings.TrimSpace(c.Query("status")),
			Page:    page,
			Limit:   limit,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, result)
	})

	group.POST("", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input app.CreationCatalogRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("创作模板参数格式错误"))
			return
		}
		item, err := svc.CreateCreationCatalogItem(user, input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.PUT("/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input app.CreationCatalogRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("创作模板参数格式错误"))
			return
		}
		item, err := svc.UpdateCreationCatalogItem(user, strings.TrimSpace(c.Param("id")), input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.DELETE("/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		if err := svc.DeleteCreationCatalogItem(user, strings.TrimSpace(c.Param("id"))); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true})
	})
}
