package hostedapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/hostedadmin"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type fakeHostedAdminService struct {
	created      bool
	ledgerFilter repository.AdminLedgerFilter
}

func (f *fakeHostedAdminService) ListUsers(repository.AdminUserFilter) ([]repository.AdminUserRow, int64, error) {
	return nil, 0, nil
}
func (f *fakeHostedAdminService) CreateUser(*model.User, hostedadmin.CreateUserInput) (*repository.AdminUserRow, error) {
	f.created = true
	return &repository.AdminUserRow{ID: "created", Username: "created", Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
}
func (f *fakeHostedAdminService) UpdateUser(*model.User, string, hostedadmin.UpdateUserInput) (*repository.AdminUserRow, error) {
	return nil, nil
}
func (f *fakeHostedAdminService) ResetUserPassword(*model.User, string, string) error { return nil }
func (f *fakeHostedAdminService) ListProjects(repository.AdminProjectFilter) ([]repository.AdminProjectRow, int64, error) {
	return nil, 0, nil
}
func (f *fakeHostedAdminService) SetProjectArchived(*model.User, string, string, bool, string) error {
	return nil
}
func (f *fakeHostedAdminService) ListAssets(repository.AdminAssetFilter) ([]repository.AdminAssetRow, int64, error) {
	return nil, 0, nil
}
func (f *fakeHostedAdminService) ModerateAsset(*model.User, string, bool, string) error { return nil }
func (f *fakeHostedAdminService) ListLedger(filter repository.AdminLedgerFilter) ([]repository.AdminLedgerRow, int64, error) {
	f.ledgerFilter = filter
	return nil, 0, nil
}
func (f *fakeHostedAdminService) ListAudit(repository.AdminAuditFilter) ([]repository.AdminAuditRow, int64, error) {
	return nil, 0, nil
}
func (f *fakeHostedAdminService) Overview(int) (repository.AdminOverviewMetrics, error) {
	return repository.AdminOverviewMetrics{}, nil
}
func (f *fakeHostedAdminService) SiteConfig() (hostedadmin.SiteConfig, error) {
	return hostedadmin.SiteConfig{SiteName: "VisiDeep AI"}, nil
}
func (f *fakeHostedAdminService) SaveSiteConfig(*model.User, hostedadmin.SiteConfig) (hostedadmin.SiteConfig, error) {
	return hostedadmin.SiteConfig{SiteName: "VisiDeep AI"}, nil
}

func newHostedAdminRouteTestEngine(svc HostedAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	resolve := func(c *gin.Context) (*model.User, error) {
		switch c.GetHeader("X-Test-Actor") {
		case "admin":
			return &model.User{ID: "admin-1", Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}, nil
		case "user":
			return &model.User{ID: "user-1", Username: "user", Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
		default:
			return nil, nil
		}
	}
	guard := func(user *model.User) error {
		if user == nil {
			return kernel.Unauthorized("请先登录")
		}
		if user.Role != model.UserRoleAdmin {
			return kernel.Forbidden("需要管理员权限")
		}
		return nil
	}
	RegisterHostedAdminRoutes(engine.Group("/api"), svc, resolve, guard)
	RegisterPublicSiteRoutes(engine.Group("/api"), svc)
	return engine
}

func adminRouteRequest(engine *gin.Engine, method string, path string, body string, actor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Test-Actor", actor)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestHostedAdminRoutesRejectOrdinaryUsers(t *testing.T) {
	engine := newHostedAdminRouteTestEngine(&fakeHostedAdminService{})
	rec := adminRouteRequest(engine, http.MethodGet, "/api/admin/users", "", "user")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("普通用户访问管理接口应返回 403，得到 %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHostedAdminRoutesRejectAnonymousUsers(t *testing.T) {
	engine := newHostedAdminRouteTestEngine(&fakeHostedAdminService{})
	rec := adminRouteRequest(engine, http.MethodGet, "/api/admin/users", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("匿名访问管理接口应返回 401，得到 %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHostedAdminRoutesCoverEveryConsolePage(t *testing.T) {
	engine := newHostedAdminRouteTestEngine(&fakeHostedAdminService{})
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/admin/overview", ""},
		{http.MethodGet, "/api/admin/users?keyword=alice&role=user&status=active", ""},
		{http.MethodGet, "/api/admin/projects?kind=canvas&status=active", ""},
		{http.MethodGet, "/api/admin/assets?kind=image&status=blocked", ""},
		{http.MethodGet, "/api/admin/credit-ledger?kind=consume&start=2026-10-01&end=2026-10-18", ""},
		{http.MethodGet, "/api/admin/site", ""},
		{http.MethodGet, "/api/admin/audit?action=site.update", ""},
		{http.MethodPatch, "/api/admin/users/u1", `{"role":"user","status":"active"}`},
		{http.MethodPost, "/api/admin/users/u1/password", `{"password":"password-2"}`},
		{http.MethodPost, "/api/admin/projects/p1/archive", `{"kind":"domain","reason":"违规"}`},
		{http.MethodPost, "/api/admin/projects/p1/restore", `{"kind":"domain"}`},
		{http.MethodPost, "/api/admin/assets/a1/block", `{"reason":"违规"}`},
		{http.MethodPost, "/api/admin/assets/a1/unblock", `{}`},
		{http.MethodPut, "/api/admin/site", `{"siteName":"VisiDeep AI","creditName":"积分","currency":"CNY"}`},
	}
	for _, tc := range cases {
		rec := adminRouteRequest(engine, tc.method, tc.path, tc.body, "admin")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s 应返回 200，得到 %d (%s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestHostedAdminLedgerPassesModelAndTokenFilters(t *testing.T) {
	service := &fakeHostedAdminService{}
	engine := newHostedAdminRouteTestEngine(service)
	recorder := adminRouteRequest(
		engine,
		http.MethodGet,
		"/api/admin/credit-ledger?keyword=alice&kind=consume&model=seedance&token=video&start=2026-10-01&end=2026-10-22",
		"",
		"admin",
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ledger status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if service.ledgerFilter.Keyword != "alice" || service.ledgerFilter.Kind != "consume" ||
		service.ledgerFilter.Model != "seedance" || service.ledgerFilter.Token != "video" {
		t.Fatalf("ledger filter=%+v", service.ledgerFilter)
	}
}

func TestHostedAdminCreateUserDoesNotIssueSessionCookie(t *testing.T) {
	svc := &fakeHostedAdminService{}
	engine := newHostedAdminRouteTestEngine(svc)
	rec := adminRouteRequest(engine, http.MethodPost, "/api/admin/users", `{"username":"new","password":"password-1"}`, "admin")
	if rec.Code != http.StatusOK || !svc.created {
		t.Fatalf("管理员创建用户失败: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("创建用户不得替换管理员会话: %v", rec.Result().Cookies())
	}
}

func TestPublicSiteConfigNeedsNoAdmin(t *testing.T) {
	engine := newHostedAdminRouteTestEngine(&fakeHostedAdminService{})
	rec := adminRouteRequest(engine, http.MethodGet, "/api/site/config", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "VisiDeep AI") {
		t.Fatalf("公开站点配置读取失败: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminOverviewAcceptsDays(t *testing.T) {
	engine := newHostedAdminRouteTestEngine(&fakeHostedAdminService{})
	rec := adminRouteRequest(engine, http.MethodGet, "/api/admin/overview?days=30", "", "admin")
	if rec.Code != http.StatusOK {
		t.Fatalf("运营概览应返回 200，得到 %d (%s)", rec.Code, rec.Body.String())
	}
}
