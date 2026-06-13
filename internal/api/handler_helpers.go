package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

func parsePathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	value := r.PathValue(name)
	id, err := uuid.Parse(value)
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

func parseLimit(r *http.Request, fallback int32) int32 {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	if value > 100 {
		value = 100
	}
	return int32(value)
}

func handleDBError(w http.ResponseWriter, err error, notFoundMessage string) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, notFoundMessage)
		return
	}
	writeError(w, http.StatusInternalServerError, "database error")
}
