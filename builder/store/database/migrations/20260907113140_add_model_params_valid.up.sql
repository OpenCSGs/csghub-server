SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata
    ADD COLUMN model_params_valid boolean NOT NULL DEFAULT false;

--bun:split

UPDATE metadata
SET model_params_valid = true
WHERE model_params > 0;
