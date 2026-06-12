package api

import (
	"log/slog"
	"net/http"

	"social-api/internal/db"
)

type Server struct {
	queries *db.Queries
	logger  *slog.Logger
	auth    Authenticator
}

func NewServer(queries *db.Queries, logger *slog.Logger, auth Authenticator) *Server {
	return &Server{
		queries: queries,
		logger:  logger,
		auth:    auth,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	logRequests := loggingMiddleware(s.logger)
	requireAuth := authMiddleware(s.auth)

	mux.Handle("GET /healthz", chain(http.HandlerFunc(s.healthz), logRequests))
	mux.Handle("GET /posts", chain(http.HandlerFunc(s.listPosts), logRequests))
	mux.Handle("GET /posts/{id}", chain(http.HandlerFunc(s.getPost), logRequests))
	mux.Handle("GET /users/{userID}/posts", chain(http.HandlerFunc(s.listPostsByUser), logRequests))

	mux.Handle("GET /me", chain(http.HandlerFunc(s.me), logRequests, requireAuth))
	mux.Handle("GET /me/posts", chain(http.HandlerFunc(s.listMyPosts), logRequests, requireAuth))
	mux.Handle("POST /posts", chain(http.HandlerFunc(s.createPost), logRequests, requireAuth))
	mux.Handle("PATCH /posts/{id}", chain(http.HandlerFunc(s.updatePost), logRequests, requireAuth))
	mux.Handle("DELETE /posts/{id}", chain(http.HandlerFunc(s.deletePost), logRequests, requireAuth))

	return mux
}
