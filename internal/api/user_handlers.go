package api

import (
	"net/http"
)

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}
	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	users, err := s.queries.ListUsers(r.Context())
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list users")
		return
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, publicUser(user))
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": response})
}
