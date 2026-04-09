UPDATE repository_file_check_rules SET pattern = LOWER(pattern);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rule_type_pattern ON repository_file_check_rules (rule_type, pattern);