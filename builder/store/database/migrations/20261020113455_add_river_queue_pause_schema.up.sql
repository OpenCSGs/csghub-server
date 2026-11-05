-- Apply River main migration 011 before starting v0.49.0-csghub.2 consumers.
-- Queue control versions and physical claim versions fence pause/resume races.
ALTER TABLE river_queue
    ADD COLUMN pause_running boolean NOT NULL DEFAULT false,
    ADD COLUMN control_version bigint NOT NULL DEFAULT 0;

ALTER TABLE river_job
    ADD COLUMN claim_version bigint NOT NULL DEFAULT 0;

INSERT INTO river_migration (line, version) VALUES ('main', 11);
