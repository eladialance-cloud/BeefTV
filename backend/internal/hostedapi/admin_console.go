package hostedapi

import (
	"strings"
	"time"

	"infinite-canvas/backend/internal/hostedadmin"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/gin-gonic/gin"
)

// HostedAdminService is the HTTP-facing admin-console boundary. The concrete
// hostedadmin.Service owns authorization, validation, moderation and audit.
type HostedAdminService interface {
	ListUsers(repository.AdminUserFilter) ([]repository.AdminUserRow, int64, error)
	CreateUser(*model.User, hostedadmin.CreateUserInput) (*repository.AdminUserRow, error)
	UpdateUser(*model.User, string, hostedadmin.UpdateUserInput) (*repository.AdminUserRow, error)
	ResetUserPassword(*model.User, string, string) error
	ListProjects(repository.AdminProjectFilter) ([]repository.AdminProjectRow, int64, error)
	SetProjectArchived(*model.User, string, string, bool, string) error
	ListAssets(repository.AdminAssetFilter) ([]repository.AdminAssetRow, int64, error)
	ModerateAsset(*model.User, string, bool, string) error
	ListLedger(repository.AdminLedgerFilter) ([]repository.AdminLedgerRow, int64, error)
	ListAudit(repository.AdminAuditFilter) ([]repository.AdminAuditRow, int64, error)
	Overview(int) (repository.AdminOverviewMetrics, error)
	SiteConfig() (hostedadmin.SiteConfig, error)
	SaveSiteConfig(*model.User, hostedadmin.SiteConfig) (hostedadmin.SiteConfig, error)
}

