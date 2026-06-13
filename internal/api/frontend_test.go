package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendRoutes(t *testing.T) {
	server := &Server{}
	routes := server.Routes()

	tests := []struct {
		name        string
		path        string
		contentType string
		body        string
	}{
		{
			name:        "index",
			path:        "/",
			contentType: "text/html",
			body:        `<script src="/assets/app.js" type="module"></script>`,
		},
		{
			name:        "posts route",
			path:        "/posts",
			contentType: "text/html",
			body:        `<script src="/assets/app.js" type="module"></script>`,
		},
		{
			name:        "tasks route",
			path:        "/tasks",
			contentType: "text/html",
			body:        `<script src="/assets/tasks.js" type="module"></script>`,
		},
		{
			name:        "tasks dashboard route",
			path:        "/tasks/dashboard",
			contentType: "text/html",
			body:        `<script src="/assets/tasks-dashboard.js" type="module"></script>`,
		},
		{
			name:        "asset",
			path:        "/assets/app.js",
			contentType: "text/javascript",
			body:        "const state =",
		},
		{
			name:        "tasks asset",
			path:        "/assets/tasks.js",
			contentType: "text/javascript",
			body:        "const state =",
		},
		{
			name:        "tasks dashboard asset",
			path:        "/assets/tasks-dashboard.js",
			contentType: "text/javascript",
			body:        "const state =",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept", "text/html")
			rec := httptest.NewRecorder()

			routes.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Fatalf("content-type = %q, want substring %q", got, tt.contentType)
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Fatalf("body did not contain %q", tt.body)
			}
		})
	}
}

func TestBackendRoutesUseAPIPrefix(t *testing.T) {
	server := &Server{}
	routes := server.Routes()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "posts page remains frontend",
			path:       "/posts",
			wantStatus: http.StatusOK,
			wantBody:   `<script src="/assets/app.js" type="module"></script>`,
		},
		{
			name:       "tasks page remains frontend",
			path:       "/tasks",
			wantStatus: http.StatusOK,
			wantBody:   `<script src="/assets/tasks.js" type="module"></script>`,
		},
		{
			name:       "api health route is prefixed",
			path:       "/api/healthz",
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
		{
			name:       "old health route is not registered",
			path:       "/healthz",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept", "application/json")
			rec := httptest.NewRecorder()

			routes.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body did not contain %q", tt.wantBody)
			}
		})
	}
}
