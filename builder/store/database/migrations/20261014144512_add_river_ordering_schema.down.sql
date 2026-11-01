-- Run only after all producers/workers stop and unresolved sequence work drains.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM river_job WHERE sequence_key IS NOT NULL AND sequence_resolved_at IS NULL) THEN
        RAISE EXCEPTION 'cannot remove sequences while unresolved jobs remain';
    END IF;
END $$;

--bun:split

-- Remove the execution lease after leased workers have been drained.
DROP INDEX river_job_lease_expiry_idx;
ALTER TABLE river_job
    DROP CONSTRAINT river_job_lease_state_valid,
    DROP COLUMN lease_expires_at;

--bun:split

DROP TABLE river_sequence_group;
ALTER TABLE river_job
    DROP CONSTRAINT river_job_sequence_shape,
    DROP COLUMN sequence_key,
    DROP COLUMN sequence_position,
    DROP COLUMN sequence_continue_on_cancelled,
    DROP COLUMN sequence_continue_on_discarded,
    DROP COLUMN sequence_resolved_at;

--bun:split

DELETE FROM river_migration WHERE line = 'main' AND version IN (8, 9, 10);
