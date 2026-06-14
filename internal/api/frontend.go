package api

import (
	"io/fs"
	"net/http"

	"github.com/tdeshazo/home-api/app"
)

func frontendAssets() http.Handler {
	sub, err := fs.Sub(app.Files, "assets")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func (s *Server) frontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, app.Files, "index.html")
}

func (s *Server) tasksFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, app.Files, "tasks.html")
}

func (s *Server) taskDashboardFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, app.Files, "tasks-dashboard.html")
}

func (s *Server) loginFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, app.Files, "login.html")
}
