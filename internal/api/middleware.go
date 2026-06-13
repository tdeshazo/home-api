package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	authpkg "github.com/tdeshazo/home-api/internal/auth"
	"github.com/tdeshazo/home-api/internal/db"
)

type Middleware func(http.Handler) http.Handler

const (
	logContextKey contextKey = "log_context"
	requestIDKey  contextKey = "request_id"
)

func chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// LogContext holds request-scoped values that should be included in access logs.
type LogContext struct {
	Username string
	Error    error
}

// SetLogError attaches an error to the request log entry, when request logging is enabled.
func SetLogError(ctx context.Context, err error) {
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Error = err
	}
}

// SetLogUsername attaches an authenticated username to the request log entry.
func SetLogUsername(ctx context.Context, username string) {
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Username = username
	}
}

// RequestIDFromContext returns the request ID assigned by middleware.
func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = rand.Text()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	bytesWritten int
	status       int
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytesWritten += n
	return n, err
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type bodyRecorder struct {
	io.ReadCloser
	bytesRead int
}

func (r *bodyRecorder) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.bytesRead += n
	return n, err
}

func loggingMiddleware(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			logCtx := &LogContext{}
			r = r.WithContext(context.WithValue(r.Context(), logContextKey, logCtx))

			body := &bodyRecorder{ReadCloser: r.Body}
			recorder := &statusRecorder{ResponseWriter: w}
			r.Body = body

			next.ServeHTTP(recorder, r)

			if recorder.status == 0 {
				recorder.status = http.StatusOK
			}

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("client_ip", r.RemoteAddr),
				slog.Duration("duration", time.Since(start)),
				slog.Int("request_body_bytes", body.bytesRead),
				slog.Int("response_status", recorder.status),
				slog.Int("response_body_bytes", recorder.bytesWritten),
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("user_agent", r.UserAgent()),
			}
			if logCtx.Username != "" {
				attrs = append(attrs, slog.String("user", logCtx.Username))
			}
			if logCtx.Error != nil {
				attrs = append(attrs, slog.Any("error", logCtx.Error))
			}

			logger.InfoContext(
				r.Context(),
				"served request",
				attrs...,
			)
		})
	}
}

func recoverMiddleware(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					err := fmt.Errorf("panic: %v", rec)
					SetLogError(r.Context(), err)

					logger.ErrorContext(
						r.Context(),
						"panic recovered",
						slog.Any("error", err),
						slog.String("stack", string(debug.Stack())),
						slog.String("request_id", RequestIDFromContext(r.Context())),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(w, `{"error":"an unexpected error occurred"}`+"\n")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func authMiddleware(auth Authenticator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := authenticateRequest(r.Context(), auth, r)
			if err != nil {
				SetLogError(r.Context(), err)
				writeError(w, http.StatusUnauthorized, "invalid or expired credentials")
				return
			}

			ctx := contextWithUser(r.Context(), user)
			SetLogUsername(ctx, user.Handle)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authenticateRequest(ctx context.Context, authenticator Authenticator, r *http.Request) (db.User, error) {
	var bearerAuthErr error
	bearerToken, bearerErr := authpkg.GetBearerToken(r.Header)
	if bearerErr == nil {
		if bearerToken == "" {
			return db.User{}, fmt.Errorf("empty bearer token")
		}
		user, err := authenticator.Authenticate(ctx, bearerToken)
		if err == nil {
			return user, nil
		}
		bearerAuthErr = err
	}

	apiKey, apiKeyErr := authpkg.GetAPIKey(r.Header)
	if apiKeyErr == nil {
		return authenticateAPIKeyRequest(ctx, authenticator, apiKey)
	}

	if bearerAuthErr != nil {
		user, err := authenticateAPIKeyRequest(ctx, authenticator, bearerToken)
		if err == nil {
			return user, nil
		}
		return db.User{}, bearerAuthErr
	}

	cookie, cookieErr := r.Cookie(accessCookieName)
	if cookieErr == nil && cookie.Value != "" {
		user, err := authenticator.Authenticate(ctx, cookie.Value)
		if err == nil {
			return user, nil
		}
		return db.User{}, err
	}

	return db.User{}, fmt.Errorf("expected Authorization: Bearer <token>, Authorization: ApiKey <key>, X-API-Key, or auth cookie")
}

func authenticateAPIKeyRequest(ctx context.Context, authenticator Authenticator, apiKey string) (db.User, error) {
	apiKeyAuth, ok := authenticator.(apiKeyAuthenticator)
	if !ok {
		return db.User{}, fmt.Errorf("api key authentication is not configured")
	}
	return apiKeyAuth.AuthenticateAPIKey(ctx, apiKey)
}
