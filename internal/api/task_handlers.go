package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tdeshazo/home-api/internal/db"

	"github.com/google/uuid"
)

var allowedTaskFrequencies = map[string]bool{
	"daily":  true,
	"weekly": true,
}

type taskInput struct {
	Title         string   `json:"title"`
	FrequencyKind string   `json:"frequency_kind"`
	DaysOfWeek    []int64  `json:"days_of_week"`
	PointValue    int32    `json:"point_value"`
	Individual    bool     `json:"individual"`
	IsActive      bool     `json:"is_active"`
	AssigneeIDs   []string `json:"assignee_ids"`
}

type taskCompletionResponse struct {
	Completion    db.CompleteTaskRow `json:"completion"`
	PointsAwarded int32              `json:"points_awarded"`
	User          userResponse       `json:"user"`
}

type taskDashboardAssignee struct {
	User           userResponse           `json:"user"`
	Tasks          []db.Task              `json:"tasks"`
	CompletedTasks []db.CompletedTasksRow `json:"completed_tasks"`
}

type taskDashboardResponse struct {
	Date      string                  `json:"date"`
	Assignees []taskDashboardAssignee `json:"assignees"`
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.queries.ListTasks(r.Context())
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list tasks")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	input, ok := parseTaskInput(w, r)
	if !ok {
		return
	}
	assigneeIDs, ok := parseAssigneeIDs(w, r, input.AssigneeIDs)
	if !ok {
		return
	}
	if s.txBeginner == nil {
		writeError(w, http.StatusInternalServerError, "task transactions are not configured")
		return
	}

	tx, err := s.txBeginner.BeginTx(r.Context(), nil)
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not begin task transaction")
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := s.queries.WithTx(tx)
	task, err := queries.CreateTask(r.Context(), db.CreateTaskParams{
		Title:         input.Title,
		FrequencyKind: input.FrequencyKind,
		DaysOfWeek:    input.DaysOfWeek,
		PointValue:    input.PointValue,
		Individual:    input.Individual,
		IsActive:      input.IsActive,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not create task")
		return
	}
	if err := assignTaskUsers(r.Context(), queries, task.ID, assigneeIDs); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not assign task")
		return
	}
	if err := tx.Commit(); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not commit task")
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	taskID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	input, ok := parseTaskInput(w, r)
	if !ok {
		return
	}
	assigneeIDs, ok := parseAssigneeIDs(w, r, input.AssigneeIDs)
	if !ok {
		return
	}
	if s.txBeginner == nil {
		writeError(w, http.StatusInternalServerError, "task transactions are not configured")
		return
	}

	tx, err := s.txBeginner.BeginTx(r.Context(), nil)
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not begin task transaction")
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := s.queries.WithTx(tx)
	task, err := queries.UpdateTask(r.Context(), db.UpdateTaskParams{
		ID:            taskID,
		Title:         input.Title,
		FrequencyKind: input.FrequencyKind,
		DaysOfWeek:    input.DaysOfWeek,
		PointValue:    input.PointValue,
		Individual:    input.Individual,
		IsActive:      input.IsActive,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		handleDBError(w, err, "task not found")
		return
	}
	if err := queries.DeleteTaskAssignments(r.Context(), task.ID); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not update assignments")
		return
	}
	if err := assignTaskUsers(r.Context(), queries, task.ID, assigneeIDs); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not assign task")
		return
	}
	if err := tx.Commit(); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not commit task")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	taskID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	rows, err := s.queries.DeleteTask(r.Context(), taskID)
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not deactivate task")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) completeTask(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	taskID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	s.completeTaskForUser(w, r, taskID, user.ID)
}

func (s *Server) completeDashboardTask(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	userID, ok := parsePathUUID(w, r, "userID")
	if !ok {
		return
	}
	taskID, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}

	s.completeTaskForUser(w, r, taskID, userID)
}

func (s *Server) completeTaskForUser(w http.ResponseWriter, r *http.Request, taskID uuid.UUID, userID uuid.UUID) {
	if s.txBeginner == nil {
		writeError(w, http.StatusInternalServerError, "task transactions are not configured")
		return
	}

	tx, err := s.txBeginner.BeginTx(r.Context(), nil)
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not begin task transaction")
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := s.queries.WithTx(tx)
	completion, err := queries.CompleteTask(r.Context(), db.CompleteTaskParams{
		TaskID:      taskID,
		UserID:      userID,
		CompletedOn: time.Now().In(time.Local),
	})
	if err != nil {
		SetLogError(r.Context(), err)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "task is not available to complete")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not complete task")
		return
	}

	updatedUser, err := queries.AddUserPoints(r.Context(), db.AddUserPointsParams{
		ID:     userID,
		Points: completion.PointValue,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not award task points")
		return
	}

	if err := tx.Commit(); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not commit task completion")
		return
	}

	writeJSON(w, http.StatusOK, taskCompletionResponse{
		Completion:    completion,
		PointsAwarded: completion.PointValue,
		User:          publicUser(updatedUser),
	})
}

