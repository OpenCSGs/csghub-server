SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD likes IF NOT EXISTS INT DEFAULT 0;

--bun:split

ALTER TABLE repositories ADD download_count IF NOT EXISTS INT DEFAULT 0;

--bun:split

ALTER TABLE repositories ADD nickname VARCHAR;
