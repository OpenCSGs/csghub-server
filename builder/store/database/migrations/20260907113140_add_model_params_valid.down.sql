SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata
    DROP COLUMN model_params_valid;
