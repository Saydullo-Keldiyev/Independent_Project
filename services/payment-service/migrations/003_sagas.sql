-- Migration 003: Create sagas and saga_steps tables for saga orchestration.
-- Supports generic saga state persistence with step-level tracking.

-- UP
CREATE TABLE IF NOT EXISTS sagas (
    id              UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_type       VARCHAR(100)   NOT NULL,
    reference_id    UUID           NOT NULL,
    state           VARCHAR(20)    NOT NULL CHECK (state IN ('running', 'completed', 'compensating', 'failed')),
    current_step    INT            NOT NULL DEFAULT 0,
    data            JSONB          NOT NULL,
    started_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    error_message   TEXT
);

CREATE INDEX IF NOT EXISTS idx_sagas_reference ON sagas(reference_id);
CREATE INDEX IF NOT EXISTS idx_sagas_state ON sagas(state) WHERE state IN ('running', 'compensating');

CREATE TABLE IF NOT EXISTS saga_steps (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id     UUID           NOT NULL REFERENCES sagas(id),
    step_index  INT            NOT NULL,
    step_name   VARCHAR(100)   NOT NULL,
    status      VARCHAR(20)    NOT NULL CHECK (status IN ('pending', 'completed', 'failed', 'compensated')),
    executed_at TIMESTAMPTZ,
    error       TEXT
);

CREATE INDEX IF NOT EXISTS idx_saga_steps_saga ON saga_steps(saga_id, step_index);

-- DOWN
-- DROP INDEX IF EXISTS idx_saga_steps_saga;
-- DROP TABLE IF EXISTS saga_steps;
-- DROP INDEX IF EXISTS idx_sagas_state;
-- DROP INDEX IF EXISTS idx_sagas_reference;
-- DROP TABLE IF EXISTS sagas;
