package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/tdeshazo/home-api/internal/db"
)

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	body, ok := parsePostBody(w, r)
	if !ok {
		return
	}

	post, err := s.queries.CreatePost(r.Context(), db.CreatePostParams{
		UserID: user.ID,
		Body:   body,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not create post")
		return
	}

	writeJSON(w, http.StatusCreated, post)
}

func (s *Server) createReply(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	parentPostID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	body, ok := parsePostBody(w, r)
	if !ok {
		return
	}

	reply, err := s.queries.CreateReply(r.Context(), db.CreateReplyParams{
		ID:     parentPostID,
		UserID: user.ID,
		Body:   body,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		handleDBError(w, err, "post not found")
		return
	}

	writeJSON(w, http.StatusCreated, reply)
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
	offset := parseOffset(r)
	posts, err := s.queries.ListPosts(r.Context(), db.ListPostsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list posts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) listPostReplies(w http.ResponseWriter, r *http.Request) {
	parentPostID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	limit := parseLimit(r, 50)
	offset := parseOffset(r)
	replies, err := s.queries.ListPostReplies(r.Context(), db.ListPostRepliesParams{
		ParentPostID: parentPostID,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list replies")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"replies": replies})
}

func (s *Server) listPostsByUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := parsePathUUID(w, r, "userID")
	if !ok {
		return
	}

	limit := parseLimit(r, 50)
	offset := parseOffset(r)
	posts, err := s.queries.ListPostsByUser(r.Context(), db.ListPostsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
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
	offset := parseOffset(r)
	posts, err := s.queries.ListPostsByUser(r.Context(), db.ListPostsByUserParams{
		UserID: user.ID,
		Limit:  limit,
		Offset: offset,
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

	body, ok := parsePostBody(w, r)
	if !ok {
		return
	}

	post, err := s.queries.UpdatePostForUser(r.Context(), db.UpdatePostForUserParams{
		ID:     postID,
		UserID: user.ID,
		Body:   body,
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

func parsePostBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return "", false
	}

	body := strings.TrimSpace(input.Body)
	if body == "" || len(body) > 280 {
		SetLogError(r.Context(), errors.New("post body length out of range"))
		writeError(w, http.StatusBadRequest, "body must be between 1 and 280 characters")
		return "", false
	}

	return body, true
}
