CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL,
    frequency_kind text NOT NULL DEFAULT 'once',
    days_of_week smallint[] NOT NULL DEFAULT '{}',
    point_value int NOT NULL DEFAULT 0,
    individual boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT tasks_frequency_kind_check CHECK (
        frequency_kind IN (
            'once',
            'daily',
            'weekly',
            'monthly',
            'yearly',
            'adaptive',
            'interval',
            'days_of_the_week',
            'day_of_the_month',
            'trigger',
            'no_repeat'
        )
    )
);

DROP TRIGGER IF EXISTS tasks_set_updated_at ON tasks;

CREATE TRIGGER tasks_set_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS task_assignments (
    task_id UUID NOT NULL,
    user_id UUID NOT NULL,
    CONSTRAINT task_assignments_pkey PRIMARY KEY (task_id, user_id),
    CONSTRAINT task_assignments_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT task_assignments_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS task_completions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    user_id UUID NOT NULL,
    completed_on date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT task_completions_task_user_completed_on_key UNIQUE(task_id, user_id, completed_on),
    CONSTRAINT task_completions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT task_completions_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);
