package api

import (
	"errors"
	"net/http"
	"strings"

	"social-api/internal/db"
)

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" || len(input.Body) > 280 {
		SetLogError(r.Context(), errors.New("post body length out of range"))
		writeError(w, http.StatusBadRequest, "body must be between 1 and 280 characters")
		return
	}

	post, err := s.queries.CreatePost(r.Context(), db.CreatePostParams{
		UserID: user.ID,
		Body:   input.Body,
	})
	if err != nil {
		SetLogError(r.Context(), err)
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
		SetLogError(r.Context(), err)
		handleDBError(w, err, "post not found")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 50)
	posts, err := s.queries.ListPosts(r.Context(), limit)
	if err != nil {
		SetLogError(r.Context(), err)
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
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list posts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) listMyPosts(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	limit := parseLimit(r, 50)
	posts, err := s.queries.ListPostsByUser(r.Context(), db.ListPostsByUserParams{
		UserID: user.ID,
		Limit:  limit,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list posts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) updatePost(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
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
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" || len(input.Body) > 280 {
		SetLogError(r.Context(), errors.New("post body length out of range"))
		writeError(w, http.StatusBadRequest, "body must be between 1 and 280 characters")
		return
	}

	post, err := s.queries.UpdatePostForUser(r.Context(), db.UpdatePostForUserParams{
		ID:     postID,
		UserID: user.ID,
		Body:   input.Body,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		handleDBError(w, err, "post not found or not owned by user")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
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
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not delete post")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
