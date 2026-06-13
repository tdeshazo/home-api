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
