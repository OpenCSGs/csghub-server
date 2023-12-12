------------------------- TagCategories --------------------------
CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_categories_name_scope ON tag_categories(name, scope);
