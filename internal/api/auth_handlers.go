package api

import (
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
)

var handlePattern = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

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

	writeJSON(w, http.StatusCreated, response)
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

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var input refreshRequest
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	response, err := s.authEndpoints.Refresh(r.Context(), input)
	if err != nil {
		handleAuthEndpointError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var input logoutRequest
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.authEndpoints.Logout(r.Context(), input); err != nil {
		handleAuthEndpointError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