// RegisterHostedAdminRoutes mounts the hosted admin console's system-wide
// management surface. Every request resolves the current session first and
// then requires an active administrator.
func RegisterHostedAdminRoutes(r *gin.RouterGroup, svc HostedAdminService, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin")

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

	group.GET("/overview", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		days, err := queryInt(c, "days", 7)
		if err != nil {
			writeError(c, err)
			return
		}
		metrics, err := svc.Overview(days)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"days": days, "metrics": metrics})
	})

	group.GET("/users", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		page, pageSize, err := adminPageParams(c)
		if err != nil {
			writeError(c, err)
			return
		}
		items, total, err := svc.ListUsers(repository.AdminUserFilter{
			Keyword: strings.TrimSpace(c.Query("keyword")),
			Role:    strings.TrimSpace(c.Query("role")),
			Status:  strings.TrimSpace(c.Query("status")),
			Page:    page, PageSize: pageSize,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, adminPage(items, total, page, pageSize))
	})

	group.POST("/users", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input hostedadmin.CreateUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("用户参数格式错误"))
			return
		}
		item, err := svc.CreateUser(user, input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.PATCH("/users/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input hostedadmin.UpdateUserInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("用户参数格式错误"))
			return
		}
		item, err := svc.UpdateUser(user, strings.TrimSpace(c.Param("id")), input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/users/:id/password", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("密码参数格式错误"))
			return
		}
		if err := svc.ResetUserPassword(user, strings.TrimSpace(c.Param("id")), input.Password); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"reset": true})
	})

	group.GET("/projects", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		page, pageSize, err := adminPageParams(c)
		if err != nil {
			writeError(c, err)
			return
		}
		items, total, err := svc.ListProjects(repository.AdminProjectFilter{
			Kind:    strings.TrimSpace(c.Query("kind")),
			Keyword: strings.TrimSpace(c.Query("keyword")),
			Status:  strings.TrimSpace(c.Query("status")),
			Page:    page, PageSize: pageSize,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, adminPage(items, total, page, pageSize))
	})

	group.POST("/projects/:id/archive", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input struct {
			Kind   string `json:"kind"`
			Reason string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("项目归档参数格式错误"))
			return
		}
		if err := svc.SetProjectArchived(user, input.Kind, strings.TrimSpace(c.Param("id")), true, input.Reason); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"archived": true})
	})

	group.POST("/projects/:id/restore", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input struct {
			Kind string `json:"kind"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("项目恢复参数格式错误"))
			return
		}
		if err := svc.SetProjectArchived(user, input.Kind, strings.TrimSpace(c.Param("id")), false, ""); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"archived": false})
	})

	group.GET("/assets", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		page, pageSize, err := adminPageParams(c)
		if err != nil {
			writeError(c, err)
			return
		}
		items, total, err := svc.ListAssets(repository.AdminAssetFilter{
			Keyword: strings.TrimSpace(c.Query("keyword")),
			Kind:    strings.TrimSpace(c.Query("kind")),
			Status:  strings.TrimSpace(c.Query("status")),
			Page:    page, PageSize: pageSize,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, adminPage(items, total, page, pageSize))
	})

	group.POST("/assets/:id/block", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input struct {
			Reason string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("素材下架参数格式错误"))
			return
		}
		if err := svc.ModerateAsset(user, strings.TrimSpace(c.Param("id")), true, input.Reason); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"blocked": true})
	})

	group.POST("/assets/:id/unblock", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		if err := svc.ModerateAsset(user, strings.TrimSpace(c.Param("id")), false, ""); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"blocked": false})
	})

	group.GET("/credit-ledger", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		page, pageSize, err := adminPageParams(c)
		if err != nil {
			writeError(c, err)
			return
		}
		start, end, err := adminTimeRange(c)
		if err != nil {
			writeError(c, err)
			return
		}
		items, total, err := svc.ListLedger(repository.AdminLedgerFilter{
			Keyword: strings.TrimSpace(c.Query("keyword")),
			Kind:    strings.TrimSpace(c.Query("kind")),
			Model:   strings.TrimSpace(c.Query("model")),
			Token:   strings.TrimSpace(c.Query("token")),
			Start:   start, End: end, Page: page, PageSize: pageSize,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, adminPage(items, total, page, pageSize))
	})

	group.GET("/site", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		cfg, err := svc.SiteConfig()
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"config": cfg})
	})

	group.PUT("/site", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input hostedadmin.SiteConfig
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("站点配置参数格式错误"))
			return
		}
		cfg, err := svc.SaveSiteConfig(user, input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"config": cfg})
	})

	group.GET("/audit", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		page, pageSize, err := adminPageParams(c)
		if err != nil {
			writeError(c, err)
			return
		}
		start, end, err := adminTimeRange(c)
		if err != nil {
			writeError(c, err)
			return
		}
		items, total, err := svc.ListAudit(repository.AdminAuditFilter{
			Keyword: strings.TrimSpace(c.Query("keyword")),
			Action:  strings.TrimSpace(c.Query("action")),
			Start:   start, End: end, Page: page, PageSize: pageSize,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, adminPage(items, total, page, pageSize))
	})
}

// RegisterPublicSiteRoutes exposes only the public branding and maintenance
// values needed before a visitor signs in.
func RegisterPublicSiteRoutes(r *gin.RouterGroup, svc HostedAdminService) {
	r.GET("/site/config", func(c *gin.Context) {
		cfg, err := svc.SiteConfig()
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"config": cfg})
	})
}

func adminPageParams(c *gin.Context) (int, int, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := queryInt(c, "pageSize", 30)
	if err != nil {
		return 0, 0, err
	}
	return page, pageSize, nil
}

func adminPage(items any, total int64, page int, pageSize int) gin.H {
	return gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize}
}

func adminTimeRange(c *gin.Context) (*time.Time, *time.Time, error) {
	start, err := adminQueryTime(c.Query("start"), false)
	if err != nil {
		return nil, nil, kernel.BadAuthRequest("开始时间格式错误")
	}
	end, err := adminQueryTime(c.Query("end"), true)
	if err != nil {
		return nil, nil, kernel.BadAuthRequest("结束时间格式错误")
	}
	return start, end, nil
}

func adminQueryTime(raw string, endOfDay bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		value := parsed.UTC()
		return &value, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return nil, err
	}
	value := parsed.UTC()
	if endOfDay {
		value = value.Add(24 * time.Hour)
	}
	return &value, nil
}
