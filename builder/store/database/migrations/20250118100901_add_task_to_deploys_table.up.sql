SET statement_timeout = 0;

--bun:split

INSERT INTO tags ("name", category, "group", "scope", built_in, show_name) VALUES('sglang', 'runtime_framework', 'inference', 'model', true, 'SGLang') ON CONFLICT ("name", category, scope) DO NOTHING;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS task VARCHAR(255);

