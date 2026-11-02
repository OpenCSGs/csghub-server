SET statement_timeout = 0;

--bun:split

-- User-scope tag category: user identity tags (upstream channel routing
-- tiers and future user-level labels) reuse the existing tags machinery.
INSERT INTO tag_categories (name, show_name, scope, enabled, auto_detected)
VALUES ('user', '用户标签', 'user', true, false)
ON CONFLICT (name, scope) DO NOTHING;

--bun:split

-- Built-in low-priority channel tag. Operators assign it (and dynamic
-- priority:<n> tags they create) through user_tags.
INSERT INTO tags (name, category, "group", scope, built_in, show_name, created_at, updated_at)
VALUES ('low_priority', 'user', 'user', 'user', true, '低优先级', now(), now())
ON CONFLICT (name, category, scope) DO UPDATE
    SET built_in = EXCLUDED.built_in,
        show_name = EXCLUDED.show_name;

--bun:split

-- Reverse lookup index for "list users by tag" queries (e.g. all users on
-- the low-priority channel). The existing idx_user_id_link_tag_id serves the
-- user→tags direction only; joining user_tags by tag_id would otherwise scan
-- the whole table.
CREATE INDEX IF NOT EXISTS idx_tag_id_link_user_id ON user_tags(tag_id);
