package auth_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-api/internal/auth"

	"github.com/google/uuid"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	ok, err := auth.CheckPasswordHash("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("check password hash: %v", err)
	}
	if !ok {
		t.Fatal("expected password to match hash")
	}

	ok, err = auth.CheckPasswordHash("wrong password", hash)
	if err != nil {
		t.Fatalf("check wrong password hash: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password not to match hash")
	}
}

func TestJWT(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	token, err := auth.MakeJWT(userID, "test-secret", time.Minute)
	if err != nil {
		t.Fatalf("make jwt: %v", err)
	}
	if parts := strings.Split(token, "."); len(parts) != 3 {
		t.Fatalf("expected three token parts, got %d", len(parts))
	}

	gotUserID, err := auth.ValidateJWT(token, "test-secret")
	if err != nil {
		t.Fatalf("validate jwt: %v", err)
	}
	if gotUserID != userID {
		t.Fatalf("user ID = %s, want %s", gotUserID, userID)
	}

	if _, err := auth.ValidateJWT(token, "wrong-secret"); err == nil {
		t.Fatal("expected wrong secret to fail validation")
	}
}

func TestJWTValidateWithClaims(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	token, err := auth.MakeJWT(userID, "test-secret", time.Minute)
	if err != nil {
		t.Fatalf("make jwt: %v", err)
	}

	gotUserID, err := auth.ValidateJWTWithClaims(token, "test-secret", string(auth.TokenTypeAccess), "")
	if err != nil {
		t.Fatalf("validate jwt with claims: %v", err)
	}
	if gotUserID != userID {
		t.Fatalf("user ID = %s, want %s", gotUserID, userID)
	}

	if _, err := auth.ValidateJWTWithClaims(token, "test-secret", "invalid-issuer", ""); err == nil {
		t.Fatal("expected invalid issuer to fail validation")
	}

	claimedToken, err := auth.MakeJWTWithClaims(userID, "test-secret", time.Minute, "social-api", "social-api-api")
	if err != nil {
		t.Fatalf("make jwt with claims: %v", err)
	}
	if _, err := auth.ValidateJWTWithClaims(claimedToken, "test-secret", "social-api", "social-api-api"); err != nil {
		t.Fatalf("validate jwt with issuer and audience: %v", err)
	}
}

func TestJWTValidation(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if _, err := auth.MakeJWT(uuid.Nil, "test-secret", time.Hour); err == nil {
		t.Fatal("make jwt accepted an empty user ID")
	}
	if _, err := auth.MakeJWT(userID, "", time.Hour); err == nil {
		t.Fatal("make jwt accepted an empty secret")
	}
	if _, err := auth.MakeJWT(userID, "test-secret", 0); err == nil {
		t.Fatal("make jwt accepted a non-positive expiration")
	}
	if _, err := auth.ValidateJWT("malformed", "test-secret"); err == nil {
		t.Fatal("validate jwt accepted malformed token")
	}
}

func TestAuthorizationHeaders(t *testing.T) {
	headers := http.Header{}
	if _, err := auth.GetBearerToken(headers); !errors.Is(err, auth.ErrNoAuthHeaderIncluded) {
		t.Fatalf("expected missing bearer token error, got %v", err)
	}

	headers.Set("Authorization", "Bearer access-token")
	bearer, err := auth.GetBearerToken(headers)
	if err != nil {
		t.Fatalf("get bearer token: %v", err)
	}
	if bearer != "access-token" {
		t.Fatalf("expected bearer token, got %q", bearer)
	}

	headers.Set("Authorization", "Bearer access-token extra")
	if _, err := auth.GetBearerToken(headers); err == nil {
		t.Fatal("expected malformed bearer token to fail")
	}

	headers.Set("Authorization", "ApiKey api-key")
	apiKey, err := auth.GetAPIKey(headers)
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if apiKey != "api-key" {
		t.Fatalf("expected api key, got %q", apiKey)
	}
}

func TestMakeRefreshToken(t *testing.T) {
	token, err := auth.MakeRefreshToken()
	if err != nil {
		t.Fatalf("make refresh token: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("expected 64 hex characters, got %d", len(token))
	}
}
