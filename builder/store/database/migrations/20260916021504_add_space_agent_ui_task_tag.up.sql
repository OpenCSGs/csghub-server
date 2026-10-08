SET statement_timeout = 0;

--bun:split

INSERT INTO tag_categories ("name", "scope", show_name, enabled)
VALUES ('task', 'space', 'Task', true)
ON CONFLICT ("name", "scope") DO NOTHING;

--bun:split

INSERT INTO tags ("name", category, "scope", built_in, show_name, i18n_key, "group")
VALUES ('agent-ui', 'task', 'space', true, 'Agent UI', 'agent-ui', '')
ON CONFLICT ("name", category, "scope") DO NOTHING;
