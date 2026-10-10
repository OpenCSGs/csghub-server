SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata
    ADD COLUMN IF NOT EXISTS has_mtp_weights BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE metadata
    ADD COLUMN IF NOT EXISTS num_nextn_predict_layers INTEGER NOT NULL DEFAULT 0;
