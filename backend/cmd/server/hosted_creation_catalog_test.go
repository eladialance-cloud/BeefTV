package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"infinite-canvas/backend/internal/bootstrap"
)

func TestHostedRuntimeRegistersCreationCatalogRoutes(t *testing.T) {
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

	tests := []struct {
		path string
		want int
	}{
		{path: "/api/creation-templates?kind=workflow", want: http.StatusOK},
		{path: "/api/admin/creation-catalog?kind=workflow", want: http.StatusUnauthorized},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		runtime.Handler().ServeHTTP(recorder, request)

		if recorder.Code == http.StatusNotFound {
			t.Errorf("%s was not registered", test.path)
			continue
		}
		if recorder.Code != test.want {
			t.Errorf("%s status = %d, want %d", test.path, recorder.Code, test.want)
		}
	}
}
