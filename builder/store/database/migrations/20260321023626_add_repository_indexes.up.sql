SET statement_timeout = 0;

--bun:split

CREATE INDEX idx_repo_type_deleted ON repositories(repository_type,deleted_at);

