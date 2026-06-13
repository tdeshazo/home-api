package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tdeshazo/home-api/internal/auth"
	"github.com/tdeshazo/home-api/internal/db"

	"github.com/lib/pq"
)

var (
	errAuthEndpointUnavailable = errors.New("auth endpoints require AUTH_MODE=jwt with JWT configuration")
	errDuplicateUser           = errors.New("user already exists")
	errInvalidCredentials      = errors.New("invalid credentials")
	errInvalidRefreshToken     = errors.New("invalid refresh token")
)

type txBeginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type authTokenResponse struct {
	User             userResponse `json:"user"`
	AccessToken      string       `json:"access_token"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshToken     string       `json:"refresh_token"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
	TokenType        string       `json:"token_type"`
}

type userResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func publicUser(user db.User) userResponse {
	return userResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		Handle:      user.Handle,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

type authEndpointService interface {
	Register(context.Context, registerRequest) (authTokenResponse, error)
	Login(context.Context, loginRequest) (authTokenResponse, error)
	Refresh(context.Context, refreshRequest) (authTokenResponse, error)
	Logout(context.Context, logoutRequest) error
}

type localAuthService struct {
	queries         *db.Queries
	txBeginner      txBeginner
	enabled         bool
	jwtSecret       string
	jwtIssuer       string
	jwtAudience     string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func newLocalAuthService(queries *db.Queries, txBeginner txBeginner, cfg AuthConfig) *localAuthService {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	enabled := mode == "jwt" && cfg.JWTSecret != "" && cfg.JWTIssuer != "" && cfg.JWTAudience != ""

	return &localAuthService{
		queries:         queries,
		txBeginner:      txBeginner,
		enabled:         enabled,
		jwtSecret:       cfg.JWTSecret,
		jwtIssuer:       cfg.JWTIssuer,
		jwtAudience:     cfg.JWTAudience,
		accessTokenTTL:  cfg.AccessTokenTTL,
		refreshTokenTTL: cfg.RefreshTokenTTL,
	}
}

func (s *localAuthService) Register(ctx context.Context, input registerRequest) (authTokenResponse, error) {
	if err := s.checkEnabled(); err != nil {
		return authTokenResponse{}, err
	}

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return authTokenResponse{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        input.Email,
		Handle:       input.Handle,
		DisplayName:  input.DisplayName,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return authTokenResponse{}, errDuplicateUser
		}
		return authTokenResponse{}, fmt.Errorf("create user: %w", err)
	}

	return s.issueTokenPair(ctx, s.queries, user)
}

func (s *localAuthService) Login(ctx context.Context, input loginRequest) (authTokenResponse, error) {
	if err := s.checkEnabled(); err != nil {
		return authTokenResponse{}, err
	}

	user, err := s.queries.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return authTokenResponse{}, errInvalidCredentials
	}

	ok, err := auth.CheckPasswordHash(input.Password, user.PasswordHash)
	if err != nil {
		return authTokenResponse{}, errInvalidCredentials
	}
	if !ok {
		return authTokenResponse{}, errInvalidCredentials
	}

	return s.issueTokenPair(ctx, s.queries, user)
}

func (s *localAuthService) Refresh(ctx context.Context, input refreshRequest) (authTokenResponse, error) {
	if err := s.checkEnabled(); err != nil {
		return authTokenResponse{}, err
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		return authTokenResponse{}, errInvalidRefreshToken
	}
	if s.txBeginner == nil {
		return authTokenResponse{}, errors.New("auth transaction manager is not configured")
	}

	tx, err := s.txBeginner.BeginTx(ctx, nil)
	if err != nil {
		return authTokenResponse{}, fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := s.queries.WithTx(tx)
	tokenHash := hashToken(input.RefreshToken)
	refreshToken, err := queries.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return authTokenResponse{}, errInvalidRefreshToken
	}
	if refreshToken.RevokedAt.Valid || !time.Now().UTC().Before(refreshToken.ExpiresAt) {
		return authTokenResponse{}, errInvalidRefreshToken
	}

	user, err := queries.GetUser(ctx, refreshToken.UserID)
	if err != nil {
		return authTokenResponse{}, errInvalidRefreshToken
	}

	if err := queries.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return authTokenResponse{}, fmt.Errorf("revoke refresh token: %w", err)
	}

	response, err := s.issueTokenPair(ctx, queries, user)
	if err != nil {
		return authTokenResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return authTokenResponse{}, fmt.Errorf("commit refresh transaction: %w", err)
	}

	return response, nil
}

func (s *localAuthService) Logout(ctx context.Context, input logoutRequest) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		return nil
	}

	if err := s.queries.RevokeRefreshToken(ctx, hashToken(input.RefreshToken)); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (s *localAuthService) checkEnabled() error {
	if !s.enabled {
		return errAuthEndpointUnavailable
	}
	if s.accessTokenTTL <= 0 || s.refreshTokenTTL <= 0 {
		return errAuthEndpointUnavailable
	}
	return nil
}

func (s *localAuthService) issueTokenPair(ctx context.Context, queries *db.Queries, user db.User) (authTokenResponse, error) {
	now := time.Now().UTC()
	accessExpiresAt := now.Add(s.accessTokenTTL)
	refreshExpiresAt := now.Add(s.refreshTokenTTL)

	accessToken, err := auth.MakeJWTWithClaims(user.ID, s.jwtSecret, s.accessTokenTTL, s.jwtIssuer, s.jwtAudience)
	if err != nil {
		return authTokenResponse{}, fmt.Errorf("make access token: %w", err)
	}

	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		return authTokenResponse{}, fmt.Errorf("make refresh token: %w", err)
	}

	if _, err := queries.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		TokenHash: hashToken(refreshToken),
		UserID:    user.ID,
		ExpiresAt: refreshExpiresAt,
	}); err != nil {
		return authTokenResponse{}, fmt.Errorf("store refresh token: %w", err)
	}

	return authTokenResponse{
		User:             publicUser(user),
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiresAt,
		TokenType:        "Bearer",
	}, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && string(pqErr.Code) == "23505"
}
