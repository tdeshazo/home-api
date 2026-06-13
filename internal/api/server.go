package api

import (
	"log/slog"
	"net/http"

	"social-api/internal/db"
)

type Server struct {
	queries       *db.Queries
	logger        *slog.Logger
	auth          Authenticator
	authEndpoints authEndpointService
}

func NewServer(queries *db.Queries, txBeginner txBeginner, logger *slog.Logger, auth Authenticator, authConfig AuthConfig) *Server {
	return &Server{
		queries:       queries,
		logger:        logger,
		auth:          auth,
		authEndpoints: newLocalAuthService(queries, txBeginner, authConfig),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	requestID := requestIDMiddleware
	logRequests := loggingMiddleware(s.logger)
	recoverPanics := recoverMiddleware(s.logger)
	requireAuth := authMiddleware(s.auth)
	common := []Middleware{requestID, logRequests, recoverPanics}

	mux.Handle("GET /healthz", chain(http.HandlerFunc(s.healthz), common...))
	mux.Handle("POST /auth/register", chain(http.HandlerFunc(s.register), common...))
	mux.Handle("POST /auth/login", chain(http.HandlerFunc(s.login), common...))
	mux.Handle("POST /auth/refresh", chain(http.HandlerFunc(s.refresh), common...))
	mux.Handle("POST /auth/logout", chain(http.HandlerFunc(s.logout), common...))

	mux.Handle("GET /posts", chain(http.HandlerFunc(s.listPosts), common...))
	mux.Handle("GET /posts/{id}", chain(http.HandlerFunc(s.getPost), common...))
	mux.Handle("GET /users/{userID}/posts", chain(http.HandlerFunc(s.listPostsByUser), common...))

	mux.Handle("GET /me", chain(http.HandlerFunc(s.me), append(common, requireAuth)...))
	mux.Handle("GET /me/posts", chain(http.HandlerFunc(s.listMyPosts), append(common, requireAuth)...))
	mux.Handle("POST /posts", chain(http.HandlerFunc(s.createPost), append(common, requireAuth)...))
	mux.Handle("PATCH /posts/{id}", chain(http.HandlerFunc(s.updatePost), append(common, requireAuth)...))
	mux.Handle("DELETE /posts/{id}", chain(http.HandlerFunc(s.deletePost), append(common, requireAuth)...))

	return mux
}
