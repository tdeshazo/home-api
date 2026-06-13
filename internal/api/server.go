package api

import (
	"log/slog"
	"net/http"

	"github.com/tdeshazo/home-api/internal/db"
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
	public := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, chain(h, common...))
	}
	publicHandler := func(pattern string, h http.Handler) {
		mux.Handle(pattern, chain(h, common...))
	}
	protected := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, chain(h, append(common, requireAuth)...))
	}

	public("GET /", s.frontend)
	publicHandler("GET /assets/", http.StripPrefix("/assets/", frontendAssets()))
	public("GET /healthz", s.healthz)
	public("POST /auth/register", s.register)
	public("POST /auth/login", s.login)
	public("POST /auth/refresh", s.refresh)
	public("POST /auth/logout", s.logout)
	public("GET /posts", s.listPosts)
	public("GET /posts/{id}", s.getPost)
	public("GET /posts/{id}/replies", s.listPostReplies)
	public("GET /users/{userID}/posts", s.listPostsByUser)

	protected("GET /me", s.me)
	protected("POST /api-keys", s.createAPIKey)
	protected("GET /me/posts", s.listMyPosts)
	protected("POST /posts", s.createPost)
	protected("POST /posts/{id}/replies", s.createReply)
	protected("PATCH /posts/{id}", s.updatePost)
	protected("DELETE /posts/{id}", s.deletePost)

	return mux
}
