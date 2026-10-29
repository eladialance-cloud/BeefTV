package hostedapi

import (
	"context"
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// ChannelAdminService is the slice of internal/app the admin console needs to
// manage system model channels. The service layer already owned every one of
// these operations for the desktop shell; this interface is only the HTTP seam,
// so the console stops being a page with nothing behind it.
type ChannelAdminService interface {
	AdminSystemChannelPage(actor *model.User, query app.AdminListQuery) (*app.AdminChannelPage, error)
	CreateSystemChannel(actor *model.User, req app.ChannelRequest) (*app.PublicModelChannel, error)
	UpdateSystemChannel(actor *model.User, id string, req app.ChannelRequest) (*app.PublicModelChannel, error)
	DeleteSystemChannel(actor *model.User, id string) error
	DuplicateSystemChannel(actor *model.User, id string) (*app.PublicModelChannel, error)

	AdminChannelModels(actor *model.User, channelID string) ([]model.ChannelModel, error)
	SaveAdminChannelModel(actor *model.User, channelID string, id string, req app.ChannelModelRequest) (*model.ChannelModel, error)
	TestAdminChannelModel(ctx context.Context, actor *model.User, channelID string, req app.ChannelModelRequest) (*app.AdminChannelModelTestResult, error)
	DeleteAdminChannelModels(actor *model.User, channelID string, ids []string) (int64, error)
	PreviewAdminChannelModels(ctx context.Context, actor *model.User, channelID string) ([]string, error)
	PreviewAdminChannelModelCatalog(ctx context.Context, actor *model.User, channelID string) ([]app.ChannelModelCatalogItem, error)
	ImportAdminChannelModels(ctx context.Context, actor *model.User, channelID string, selected []string) (*app.AdminChannelModelFetchResult, error)
	FetchAdminChannelModels(ctx context.Context, actor *model.User, channelID string) (*app.AdminChannelModelFetchResult, error)
}

// RegisterChannelAdminRoutes mounts the admin console's model-channel surface.
// Secrets stay server-side: the service returns masked public views, so the
// browser never receives a decryptable credential.
func RegisterChannelAdminRoutes(r *gin.RouterGroup, svc ChannelAdminService, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin/channels")

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
		limit, err := queryInt(c, "pageSize", 20)
		if err != nil {
			writeError(c, err)
			return
		}
		result, err := svc.AdminSystemChannelPage(user, app.AdminListQuery{
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
		var input app.ChannelRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("模型渠道参数格式错误"))
			return
		}
		item, err := svc.CreateSystemChannel(user, input)
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
		var input app.ChannelRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("模型渠道参数格式错误"))
			return
		}
		item, err := svc.UpdateSystemChannel(user, strings.TrimSpace(c.Param("id")), input)
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
		if err := svc.DeleteSystemChannel(user, strings.TrimSpace(c.Param("id"))); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true})
	})

	group.POST("/:id/duplicate", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.DuplicateSystemChannel(user, strings.TrimSpace(c.Param("id")))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/:id/models", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.AdminChannelModels(user, strings.TrimSpace(c.Param("id")))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.POST("/:id/models/test", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input app.ChannelModelRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("渠道模型测试参数格式错误"))
			return
		}
		item, err := svc.TestAdminChannelModel(c.Request.Context(), user, strings.TrimSpace(c.Param("id")), input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/:id/models", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var body struct {
			ID string `json:"id"`
			app.ChannelModelRequest
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, kernel.BadAuthRequest("渠道模型参数格式错误"))
			return
		}
		item, err := svc.SaveAdminChannelModel(user, strings.TrimSpace(c.Param("id")), strings.TrimSpace(body.ID), body.ChannelModelRequest)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.DELETE("/:id/models", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var body struct {
			IDs []string `json:"ids"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, kernel.BadAuthRequest("渠道模型参数格式错误"))
			return
		}
		deleted, err := svc.DeleteAdminChannelModels(user, strings.TrimSpace(c.Param("id")), body.IDs)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": deleted})
	})

	// preview 只回目录、不落库；fetch 把上游目录里缺失的模型登记为停用状态，
	// import 只导入管理员勾选的部分。三者分开是上游接口语义，不是冗余。
	group.POST("/:id/models/preview", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		catalog, err := svc.PreviewAdminChannelModelCatalog(c.Request.Context(), user, strings.TrimSpace(c.Param("id")))
		if err != nil {
			writeError(c, err)
			return
		}
		models := make([]string, 0, len(catalog))
		for _, item := range catalog {
			models = append(models, item.ID)
		}
		writeOK(c, gin.H{"models": models, "catalog": catalog})
	})

	group.POST("/:id/models/import", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var body struct {
			Models []string `json:"models"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, kernel.BadAuthRequest("渠道模型参数格式错误"))
			return
		}
		result, err := svc.ImportAdminChannelModels(c.Request.Context(), user, strings.TrimSpace(c.Param("id")), body.Models)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, result)
	})

	group.POST("/:id/models/fetch", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		result, err := svc.FetchAdminChannelModels(c.Request.Context(), user, strings.TrimSpace(c.Param("id")))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, result)
	})
}

func queryInt(c *gin.Context, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, kernel.NewAppError(kernel.CodeInvalidArgument, name+" 必须是正整数")
	}
	return parsed, nil
}
