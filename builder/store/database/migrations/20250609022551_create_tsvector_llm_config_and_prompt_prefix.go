package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] ")
		// must use postgresql
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		if cfg.Database.Driver != "pg" {
			return nil
		}
		searchConfiguration := cfg.Database.SearchConfiguration
		// -------------------------
		// create column search_vector for table llm_configs and prompt_prefixes
		// -------------------------
		if _, err := db.Exec(`
        ALTER TABLE llm_configs
        ADD COLUMN IF NOT EXISTS search_vector tsvector
		`); err != nil {
			return fmt.Errorf("failed to add 'search_vector' column: %w", err)
		}
		if _, err := db.Exec(`
		ALTER TABLE prompt_prefixes
		ADD COLUMN IF NOT EXISTS search_vector tsvector
		`); err != nil {
			return fmt.Errorf("failed to add 'search_vector' column to prompt_prefixes: %w", err)
		}

		// -------------------------
		// creat GIN index
		// -------------------------
		if _, err := db.Exec(`
        CREATE INDEX IF NOT EXISTS idx_llm_configs_search_vector
        ON llm_configs
        USING GIN (search_vector)
		`); err != nil {
			return fmt.Errorf("failed to create GIN index on 'search_vector': %w", err)
		}
		if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_prompt_prefixes_search_vector
		ON prompt_prefixes
		USING GIN (search_vector)
		`); err != nil {
			return fmt.Errorf("failed to create GIN index on 'prompt_prefixes.search_vector': %w", err)
		}

		// -------------------------
		// create function
		// -------------------------
		if _, err := db.Exec(`
        CREATE OR REPLACE FUNCTION update_llm_configs_search_vector() RETURNS trigger AS $$
        BEGIN
            NEW.search_vector :=
                setweight(
                    to_tsvector(
                        '` + searchConfiguration + `',
                        COALESCE(
                            -- First, separate letters from numbers and numbers from letters
                            regexp_replace(
                                -- Then, replace common separators like - and / with a space
                                regexp_replace(NEW.model_name, '[-/]', ' ', 'g'),
                                '(?<=[a-zA-Z])(?=[0-9])|(?<=[0-9])(?=[a-zA-Z])', ' ', 'g'
                            ),
                            ''
                        )
                    ),
                    'A'
                );
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
		`); err != nil {
			return fmt.Errorf("failed to create custom function update_llm_configs_search_vector: %w", err)
		}

		if _, err := db.Exec(`
		CREATE OR REPLACE FUNCTION update_prompt_prefixes_search_vector() RETURNS trigger AS $$
		BEGIN
			NEW.search_vector :=
				setweight(to_tsvector('` + searchConfiguration + `', COALESCE(regexp_replace(NEW.zh, '[[:punct:]\[\]]', ' ', 'g'), '')), 'A') ||
				setweight(to_tsvector('` + searchConfiguration + `', COALESCE(regexp_replace(NEW.en, '[[:punct:]\[\]]', ' ', 'g'), '')), 'B');
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		`); err != nil {
			return fmt.Errorf("failed to create custom function update_prompt_prefixes_search_vector: %w", err)
		}

		// -------------------------
		// create trigger
		// -------------------------
		if _, err := db.Exec(`
		CREATE TRIGGER llm_configs_search_vector_update
		BEFORE INSERT OR UPDATE OF model_name, api_endpoint
		ON llm_configs
		FOR EACH ROW
		EXECUTE FUNCTION update_llm_configs_search_vector()
		`); err != nil {
			return fmt.Errorf("failed to create new trigger for 'search_vector': %w", err)
		}
		if _, err := db.Exec(`
		CREATE TRIGGER prompt_prefixes_search_vector_update
		BEFORE INSERT OR UPDATE OF zh, en
		ON prompt_prefixes
		FOR EACH ROW
		EXECUTE FUNCTION update_prompt_prefixes_search_vector()
		`); err != nil {
			return fmt.Errorf("failed to create new trigger for 'prompt_prefixes.search_vector': %w", err)
		}

		// -------------------------
		// init search_vector
		// -------------------------
		if _, err := db.Exec(`
		UPDATE llm_configs
        SET search_vector =
            setweight(
                to_tsvector(
                    '` + searchConfiguration + `',
                    COALESCE(
                        regexp_replace(
                            regexp_replace(model_name, '[-/]', ' ', 'g'),
                            '(?<=[a-zA-Z])(?=[0-9])|(?<=[0-9])(?=[a-zA-Z])', ' ', 'g'
                        ),
                        ''
                    )
                ),
                'A'
            )
        WHERE search_vector IS NULL OR true; -- Added "OR true" to force update if logic changed
		`); err != nil {
			return fmt.Errorf("failed to init existing data for 'search_vector': %w", err)
		}
		if _, err := db.Exec(`
		UPDATE prompt_prefixes
		SET search_vector =
			setweight(to_tsvector('` + searchConfiguration + `', COALESCE(regexp_replace(zh, '[[:punct:]\[\]]', ' ', 'g'), '')), 'A') ||
			setweight(to_tsvector('` + searchConfiguration + `', COALESCE(regexp_replace(en, '[[:punct:]\[\]]', ' ', 'g'), '')), 'B');
		`); err != nil {
			return fmt.Errorf("failed to init existing data for 'prompt_prefixes.search_vector': %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		// must use postgresql
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		if cfg.Database.Driver != "pg" {
			return nil
		}
		// -------------------------
		// drop trigger
		// -------------------------
		if _, err := db.Exec(`
        DROP TRIGGER IF EXISTS llm_configs_search_vector_update
        ON llm_configs
		`); err != nil {
			return fmt.Errorf("failed to drop trigger: %w", err)
		}
		if _, err := db.Exec(`
		DROP TRIGGER IF EXISTS prompt_prefixes_search_vector_update
		ON prompt_prefixes
		`); err != nil {
			return fmt.Errorf("failed to drop trigger for 'prompt_prefixes.search_vector': %w", err)
		}

		// -------------------------
		// drop function
		// -------------------------
		if _, err := db.Exec(`
        DROP FUNCTION IF EXISTS update_llm_configs_search_vector()
		`); err != nil {
			return fmt.Errorf("failed to drop function update_llm_configs_search_vector: %w", err)
		}
		if _, err := db.Exec(`
		DROP FUNCTION IF EXISTS update_prompt_prefixes_search_vector()
		`); err != nil {
			return fmt.Errorf("failed to drop function update_prompt_prefixes_search_vector: %w", err)
		}

		// -------------------------
		// drop GIN index
		// -------------------------
		if _, err := db.Exec(`
        DROP INDEX IF EXISTS idx_llm_configs_search_vector
		`); err != nil {
			return fmt.Errorf("failed to drop 'idx_llm_configs_search_vector' index: %w", err)
		}
		if _, err := db.Exec(`
		DROP INDEX IF EXISTS idx_prompt_prefixes_search_vector
		`); err != nil {
			return fmt.Errorf("failed to drop 'idx_prompt_prefixes_search_vector' index: %w", err)
		}

		// -------------------------
		// drop search_vector column
		// -------------------------
		if _, err := db.Exec(`
        ALTER TABLE llm_configs
        DROP COLUMN IF EXISTS search_vector
		`); err != nil {
			return fmt.Errorf("failed to drop 'search_vector' column: %w", err)
		}
		if _, err := db.Exec(`
		ALTER TABLE prompt_prefixes
		DROP COLUMN IF EXISTS search_vector
		`); err != nil {
			return fmt.Errorf("failed to drop 'prompt_prefixes.search_vector' column: %w", err)
		}

		return nil
	})
}
