package api

import (
	"log/slog"
	"net/http"

	"github.com/tdeshazo/home-api/internal/db"
)

type Server struct {
	queries       *db.Queries
	txBeginner    txBeginner
	logger        *slog.Logger
	auth          Authenticator
	authConfig    AuthConfig
	authEndpoints authEndpointService
}

func NewServer(queries *db.Queries, txBeginner txBeginner, logger *slog.Logger, auth Authenticator, authConfig AuthConfig) *Server {
	return &Server{
		queries:       queries,
		txBeginner:    txBeginner,
		logger:        logger,
		auth:          auth,
		authConfig:    authConfig,
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

	public("GET /{$}", s.frontend)
	public("GET /login", s.loginFrontend)
	public("GET /tasks", s.tasksFrontend)
	public("GET /posts", s.frontend)
	public("GET /tasks/dashboard", s.taskDashboardFrontend)
	publicHandler("GET /assets/", http.StripPrefix("/assets/", frontendAssets()))

	public("GET /api/healthz", s.healthz)
	public("POST /api/auth/register", s.register)
	public("POST /api/auth/login", s.login)
	public("POST /api/auth/dev-login", s.devLogin)
	public("POST /api/auth/refresh", s.refresh)
	public("POST /api/auth/logout", s.logout)
	public("GET /api/posts", s.listPosts)
	public("GET /api/posts/{id}", s.getPost)
	public("GET /api/posts/{id}/replies", s.listPostReplies)
	public("GET /api/tasks", s.listTasks)
	public("GET /api/users/{userID}/posts", s.listPostsByUser)

	protected("POST /api/api-keys", s.createAPIKey)
	protected("GET /api/users", s.listUsers)
	protected("GET /api/me", s.me)
	protected("GET /api/me/tasks", s.listMyDailyTasks)
	protected("GET /api/me/posts", s.listMyPosts)
	protected("POST /api/posts", s.createPost)
	protected("POST /api/posts/{id}/replies", s.createReply)
	protected("PATCH /api/posts/{id}", s.updatePost)
	protected("DELETE /api/posts/{id}", s.deletePost)
	protected("POST /api/tasks", s.createTask)
	protected("POST /api/tasks/{id}/complete", s.completeTask)
	protected("PATCH /api/tasks/{id}", s.updateTask)
	protected("DELETE /api/tasks/{id}", s.deleteTask)
	protected("GET /api/tasks/dashboard/data", s.taskDashboardData)
	protected("POST /api/tasks/dashboard/users/{userID}/tasks/{id}/complete", s.completeDashboardTask)

	return mux
}
