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
			name:        "asset",
			path:        "/assets/app.js",
			contentType: "text/javascript",
			body:        "const state =",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
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
