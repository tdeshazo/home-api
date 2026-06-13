-- name: CreateTask :one
INSERT INTO tasks (
    id,
    title,
    frequency_kind,
    days_of_week,
    point_value,
    individual,
    is_active
) VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    title,
    frequency_kind,
    days_of_week,
    point_value,
    individual,
    is_active,
    created_at,
    updated_at;

-- name: GetTask :one
SELECT
    id,
    title,
    frequency_kind,
    days_of_week,
    point_value,
    individual,
    is_active,
    created_at,
    updated_at
FROM tasks
WHERE id = $1;

-- name: ListTasks :many
SELECT
    id,
    title,
    frequency_kind,
    days_of_week,
    point_value,
    individual,
    is_active,
    created_at,
    updated_at
FROM tasks
ORDER BY created_at ASC, id ASC;

-- name: UpdateTask :one
UPDATE tasks
SET
    title = $2,
    frequency_kind = $3,
    days_of_week = $4,
    point_value = $5,
    individual = $6,
    is_active = $7
WHERE id = $1
RETURNING
    id,
    title,
    frequency_kind,
    days_of_week,
    point_value,
    individual,
    is_active,
    created_at,
    updated_at;

-- name: DeleteTask :execrows
UPDATE tasks
SET is_active = false
WHERE id = $1;

-- name: AssignTask :exec
INSERT INTO task_assignments (task_id, user_id)
VALUES ($1, $2)
ON CONFLICT (task_id, user_id) DO NOTHING;

-- name: DeleteTaskAssignments :exec
DELETE FROM task_assignments
WHERE task_id = $1;

-- name: CompleteTask :one
WITH task_to_complete AS (
    SELECT t.id, t.point_value
    FROM tasks t
    JOIN task_assignments ta ON ta.task_id = t.id
    WHERE t.id = $1
      AND ta.user_id = $2
      AND t.is_active = true
      AND (
          t.frequency_kind = 'daily'
          OR (
              t.frequency_kind = 'weekly'
              AND EXTRACT(ISODOW FROM $3::date)::smallint = ANY(t.days_of_week)
          )
      )
),
inserted AS (
    INSERT INTO task_completions (task_id, user_id, completed_on)
    SELECT id, $2, $3::date
    FROM task_to_complete
    ON CONFLICT (task_id, user_id, completed_on) DO NOTHING
    RETURNING id, task_id, user_id, completed_on, created_at
)
SELECT
    inserted.id,
    inserted.task_id,
    inserted.user_id,
    inserted.completed_on,
    inserted.created_at,
    task_to_complete.point_value
FROM inserted
JOIN task_to_complete ON task_to_complete.id = inserted.task_id;

-- name: ListAvailableTaskAssignees :many
SELECT DISTINCT
    u.id,
    u.email,
    u.handle,
    u.display_name,
    u.password_hash,
    u.points,
    u.is_admin,
    u.created_at,
    u.updated_at
FROM users u
JOIN task_assignments ta ON ta.user_id = u.id
JOIN tasks t ON t.id = ta.task_id
WHERE (
      t.is_active = true
      AND (
          t.frequency_kind = 'daily'
          OR (
              t.frequency_kind = 'weekly'
              AND EXTRACT(ISODOW FROM $1::date)::smallint = ANY(t.days_of_week)
          )
      )
  )
  OR EXISTS (
      SELECT 1
      FROM task_completions tc
      WHERE tc.task_id = t.id
        AND tc.user_id = ta.user_id
        AND tc.completed_on = $1::date
  )
ORDER BY u.display_name ASC, u.handle ASC;

-- name: AvailableTasks :many
SELECT t.*
FROM tasks t
JOIN task_assignments ta ON ta.task_id = t.id
WHERE ta.user_id = $1
  AND t.is_active = true
  AND (
      t.frequency_kind = 'daily'
      OR (
          t.frequency_kind = 'weekly'
          AND EXTRACT(ISODOW FROM $2::date)::smallint = ANY(t.days_of_week)
      )
  )
  AND NOT EXISTS (
      SELECT 1
      FROM task_completions tc
      WHERE tc.task_id = t.id
        AND tc.user_id = ta.user_id
        AND tc.completed_on = $2::date
  );

-- name: CompletedTasks :many
SELECT
    t.id,
    t.title,
    t.frequency_kind,
    t.days_of_week,
    t.point_value,
    t.individual,
    t.is_active,
    t.created_at,
    t.updated_at,
    tc.created_at AS completed_at
FROM tasks t
JOIN task_completions tc ON tc.task_id = t.id
WHERE tc.user_id = $1
  AND tc.completed_on = $2::date
ORDER BY tc.created_at ASC, t.title ASC, t.id ASC;
