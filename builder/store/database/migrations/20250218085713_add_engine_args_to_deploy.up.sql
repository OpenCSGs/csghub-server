SET statement_timeout = 0;

--bun:split

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name) VALUES('llama.cpp', 'runtime_framework', 'inference', 'model', true, 'Llama.cpp') ON CONFLICT ("name", category, scope) DO NOTHING;

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS engine_args VARCHAR;
