package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"social-api/internal/db"

	"github.com/google/uuid"
)

type fakeCredentialAuthenticator struct {
	bearerToken string
	apiKey      string
	user        db.User
}

func (a fakeCredentialAuthenticator) Authenticate(_ context.Context, bearerToken string) (db.User, error) {
	if bearerToken != a.bearerToken {
		return db.User{}, errors.New("invalid bearer token")
	}
	return a.user, nil
}

func (a fakeCredentialAuthenticator) AuthenticateAPIKey(_ context.Context, apiKey string) (db.User, error) {
	if apiKey != a.apiKey {
		return db.User{}, errors.New("invalid api key")
	}
	return a.user, nil
}

func TestAuthenticateRequestAcceptsBearerAndAPIKey(t *testing.T) {
	user := db.User{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Handle: "user_1"}
	authenticator := fakeCredentialAuthenticator{
		bearerToken: "access-token",
		apiKey:      "api-key",
		user:        user,
	}

	tests := []struct {
		name      string
		header    string
		wantError bool
	}{
		{name: "bearer", header: "Bearer access-token"},
		{name: "api key", header: "ApiKey api-key"},
		{name: "invalid", header: "Basic something", wantError: true},
		{name: "wrong api key", header: "ApiKey wrong", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			headers.Set("Authorization", tt.header)

			gotUser, err := authenticateRequest(context.Background(), authenticator, headers)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("authenticate request: %v", err)
			}
			if gotUser.ID != user.ID {
				t.Fatalf("user ID = %s, want %s", gotUser.ID, user.ID)
			}
		})
	}
}
