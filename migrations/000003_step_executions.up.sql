CREATE TABLE IF NOT EXISTS saga_step_executions (
    saga_id TEXT NOT NULL REFERENCES saga_instances(id) ON DELETE CASCADE,
    step_name TEXT NOT NULL,
    branch_name TEXT NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (saga_id, step_name)
);

CREATE INDEX IF NOT EXISTS idx_saga_step_executions_saga_id
    ON saga_step_executions (saga_id);
