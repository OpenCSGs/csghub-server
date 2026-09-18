SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories
    ADD COLUMN IF NOT EXISTS compliance_status VARCHAR(32) NOT NULL DEFAULT 'pending_review',
    ADD COLUMN IF NOT EXISTS commercial_permission VARCHAR(32) NOT NULL DEFAULT 'custom_terms';

--bun:split

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'repositories_compliance_status_chk'
          AND conrelid = 'repositories'::regclass
    ) THEN
        ALTER TABLE repositories
            ADD CONSTRAINT repositories_compliance_status_chk
            CHECK (compliance_status IN ('compliant', 'pending_review', 'non_compliant'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'repositories_commercial_permission_chk'
          AND conrelid = 'repositories'::regclass
    ) THEN
        ALTER TABLE repositories
            ADD CONSTRAINT repositories_commercial_permission_chk
            CHECK (commercial_permission IN ('commercial_allowed', 'conditional_commercial', 'non_commercial', 'copyleft', 'custom_terms'));
    END IF;
END
$$;

--bun:split

UPDATE repositories
SET compliance_status = 'compliant',
    commercial_permission = CASE
        WHEN LOWER(TRIM(license)) IN ('apache-2.0', 'mit', 'afl-3.0', 'ecl-2.0', 'cc0-1.0', 'cc-by-4.0') THEN 'commercial_allowed'
        WHEN LOWER(TRIM(license)) = 'creativeml-openrail-m' THEN 'conditional_commercial'
        WHEN LOWER(TRIM(license)) = 'cc-by-nc-4.0'
          OR LOWER(TRIM(license)) LIKE 'cc-by-nc-nd-%'
          OR LOWER(TRIM(license)) LIKE 'cc-by-nc-sa-%' THEN 'non_commercial'
        WHEN LOWER(TRIM(license)) IN ('gpl', 'agpl-3.0', 'lgpl')
          OR LOWER(TRIM(license)) LIKE 'gpl-%'
          OR LOWER(TRIM(license)) LIKE 'lgpl-%' THEN 'copyleft'
        ELSE 'custom_terms'
    END
WHERE repository_type IN ('model', 'dataset')
  AND (
      LOWER(TRIM(license)) IN (
          'apache-2.0', 'mit', 'afl-3.0', 'ecl-2.0', 'cc0-1.0', 'cc-by-4.0',
          'creativeml-openrail-m', 'cc-by-nc-4.0', 'gpl', 'agpl-3.0', 'lgpl'
      )
      OR LOWER(TRIM(license)) LIKE 'cc-by-nc-nd-%'
      OR LOWER(TRIM(license)) LIKE 'cc-by-nc-sa-%'
      OR LOWER(TRIM(license)) LIKE 'gpl-%'
      OR LOWER(TRIM(license)) LIKE 'lgpl-%'
  );

--bun:split

INSERT INTO tags (name, category, "group", scope, built_in, show_name)
VALUES
    ('other', 'license', '', 'model', true, 'Other'),
    ('other', 'license', '', 'dataset', true, 'Other')
ON CONFLICT (name, category, scope) DO UPDATE SET
    built_in = EXCLUDED.built_in,
    show_name = EXCLUDED.show_name;
