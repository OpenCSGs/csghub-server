SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata DROP COLUMN IF EXISTS has_mtp_weights;

ALTER TABLE metadata DROP COLUMN IF EXISTS num_nextn_predict_layers;