func (s *Server) taskDashboardData(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	today := time.Now().In(time.Local)
	users, err := s.queries.ListAvailableTaskAssignees(r.Context(), today)
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list dashboard assignees")
		return
	}

	assignees := make([]taskDashboardAssignee, 0, len(users))
	for _, user := range users {
		tasks, err := s.queries.AvailableTasks(r.Context(), db.AvailableTasksParams{
			UserID:  user.ID,
			Column2: today,
		})
		if err != nil {
			SetLogError(r.Context(), err)
			writeError(w, http.StatusInternalServerError, "could not list dashboard tasks")
			return
		}
		completedTasks, err := s.queries.CompletedTasks(r.Context(), db.CompletedTasksParams{
			UserID:  user.ID,
			Column2: today,
		})
		if err != nil {
			SetLogError(r.Context(), err)
			writeError(w, http.StatusInternalServerError, "could not list completed dashboard tasks")
			return
		}
		assignees = append(assignees, taskDashboardAssignee{
			User:           publicUser(user),
			Tasks:          tasks,
			CompletedTasks: completedTasks,
		})
	}

	writeJSON(w, http.StatusOK, taskDashboardResponse{
		Date:      today.Format("2006-01-02"),
		Assignees: assignees,
	})
}

func (s *Server) listMyDailyTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	today := time.Now().In(time.Local)
	tasks, err := s.queries.AvailableTasks(r.Context(), db.AvailableTasksParams{
		UserID:  user.ID,
		Column2: today,
	})
	if err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "could not list daily tasks")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func parseTaskInput(w http.ResponseWriter, r *http.Request) (taskInput, bool) {
	var input taskInput
	if err := decodeJSON(r, &input); err != nil {
		SetLogError(r.Context(), err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return taskInput{}, false
	}

	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" || len(input.Title) > 200 {
		SetLogError(r.Context(), errors.New("task title length out of range"))
		writeError(w, http.StatusBadRequest, "title must be between 1 and 200 characters")
		return taskInput{}, false
	}
	input.FrequencyKind = strings.TrimSpace(input.FrequencyKind)
	if input.FrequencyKind == "" {
		input.FrequencyKind = "daily"
	}
	if !allowedTaskFrequencies[input.FrequencyKind] {
		writeError(w, http.StatusBadRequest, "frequency_kind must be daily or weekly")
		return taskInput{}, false
	}
	if input.FrequencyKind == "daily" {
		input.DaysOfWeek = []int64{}
	}
	if input.FrequencyKind == "weekly" {
		if len(input.DaysOfWeek) == 0 {
			writeError(w, http.StatusBadRequest, "weekly tasks require days_of_week")
			return taskInput{}, false
		}
		for _, day := range input.DaysOfWeek {
			if day < 1 || day > 7 {
				writeError(w, http.StatusBadRequest, "days_of_week values must be between 1 and 7")
				return taskInput{}, false
			}
		}
	}
	if input.PointValue < 0 {
		writeError(w, http.StatusBadRequest, "point_value must be zero or greater")
		return taskInput{}, false
	}

	return input, true
}

func parseAssigneeIDs(w http.ResponseWriter, r *http.Request, rawIDs []string) ([]uuid.UUID, bool) {
	ids := make([]uuid.UUID, 0, len(rawIDs))
	seen := map[uuid.UUID]bool{}
	for _, rawID := range rawIDs {
		id, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid assignee id %q", rawID))
			return nil, false
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, true
}

func assignTaskUsers(ctx context.Context, queries *db.Queries, taskID uuid.UUID, userIDs []uuid.UUID) error {
	for _, userID := range userIDs {
		if err := queries.AssignTask(ctx, db.AssignTaskParams{
			TaskID: taskID,
			UserID: userID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, ok := userFromContext(r.Context())
	if !ok {
		SetLogError(r.Context(), errors.New("missing user context"))
		writeError(w, http.StatusUnauthorized, "missing user context")
		return false
	}
	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, "admin access required")
		return false
	}
	return true
}
