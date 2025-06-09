SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata ADD COLUMN IF NOT EXISTS mini_gpu_finetune_gb FLOAT;