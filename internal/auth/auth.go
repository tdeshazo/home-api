package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
)

type TokenType string

const (
	TokenTypeAccess TokenType = "access"
)

var ErrNoAuthHeaderIncluded = errors.New("no auth header included in request")

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type jwtClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return match, nil
}

func MakeJWT(
	userID uuid.UUID,
	tokenSecret string,
	expiresIn time.Duration,
) (string, error) {
	return MakeJWTWithClaims(userID, tokenSecret, expiresIn, string(TokenTypeAccess), "")
}

func MakeJWTWithClaims(
	userID uuid.UUID,
	tokenSecret string,
	expiresIn time.Duration,
	issuer string,
	audience string,
) (string, error) {
	if userID == uuid.Nil {
		return "", errors.New("user id is required")
	}
	if tokenSecret == "" {
		return "", errors.New("token secret is required")
	}
	if expiresIn <= 0 {
		return "", errors.New("token expiration must be greater than zero")
	}

	now := time.Now().UTC()
	header, err := encodeJWTPart(jwtHeader{
		Algorithm: "HS256",
		Type:      "JWT",
	})
	if err != nil {
		return "", err
	}
	claims, err := encodeJWTPart(jwtClaims{
		Issuer:    issuer,
		Audience:  audience,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(expiresIn).Unix(),
		Subject:   userID.String(),
	})
	if err != nil {
		return "", err
	}

	signingInput := header + "." + claims
	signature := signJWT(signingInput, []byte(tokenSecret))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	return ValidateJWTWithClaims(tokenString, tokenSecret, string(TokenTypeAccess), "")
}

func ValidateJWTWithClaims(
	tokenString,
	tokenSecret,
	expectedIssuer,
	expectedAudience string,
) (uuid.UUID, error) {
	if tokenSecret == "" {
		return uuid.Nil, errors.New("token secret is required")
	}

	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return uuid.Nil, errors.New("malformed token")
	}

	var header jwtHeader
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return uuid.Nil, fmt.Errorf("decode token header: %w", err)
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return uuid.Nil, errors.New("unsupported token header")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode token signature: %w", err)
	}
	expected := signJWT(parts[0]+"."+parts[1], []byte(tokenSecret))
	if !hmac.Equal(signature, expected) {
		return uuid.Nil, errors.New("invalid token signature")
	}

	var claims jwtClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return uuid.Nil, fmt.Errorf("decode token claims: %w", err)
	}
	if expectedIssuer != "" && claims.Issuer != expectedIssuer {
		return uuid.Nil, errors.New("invalid issuer")
	}
	if expectedAudience != "" && claims.Audience != expectedAudience {
		return uuid.Nil, errors.New("invalid audience")
	}

	now := time.Now().UTC().Unix()
	if claims.ExpiresAt <= now {
		return uuid.Nil, errors.New("token expired")
	}
	if claims.IssuedAt > now+60 {
		return uuid.Nil, errors.New("token issued in the future")
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user id: %w", err)
	}
	if id == uuid.Nil {
		return uuid.Nil, errors.New("invalid user id")
	}

	return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", ErrNoAuthHeaderIncluded
	}
	splitAuth := strings.Fields(authHeader)
	if len(splitAuth) != 2 || splitAuth[0] != "Bearer" {
		return "", errors.New("malformed authorization header")
	}

	return splitAuth[1], nil
}

func MakeRefreshToken() (string, error) {
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func GetAPIKey(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", ErrNoAuthHeaderIncluded
	}
	splitAuth := strings.Fields(authHeader)
	if len(splitAuth) != 2 || splitAuth[0] != "ApiKey" {
		return "", errors.New("malformed authorization header")
	}

	return splitAuth[1], nil
}

func encodeJWTPart(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeJWTPart(part string, dst any) error {
	data, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func signJWT(input string, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	return mac.Sum(nil)
}
