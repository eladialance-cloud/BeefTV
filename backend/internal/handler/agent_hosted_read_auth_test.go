package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func hostedAssistantReadRequest(t *testing.T, env *assistantTestEnv, method, path, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "https://zt.shentongapi.cn/api"+path, nil)
	request.Host = "zt.shentongapi.cn"
	request.RemoteAddr = "203.0.113.7:43120"
	request.Header.Set("X-Beeftv-Ui-Session", env.uiToken)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	env.router.ServeHTTP(recorder, request)
	return recorder
}

func useHostedAssistantRouter(t *testing.T, env *assistantTestEnv) {
	t.Helper()
	owner, err := env.service.LocalWorkspaceOwner()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(RuntimeDependenciesMiddleware(RuntimeDependencies{AssistantHost: env.assistantHost}))
	router.Use(func(c *gin.Context) {
		MarkHostedRequest(c)
		SetSessionUser(c, owner)
		c.Next()
	})
	RegisterAgentProxyRoutes(router.Group("/api"), env.service, env.clients, newUISessionStoreForUser(env), env.assistantHost)
	env.router = router
}

func newUISessionStoreForUser(env *assistantTestEnv) *uiSessionStore {
	ownerID := ""
	if owner, err := env.service.LocalWorkspaceOwner(); err == nil {
		ownerID = owner.ID
	}
	store := newUISessionStore()
	session := store.issue(ownerID)
	env.uiToken = session.Token
	return store
}

func TestHostedAssistantStatusAcceptsAuthenticatedSameSiteGetWithoutOrigin(t *testing.T) {
	env := newAssistantTestEnv(t, func(env *assistantTestEnv) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"busy":false,"model":"MiniMax-M3","instance":"` + assistantInstanceProof() + `"}`))
		})
	})
	useEnvProvider(t, "chat-completion")
	useHostedAssistantRouter(t, env)

	recorder := hostedAssistantReadRequest(t, env, http.MethodGet, "/assistant/status", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("authenticated same-site GET without Origin must be accepted: %d %s", recorder.Code, recorder.Body.String())
	}
	data := decodeEnvelope(t, recorder)
	if available, _ := data["available"].(bool); !available {
		t.Fatalf("host should be reported ready: %#v", data)
	}
}

func TestHostedAssistantHistoryAcceptsAuthenticatedSameSiteGetWithoutOrigin(t *testing.T) {
	env := newAssistantTestEnv(t, func(env *assistantTestEnv) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/history" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sessionId":"s1","turns":[]}`))
		})
	})
	useHostedAssistantRouter(t, env)

	recorder := hostedAssistantReadRequest(t, env, http.MethodGet, "/assistant/history?canvasId="+env.canvasID, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("authenticated same-site history GET without Origin must be accepted: %d %s", recorder.Code, recorder.Body.String())
	}
	if data := decodeEnvelope(t, recorder); data["sessionId"] != "s1" {
		t.Fatalf("history should be proxied: %#v", data)
	}
}

func TestHostedAssistantReadStillRejectsAuthenticatedCrossOrigin(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	useHostedAssistantRouter(t, env)

	recorder := hostedAssistantReadRequest(t, env, http.MethodGet, "/assistant/status", "https://attacker.example")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin assistant read must remain forbidden: %d %s", recorder.Code, recorder.Body.String())
	}
}
