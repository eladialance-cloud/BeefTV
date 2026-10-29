package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"infinite-canvas/backend/internal/agentops"
)

// 网页版画布提交必须能从“同源前端 + 已登录会话”这条路进来；
// 桌面形态则必须继续只认本机 loopback。
func TestTrustedHostedOriginRules(t *testing.T) {
	cases := []struct {
		name        string
		host        string
		origin      string
		allowed     string
		corsAllowed string
		want        bool
	}{
		{"same origin public host", "app.beeftv.example", "https://app.beeftv.example", "", "", true},
		{"same origin with port", "app.beeftv.example:8443", "https://app.beeftv.example:8443", "", "", true},
		{"explicitly allowed origin", "app.beeftv.example", "https://dev.beeftv.example", "https://dev.beeftv.example", "", true},
		{"cors origins are reused", "app.beeftv.example", "https://dev.beeftv.example", "", "https://dev.beeftv.example", true},
		{"wildcard never widens", "app.beeftv.example", "https://evil.example", "", "*", false},
		{"unlisted origin", "app.beeftv.example", "https://evil.example", "", "", false},
		{"missing origin", "app.beeftv.example", "", "", "", false},
		{"non http scheme", "app.beeftv.example", "wails://wails", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEEFTV_ALLOWED_ORIGINS", tc.allowed)
			t.Setenv("CANVAS_CORS_ORIGINS", tc.corsAllowed)
			req := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/api/ops/canvas.document.commit", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if got := trustedHostedOrigin(req); got != tc.want {
				t.Fatalf("trustedHostedOrigin=%v want %v", got, tc.want)
			}
		})
	}
}

func TestHostedOpsCommitAllowsTrustedOriginButDesktopStaysLoopbackOnly(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	hosted := gin.New()
	hosted.Use(RuntimeDependenciesMiddleware(RuntimeDependencies{}))
	RegisterAgentOpsRoutes(hosted.Group("/api"), env.service, agentops.NewStore(env.service.Database()), env.clients)

	body := `{"canvasId":"` + env.canvasID + `","expectedRevision":1,"document":{"title":"网页版提交","nodes":[{"id":"n1","type":"text","title":"网页"}],"connections":[]}}`

	code, response := postOpsFrom(t, hosted, "hosted-doc", "app.beeftv.example", "https://app.beeftv.example", body)
	if code != http.StatusOK {
		t.Fatalf("托管同源提交应放行: %d %s", code, response)
	}
	if !strings.Contains(response, `"caller":"manual"`) {
		t.Fatalf("托管前端写入应归属 manual: %s", response)
	}

	unlisted, unlistedBody := postOpsFrom(t, hosted, "hosted-unlisted", "app.beeftv.example", "https://evil.example", body)
	if unlisted != http.StatusForbidden {
		t.Fatalf("未放行 Origin 必须拒绝: %d %s", unlisted, unlistedBody)
	}

	// 桌面形态有 DesktopTrust：即使 Origin 同源，也不能绕过 loopback 判定。
	trust := func(*http.Request) bool { return true }
	desktop := gin.New()
	desktop.Use(RuntimeDependenciesMiddleware(RuntimeDependencies{DesktopTrust: trust}))
	RegisterAgentOpsRoutes(desktop.Group("/api"), env.service, agentops.NewStore(env.service.Database()), env.clients)
	desktopCode, desktopBody := postOpsFrom(t, desktop, "desktop-public", "app.beeftv.example", "https://app.beeftv.example", body)
	if desktopCode != http.StatusForbidden {
		t.Fatalf("桌面形态不得接受非本机来源: %d %s", desktopCode, desktopBody)
	}
}

func postOpsFrom(t *testing.T, router http.Handler, opID, host, origin, params string) (int, string) {
	t.Helper()
	body := `{"opId":` + strconvQuote(opID) + `,"params":` + params + `}`
	request := httptest.NewRequest(http.MethodPost, "http://"+host+"/api/ops/canvas.document.commit", strings.NewReader(body))
	request.Host = host
	request.RemoteAddr = "203.0.113.7:44321"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", origin)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}
