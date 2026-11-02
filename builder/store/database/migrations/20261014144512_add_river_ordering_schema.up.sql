-- River main 008 is a PostgreSQL no-op. Apply fork sequence and lease schema.

-- Add durable sequence eligibility without changing core job state transitions.
CREATE TABLE river_sequence_group (
    key text PRIMARY KEY,
    next_position bigint NOT NULL DEFAULT 1,
    mode text NOT NULL DEFAULT 'ordered',
    blocked_job_id bigint,
    blocked_position bigint,
    blocked_reason text,
    blocked_at timestamptz,
    next_check_at timestamptz,
    last_progress_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (char_length(key) > 0 AND char_length(key) < 512),
    CHECK (next_position > 0),
    CHECK (mode IN ('ordered', 'blocked', 'bypass')),
    CHECK (
        (mode = 'blocked'
            AND blocked_job_id IS NOT NULL
            AND blocked_position IS NOT NULL
            AND blocked_position > 0
            AND blocked_reason IS NOT NULL
            AND blocked_reason IN ('cancelled', 'discarded')
            AND blocked_at IS NOT NULL)
        OR
        (mode <> 'blocked'
            AND blocked_job_id IS NULL
            AND blocked_position IS NULL
            AND blocked_reason IS NULL
            AND blocked_at IS NULL)
    )
);

CREATE INDEX river_sequence_group_next_check_idx
    ON river_sequence_group(next_check_at, key)
    WHERE next_check_at IS NOT NULL;

-- Preserve fair keyset scanning without visiting idle groups in generic plans.
CREATE INDEX river_sequence_group_due_key_idx
    ON river_sequence_group(key)
    WHERE next_check_at IS NOT NULL;

ALTER TABLE river_job
    ADD COLUMN sequence_key text,
    ADD COLUMN sequence_position bigint,
    ADD COLUMN sequence_continue_on_cancelled boolean,
    ADD COLUMN sequence_continue_on_discarded boolean,
    ADD COLUMN sequence_resolved_at timestamptz;

ALTER TABLE river_job
    ADD CONSTRAINT river_job_sequence_shape CHECK (
        (sequence_key IS NULL
            AND sequence_position IS NULL
            AND sequence_continue_on_cancelled IS NULL
            AND sequence_continue_on_discarded IS NULL
            AND sequence_resolved_at IS NULL)
        OR
        (sequence_key IS NOT NULL
            AND char_length(sequence_key) > 0
            AND char_length(sequence_key) < 512
            AND sequence_position IS NOT NULL
            AND sequence_position > 0
            AND sequence_continue_on_cancelled IS NOT NULL
            AND sequence_continue_on_discarded IS NOT NULL)
    );

CREATE UNIQUE INDEX river_job_sequence_position_idx
    ON river_job(sequence_key, sequence_position)
    WHERE sequence_key IS NOT NULL;

CREATE INDEX river_job_sequence_unresolved_idx
    ON river_job(sequence_key, sequence_position)
    WHERE sequence_key IS NOT NULL AND sequence_resolved_at IS NULL;

CREATE INDEX river_job_sequence_active_idx
    ON river_job(sequence_key, sequence_position)
    WHERE sequence_key IS NOT NULL
        AND (
            state IN ('available', 'running', 'retryable', 'scheduled')
            OR (state = 'pending' AND sequence_resolved_at IS NOT NULL)
        );

--bun:split

-- Store a lease only while a job is running.
ALTER TABLE river_job
    ADD COLUMN lease_expires_at timestamptz,
    ADD CONSTRAINT river_job_lease_state_valid CHECK (lease_expires_at IS NULL OR state = 'running') NOT VALID;

-- Support rescuer scans independently of business execution duration.
CREATE INDEX river_job_lease_expiry_idx
    ON river_job (lease_expires_at, id)
    WHERE state = 'running' AND lease_expires_at IS NOT NULL;

--bun:split

INSERT INTO river_migration (line, version) VALUES ('main', 8), ('main', 9), ('main', 10);
