-- Stop all River consumers before removing physical claim fencing.
-- Finish cooperative pauses before rollback so resumable ordering is not lost.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM river_job WHERE state = 'running')
        OR EXISTS (SELECT 1 FROM river_queue WHERE paused_at IS NOT NULL AND pause_running) THEN
        RAISE EXCEPTION 'cannot roll back River queue pause schema while running jobs or cooperative pauses remain';
    END IF;
END $$;

ALTER TABLE river_job DROP COLUMN claim_version;
ALTER TABLE river_queue
    DROP COLUMN control_version,
    DROP COLUMN pause_running;

DELETE FROM river_migration WHERE line = 'main' AND version = 11;
