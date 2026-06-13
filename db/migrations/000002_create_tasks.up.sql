CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL,
    frequency_type text NOT NULL DEFAULT 'once',
    days_of_week smallint[] NOT NULL DEFAULT '{}',
    point_value int NOT NULL DEFAULT 0,
    individual boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()

    CONSTRAINT task_frequency_type_check CHECK (
        frequency_type IN (
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
);

CREATE TRIGGER tasks_set_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS task_assignments (
    task_id UUID NOT NULL,
    user_id UUID NOT NULL,
    PRIMARY KEY (task_id, user_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS task_completions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    user_id UUID NOT NULL,
    completed_on date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(task_id, user_id, completed_on),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES tasks (id) ON DELETE CASCADE
);
