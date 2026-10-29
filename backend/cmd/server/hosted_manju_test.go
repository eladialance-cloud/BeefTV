package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"infinite-canvas/backend/internal/bootstrap"
)

func TestHostedRuntimeRegistersManjuProjectsRoute(t *testing.T) {
	runtime, err := bootstrap.Open(context.Background(), bootstrap.Config{
		Profile:         bootstrap.ProfileServer,
		DataDir:         t.TempDir(),
		ListenAddr:      "127.0.0.1:0",
		AutoMigrate:     true,
		HostedRoutes:    registerHostedRoutes,
		ShutdownTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/manju/projects", nil)
	runtime.Handler().ServeHTTP(recorder, request)

	if recorder.Code == http.StatusNotFound {
		t.Fatal("hosted runtime did not register GET /api/manju/projects")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/manju/projects status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
