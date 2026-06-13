-- name: CreateTask :one
INSERT INTO tasks (
    id,
    title
) VALUES (
    gen_random_uuid(),
    $1
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
    title = COALESCE(sqlc.narg('title'), title)
WHERE id = sqlc.arg('id')
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
DELETE FROM tasks
WHERE id = $1;

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
