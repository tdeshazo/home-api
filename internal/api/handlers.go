package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"social-api/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" || len(input.Body) > 280 {
		writeError(w, http.StatusBadRequest, "body must be between 1 and 280 characters")
		return
	}

	post, err := s.queries.CreatePost(r.Context(), db.CreatePostParams{
		UserID: user.ID,
		Body:   input.Body,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create post")
		return
	}

	writeJSON(w, http.StatusCreated, post)
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	postID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	post, err := s.queries.GetPost(r.Context(), postID)
	if err != nil {
		handleDBError(w, err, "post not found")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 50)
	posts, err := s.queries.ListPosts(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list posts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) listPostsByUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := parsePathUUID(w, r, "userID")
	if !ok {
		return
	}

	limit := parseLimit(r, 50)
	posts, err := s.queries.ListPostsByUser(r.Context(), db.ListPostsByUserParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list posts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) listMyPosts(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	limit := parseLimit(r, 50)
	posts, err := s.queries.ListPostsByUser(r.Context(), db.ListPostsByUserParams{
		UserID: user.ID,
		Limit:  limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list posts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) updatePost(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	postID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" || len(input.Body) > 280 {
		writeError(w, http.StatusBadRequest, "body must be between 1 and 280 characters")
		return
	}

	post, err := s.queries.UpdatePostForUser(r.Context(), db.UpdatePostForUserParams{
		ID:     postID,
		UserID: user.ID,
		Body:   input.Body,
	})
	if err != nil {
		handleDBError(w, err, "post not found or not owned by user")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	postID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	err := s.queries.DeletePostForUser(r.Context(), db.DeletePostForUserParams{
		ID:     postID,
		UserID: user.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete post")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parsePathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	value := r.PathValue(name)
	id, err := uuid.Parse(value)
	if err != nil {
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
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, notFoundMessage)
		return
	}
	writeError(w, http.StatusInternalServerError, "database error")
}
