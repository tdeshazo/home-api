package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tdeshazo/home-api/internal/auth"
	"github.com/tdeshazo/home-api/internal/db"

	"github.com/google/uuid"
)

type AuthConfig struct {
	// Environment is usually development, test, staging, or production.
	Environment string

	// Mode must be dev or jwt.
	Mode string

	// JWTSecret is used by the scaffold's HS256 JWT authenticator.
	// For a production IdP, prefer asymmetric signing and JWKS instead.
	JWTSecret string

	JWTIssuer   string
	JWTAudience string

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type Authenticator interface {
	Authenticate(ctx context.Context, bearerToken string) (db.User, error)
}

type apiKeyAuthenticator interface {
	AuthenticateAPIKey(ctx context.Context, apiKey string) (db.User, error)
}

func NewAuthenticator(queries *db.Queries, cfg AuthConfig) (Authenticator, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "dev"
	}

	environment := strings.ToLower(strings.TrimSpace(cfg.Environment))
	if environment == "" {
		environment = "development"
	}

	if environment == "production" && mode == "dev" {
		return nil, errors.New("AUTH_MODE=dev is not allowed when APP_ENV=production")
	}

	switch mode {
	case "dev":
		return DevAuthenticator{queries: queries}, nil
	case "jwt":
		if cfg.JWTSecret == "" {
			return nil, errors.New("JWT_SECRET is required when AUTH_MODE=jwt")
		}
		if cfg.JWTIssuer == "" {
			return nil, errors.New("JWT_ISSUER is required when AUTH_MODE=jwt")
		}
		if cfg.JWTAudience == "" {
			return nil, errors.New("JWT_AUDIENCE is required when AUTH_MODE=jwt")
		}

		return JWTAuthenticator{
			queries:  queries,
			secret:   []byte(cfg.JWTSecret),
			issuer:   cfg.JWTIssuer,
			audience: cfg.JWTAudience,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported AUTH_MODE %q; expected dev or jwt", cfg.Mode)
	}
}

type DevAuthenticator struct {
	queries *db.Queries
}

func (a DevAuthenticator) Authenticate(ctx context.Context, bearerToken string) (db.User, error) {
	const prefix = "dev:"
	if !strings.HasPrefix(bearerToken, prefix) {
		return db.User{}, errors.New("expected dev token format dev:<user_uuid>")
	}

	userID, err := uuid.Parse(strings.TrimPrefix(bearerToken, prefix))
	if err != nil {
		return db.User{}, errors.New("invalid dev user id")
	}

	user, err := a.queries.GetUser(ctx, userID)
	if err != nil {
		return db.User{}, errors.New("unknown user")
	}

	return user, nil
}

func (a DevAuthenticator) AuthenticateAPIKey(ctx context.Context, apiKey string) (db.User, error) {
	return authenticateAPIKey(ctx, a.queries, apiKey)
}

type JWTAuthenticator struct {
	queries  *db.Queries
	secret   []byte
	issuer   string
	audience string
}

func (a JWTAuthenticator) Authenticate(ctx context.Context, bearerToken string) (db.User, error) {
	userID, err := auth.ValidateJWTWithClaims(
		bearerToken,
		string(a.secret),
		a.issuer,
		a.audience,
	)
	if err != nil {
		return db.User{}, errors.New("invalid jwt")
	}

	user, err := a.queries.GetUser(ctx, userID)
	if err != nil {
		return db.User{}, errors.New("unknown user")
	}

	return user, nil
}

func (a JWTAuthenticator) AuthenticateAPIKey(ctx context.Context, apiKey string) (db.User, error) {
	return authenticateAPIKey(ctx, a.queries, apiKey)
}

func authenticateAPIKey(ctx context.Context, queries *db.Queries, apiKey string) (db.User, error) {
	if strings.TrimSpace(apiKey) == "" {
		return db.User{}, errors.New("empty api key")
	}

	userID, err := queries.TouchAPIKey(ctx, hashToken(apiKey))
	if err != nil {
		return db.User{}, errors.New("invalid api key")
	}

	user, err := queries.GetUser(ctx, userID)
	if err != nil {
		return db.User{}, errors.New("unknown api key user")
	}

	return user, nil
}
