package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tdeshazo/home-api/internal/auth"
	"github.com/tdeshazo/home-api/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxAPIKeyIssueAttempts = 3

type apiKeyResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type createAPIKeyResponse struct {
	APIKey              apiKeyResponse `json:"api_key"`
	Key                 string         `json:"key"`
	AuthorizationHeader string         `json:"authorization_header"`
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if err := validateCreateAPIKeyName(input.Name); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.issueAPIKey(r.Context(), user.ID, input.Name)
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not create api key")
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func validateCreateAPIKeyName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 100 {
		return errors.New("name must be 100 characters or fewer")
	}
	return nil
}

func (s *Server) issueAPIKey(ctx context.Context, userID uuid.UUID, name string) (createAPIKeyResponse, error) {
	for attempt := 0; attempt < maxAPIKeyIssueAttempts; attempt++ {
		key, err := auth.MakeRefreshToken()
		if err != nil {
			return createAPIKeyResponse{}, fmt.Errorf("make api key: %w", err)
		}

		row, err := s.queries.CreateAPIKey(ctx, db.CreateAPIKeyParams{
			UserID:  userID,
			Name:    name,
			KeyHash: hashToken(key),
		})
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return createAPIKeyResponse{}, fmt.Errorf("store api key: %w", err)
		}

		return createAPIKeyResponse{
			APIKey:              publicAPIKey(row),
			Key:                 key,
			AuthorizationHeader: "Authorization: Bearer " + key,
		}, nil
	}

	return createAPIKeyResponse{}, errors.New("could not generate unique api key")
}

func publicAPIKey(row db.CreateAPIKeyRow) apiKeyResponse {
	return apiKeyResponse{
		ID:         row.ID.String(),
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		LastUsedAt: nullableTime(row.LastUsedAt),
		RevokedAt:  nullableTime(row.RevokedAt),
	}
}

func nullableTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
