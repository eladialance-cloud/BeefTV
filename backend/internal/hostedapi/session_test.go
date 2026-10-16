package hostedapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	handlercore "infinite-canvas/backend/internal/handler"

	"github.com/gin-gonic/gin"
)

// probeEngine mounts the session middleware in front of a probe route that
// reports what the middleware published. It mirrors how cmd/server installs the
// middleware globally, without needing a full hosted runtime.
func probeEngine(svc AuthService) (*gin.Engine, *probeResult) {
	result := &probeResult{}
	engine := gin.New()
	engine.Use(SessionMiddleware(svc))
	engine.GET("/probe", func(c *gin.Context) {
		result.hosted = handlercore.IsHostedRequest(c)
		if user, ok := handlercore.SessionUser(c); ok {
			result.userID = user.ID
			result.username = user.Username
		}
		c.Status(http.StatusOK)
	})
	return engine, result
}

type probeResult struct {
	hosted   bool
	userID   string
	username string
}

func callProbe(t *testing.T, engine *gin.Engine, cookie *http.Cookie) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	engine.ServeHTTP(httptest.NewRecorder(), request)
}

func TestSessionMiddlewareMarksEveryRequestAndPublishesValidSession(t *testing.T) {
	api, svc := newAuthTestEnv(t)
	registered := authRequest(t, api, http.MethodPost, "/api/register", map[string]any{
		"username": "alice", "password": "password-1",
	}, nil)
	cookie := sessionCookie(t, registered)
	if cookie == nil {
		t.Fatal("注册后应拿到会话 cookie")
	}

	engine, result := probeEngine(svc)

	callProbe(t, engine, cookie)
	if !result.hosted {
		t.Fatal("带 cookie 的请求也必须被标记为托管请求")
	}
	if result.username != "alice" || result.userID == "" {
		t.Fatalf("有效会话应发布登录用户，得到 %#v", result)
	}
}

func TestSessionMiddlewareMarkWithoutPublishingWhenUnauthenticated(t *testing.T) {
	_, svc := newAuthTestEnv(t)
	engine, result := probeEngine(svc)

	callProbe(t, engine, nil)
	if !result.hosted {
		t.Fatal("未登录请求也必须被标记为托管请求，否则会回退到共享 owner")
	}
	if result.userID != "" {
		t.Fatalf("未登录请求不得发布会话用户，得到 %#v", result)
	}
}

func TestSessionMiddlewareIgnoresUnknownCookie(t *testing.T) {
	_, svc := newAuthTestEnv(t)
	engine, result := probeEngine(svc)

	callProbe(t, engine, &http.Cookie{Name: SessionCookieName, Value: "not-a-real-token"})
	if !result.hosted {
		t.Fatal("伪造 cookie 的请求仍应被标记为托管请求")
	}
	if result.userID != "" {
		t.Fatalf("未知令牌不得发布会话用户，得到 %#v", result)
	}
}

// 两名用户必须解析成两个不同的身份：这是「互相看不见对方数据」的地基。
func TestSessionMiddlewareSeparatesTwoUsers(t *testing.T) {
	api, svc := newAuthTestEnv(t)
	alice := sessionCookie(t, authRequest(t, api, http.MethodPost, "/api/register", map[string]any{
		"username": "alice", "password": "password-1",
	}, nil))
	bob := sessionCookie(t, authRequest(t, api, http.MethodPost, "/api/register", map[string]any{
		"username": "bob", "password": "password-1",
	}, nil))
	if alice == nil || bob == nil {
		t.Fatal("两名用户都应拿到会话 cookie")
	}

	engine, result := probeEngine(svc)

	callProbe(t, engine, alice)
	aliceID, aliceName := result.userID, result.username
	callProbe(t, engine, bob)

	if aliceName != "alice" || result.username != "bob" {
		t.Fatalf("会话解析用户不符: alice=%q bob=%q", aliceName, result.username)
	}
	if aliceID == "" || result.userID == "" || aliceID == result.userID {
		t.Fatalf("两名用户必须解析成不同身份: %q vs %q", aliceID, result.userID)
	}
}
