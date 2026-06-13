package api

import (
	"errors"
	"net/http"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	writeJSON(w, http.StatusOK, publicUser(user))
}
