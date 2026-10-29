package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHostedAssistantStatusAcceptsTrustedOrigin(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	router := gin.New()
	router.Use(RuntimeDependenciesMiddleware(RuntimeDependencies{}))
	RegisterAgentProxyRoutes(router.Group("/api"), env.service, env.clients, newUISessionStore(), env.assistantHost)

	request := httptest.NewRequest(http.MethodGet, "http://app.beeftv.example/api/assistant/status", nil)
	request.Host = "app.beeftv.example"
	request.RemoteAddr = "203.0.113.7:44321"
	request.Header.Set("Origin", "https://app.beeftv.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code == http.StatusForbidden {
		t.Fatalf("trusted hosted origin must not be rejected: %d %s", recorder.Code, recorder.Body.String())
	}
}
