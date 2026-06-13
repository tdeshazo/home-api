package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var frontendFiles embed.FS

func frontendAssets() http.Handler {
	sub, err := fs.Sub(frontendFiles, "static/assets")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func (s *Server) frontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, frontendFiles, "static/index.html")
}

func (s *Server) tasksFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, frontendFiles, "static/tasks.html")
}

func (s *Server) taskDashboardFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, frontendFiles, "static/tasks-dashboard.html")
}

func (s *Server) loginFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, frontendFiles, "static/login.html")
}
