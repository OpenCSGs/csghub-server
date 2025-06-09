SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata DROP COLUMN IF EXISTS mini_gpu_finetune_gb;
