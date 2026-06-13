package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeAuthEndpointService struct {
	register func(context.Context, registerRequest) (authTokenResponse, error)
	login    func(context.Context, loginRequest) (authTokenResponse, error)
	refresh  func(context.Context, refreshRequest) (authTokenResponse, error)
	logout   func(context.Context, logoutRequest) error
}

func (s fakeAuthEndpointService) Register(ctx context.Context, input registerRequest) (authTokenResponse, error) {
	if s.register == nil {
		return authTokenResponse{}, errors.New("unexpected register call")
	}
	return s.register(ctx, input)
}

func (s fakeAuthEndpointService) Login(ctx context.Context, input loginRequest) (authTokenResponse, error) {
	if s.login == nil {
		return authTokenResponse{}, errors.New("unexpected login call")
	}
	return s.login(ctx, input)
}

func (s fakeAuthEndpointService) Refresh(ctx context.Context, input refreshRequest) (authTokenResponse, error) {
	if s.refresh == nil {
		return authTokenResponse{}, errors.New("unexpected refresh call")
	}
	return s.refresh(ctx, input)
}

func (s fakeAuthEndpointService) Logout(ctx context.Context, input logoutRequest) error {
	if s.logout == nil {
		return errors.New("unexpected logout call")
	}
	return s.logout(ctx, input)
}

func TestValidateRegisterRequest(t *testing.T) {
	valid := registerRequest{
		Email:       "user@example.com",
		Handle:      "user_1",
		DisplayName: "User One",
		Password:    "password123",
	}

	tests := []struct {
		name  string
		input registerRequest
	}{
		{name: "valid", input: valid},
		{name: "invalid email", input: withRegisterEmail(valid, "not-email")},
		{name: "invalid handle", input: withRegisterHandle(valid, "User")},
		{name: "blank display name", input: withRegisterDisplayName(valid, " ")},
		{name: "short password", input: withRegisterPassword(valid, "short")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := normalizeRegisterRequest(tt.input)
			err := validateRegisterRequest(input)
			if tt.name == "valid" && err != nil {
				t.Fatalf("expected valid input, got %v", err)
			}
			if tt.name != "valid" && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestAuthHandlersMapErrors(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		service    fakeAuthEndpointService
		wantStatus int
		wantBody   string
	}{
		{
			name:       "register duplicate",
			path:       "/auth/register",
			body:       `{"email":"user@example.com","handle":"user_1","display_name":"User One","password":"password123"}`,
			wantStatus: http.StatusConflict,
			wantBody:   "email or handle already exists",
			service: fakeAuthEndpointService{
				register: func(context.Context, registerRequest) (authTokenResponse, error) {
					return authTokenResponse{}, errDuplicateUser
				},
			},
		},
		{
			name:       "login invalid credentials",
			path:       "/auth/login",
			body:       `{"email":"user@example.com","password":"password123"}`,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid email or password",
			service: fakeAuthEndpointService{
				login: func(context.Context, loginRequest) (authTokenResponse, error) {
					return authTokenResponse{}, errInvalidCredentials
				},
			},
		},
		{
			name:       "refresh invalid token",
			path:       "/auth/refresh",
			body:       `{"refresh_token":""}`,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid refresh token",
			service: fakeAuthEndpointService{
				refresh: func(context.Context, refreshRequest) (authTokenResponse, error) {
					return authTokenResponse{}, errInvalidRefreshToken
				},
			},
		},
		{
			name:       "auth unavailable",
			path:       "/auth/login",
			body:       `{"email":"user@example.com","password":"password123"}`,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "authentication is not configured",
			service: fakeAuthEndpointService{
				login: func(context.Context, loginRequest) (authTokenResponse, error) {
					return authTokenResponse{}, errAuthEndpointUnavailable
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{authEndpoints: tt.service}
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			switch tt.path {
			case "/auth/register":
				server.register(rec, req)
			case "/auth/login":
				server.login(rec, req)
			case "/auth/refresh":
				server.refresh(rec, req)
			default:
				t.Fatalf("unknown path %s", tt.path)
			}

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %s, want substring %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestAuthHandlersSuccess(t *testing.T) {
	response := authTokenResponse{
		User: userResponse{
			ID:          "00000000-0000-0000-0000-000000000001",
			Email:       "user@example.com",
			Handle:      "user_1",
			DisplayName: "User One",
			CreatedAt:   time.Unix(1, 0).UTC(),
			UpdatedAt:   time.Unix(1, 0).UTC(),
		},
		AccessToken:      "access",
		AccessExpiresAt:  time.Unix(2, 0).UTC(),
		RefreshToken:     "refresh-2",
		RefreshExpiresAt: time.Unix(3, 0).UTC(),
		TokenType:        "Bearer",
	}

	t.Run("register normalizes input and returns tokens", func(t *testing.T) {
		server := &Server{authEndpoints: fakeAuthEndpointService{
			register: func(_ context.Context, input registerRequest) (authTokenResponse, error) {
				if input.Email != "user@example.com" || input.DisplayName != "User One" {
					t.Fatalf("input was not normalized: %#v", input)
				}
				return response, nil
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":" USER@example.com ","handle":"user_1","display_name":" User One ","password":"password123"}`))
		rec := httptest.NewRecorder()

		server.register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"refresh_token":"refresh-2"`) {
			t.Fatalf("expected refresh token in body, got %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "password_hash") {
			t.Fatalf("response leaked password hash: %s", rec.Body.String())
		}
	})

	t.Run("refresh returns rotated token", func(t *testing.T) {
		server := &Server{authEndpoints: fakeAuthEndpointService{
			refresh: func(_ context.Context, input refreshRequest) (authTokenResponse, error) {
				if input.RefreshToken != "refresh-1" {
					t.Fatalf("refresh token = %q, want refresh-1", input.RefreshToken)
				}
				return response, nil
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"refresh_token":"refresh-1"}`))
		rec := httptest.NewRecorder()

		server.refresh(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"refresh_token":"refresh-2"`) {
			t.Fatalf("expected rotated token in body, got %s", rec.Body.String())
		}
	})

	t.Run("logout is idempotent", func(t *testing.T) {
		server := &Server{authEndpoints: fakeAuthEndpointService{
			logout: func(_ context.Context, input logoutRequest) error {
				if input.RefreshToken != "" {
					t.Fatalf("refresh token = %q, want empty", input.RefreshToken)
				}
				return nil
			},
		}}
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(`{"refresh_token":""}`))
		rec := httptest.NewRecorder()

		server.logout(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusNoContent, rec.Body.String())
		}
	})
}

func withRegisterEmail(input registerRequest, email string) registerRequest {
	input.Email = email
	return input
}

func withRegisterHandle(input registerRequest, handle string) registerRequest {
	input.Handle = handle
	return input
}

func withRegisterDisplayName(input registerRequest, displayName string) registerRequest {
	input.DisplayName = displayName
	return input
}

func withRegisterPassword(input registerRequest, password string) registerRequest {
	input.Password = password
	return input
}
