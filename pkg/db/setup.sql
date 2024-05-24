CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    command TEXT NOT NULL,
    schedule_at TIMESTAMP NOT NULL,
    picked_at TIMESTAMP,
    started_at TIMESTAMP, -- when the worker started executing the task
    completed_at TIMESTAMP, -- when the task was completed (success case)
    failed_at TIMESTAMP, -- when the task faild (failed case)
);

CREATE INDEX idx_task_schedule_at ON tasks (schedule_at);