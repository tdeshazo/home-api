package api

import (
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var handlePattern = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

const (
	accessCookieName  = "home_api_access"
	refreshCookieName = "home_api_refresh"
)

type registerRequest struct {
	Email       string `json:"email"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type devLoginRequest struct {
	UserID string `json:"user_id"`
}

type authSessionResponse struct {
	User             userResponse `json:"user"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshExpiresAt *time.Time   `json:"refresh_expires_at,omitempty"`
	TokenType        string       `json:"token_type"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input registerRequest
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	input = normalizeRegisterRequest(input)
	if err := validateRegisterRequest(input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.authEndpoints.Register(r.Context(), input)
	if err != nil {
		handleAuthEndpointError(w, r, err)
		return
	}

	s.setAuthCookies(w, response)
	writeJSON(w, http.StatusCreated, newAuthSessionResponse(response))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	input.Email = normalizeEmail(input.Email)
	if err := validateLoginRequest(input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.authEndpoints.Login(r.Context(), input)
	if err != nil {
		handleAuthEndpointError(w, r, err)
		return
	}

	s.setAuthCookies(w, response)
	writeJSON(w, http.StatusOK, newAuthSessionResponse(response))
}

func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	var input devLoginRequest
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(input.UserID))
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "user_id must be a valid UUID")
		return
	}

	user, err := s.auth.Authenticate(r.Context(), "dev:"+userID.String())
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusUnauthorized, "invalid dev user")
		return
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	http.SetCookie(w, s.authCookie(accessCookieName, "dev:"+userID.String(), expiresAt))
	clearCookie(w, s.authCookie(refreshCookieName, "", time.Time{}))
	writeJSON(w, http.StatusOK, authSessionResponse{
		User:            publicUser(user),
		AccessExpiresAt: expiresAt,
		TokenType:       "Bearer",
	})
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var input refreshRequest
	if err := decodeOptionalJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		if cookie, err := r.Cookie(refreshCookieName); err == nil {
			input.RefreshToken = cookie.Value
		}
	}

	response, err := s.authEndpoints.Refresh(r.Context(), input)
	if err != nil {
		handleAuthEndpointError(w, r, err)
		return
	}

	s.setAuthCookies(w, response)
	writeJSON(w, http.StatusOK, newAuthSessionResponse(response))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var input logoutRequest
	if err := decodeOptionalJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		if cookie, err := r.Cookie(refreshCookieName); err == nil {
			input.RefreshToken = cookie.Value
		}
	}

	if strings.TrimSpace(input.RefreshToken) != "" {
		if err := s.authEndpoints.Logout(r.Context(), input); err != nil {
			handleAuthEndpointError(w, r, err)
			return
		}
	}

	s.clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func newAuthSessionResponse(response authTokenResponse) authSessionResponse {
	refreshExpiresAt := response.RefreshExpiresAt
	return authSessionResponse{
		User:             response.User,
		AccessExpiresAt:  response.AccessExpiresAt,
		RefreshExpiresAt: &refreshExpiresAt,
		TokenType:        response.TokenType,
	}
}

func (s *Server) setAuthCookies(w http.ResponseWriter, response authTokenResponse) {
	http.SetCookie(w, s.authCookie(accessCookieName, response.AccessToken, response.AccessExpiresAt))
	http.SetCookie(w, s.authCookie(refreshCookieName, response.RefreshToken, response.RefreshExpiresAt))
}

func (s *Server) clearAuthCookies(w http.ResponseWriter) {
	clearCookie(w, s.authCookie(accessCookieName, "", time.Time{}))
	clearCookie(w, s.authCookie(refreshCookieName, "", time.Time{}))
}

func (s *Server) authCookie(name, value string, expires time.Time) *http.Cookie {
	maxAge := 0
	if !expires.IsZero() {
		maxAge = max(0, int(time.Until(expires).Seconds()))
	}
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   strings.EqualFold(s.authConfig.Environment, "production"),
		SameSite: http.SameSiteLaxMode,
	}
}

func clearCookie(w http.ResponseWriter, cookie *http.Cookie) {
	cookie.Value = ""
	cookie.Expires = time.Unix(0, 0).UTC()
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
}

func normalizeRegisterRequest(input registerRequest) registerRequest {
	input.Email = normalizeEmail(input.Email)
	input.Handle = strings.TrimSpace(input.Handle)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	return input
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateRegisterRequest(input registerRequest) error {
	if !isValidEmail(input.Email) {
		return errors.New("email must be valid")
	}
	if !handlePattern.MatchString(input.Handle) {
		return errors.New("handle must be 3-32 lowercase letters, numbers, underscores, or hyphens")
	}
	if input.DisplayName == "" {
		return errors.New("display_name is required")
	}
	if len(input.Password) < 8 || len(input.Password) > 256 {
		return errors.New("password must be between 8 and 256 bytes")
	}
	return nil
}

func validateLoginRequest(input loginRequest) error {
	if !isValidEmail(input.Email) {
		return errors.New("email must be valid")
	}
	if input.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func handleAuthEndpointError(w http.ResponseWriter, r *http.Request, err error) {
	SetLogError(r.Context(), err)

	switch {
	case errors.Is(err, errAuthEndpointUnavailable):
		writeError(w, http.StatusServiceUnavailable, "authentication is not configured")
	case errors.Is(err, errDuplicateUser):
		writeError(w, http.StatusConflict, "email or handle already exists")
	case errors.Is(err, errInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, errInvalidRefreshToken):
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
	default:
		writeError(w, http.StatusInternalServerError, "authentication failed")
	}
}
