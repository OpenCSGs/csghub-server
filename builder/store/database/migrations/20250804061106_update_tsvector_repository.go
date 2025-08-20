package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// check whether the database is PostgreSQL
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		if cfg.Database.Driver != "pg" {
			return nil
		}

		// check whether the search_vector column exists
		var searchVectorExists bool
		if err := db.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'repositories' AND column_name = 'search_vector')
		`).Scan(&searchVectorExists); err != nil {
			return fmt.Errorf("failed to check whether the search_vector column exists: %w", err)
		}
		if !searchVectorExists {
			return nil
		}

		// update trigger function and trigger to include hf_path and ms_path
		searchConfiguration := cfg.Database.SearchConfiguration
		if _, err := db.Exec(`
			-- Create or replace the trigger function
			CREATE OR REPLACE FUNCTION update_repo_search_vector() RETURNS trigger AS $$
			BEGIN
				NEW.search_vector := 
					setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.path, '[-/]', ' ', 'g'), '')), 'A')
					|| setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.hf_path, '[-/]', ' ', 'g'), '')), 'B')
					|| setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.ms_path, '[-/]', ' ', 'g'), '')), 'B')
					|| setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.nickname, '[-/]', ' ', 'g'), '')), 'C')
					|| setweight(to_tsvector('` + searchConfiguration + `', COALESCE(NEW.description, '')), 'D');
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;

			-- Create or replace the trigger
			CREATE OR REPLACE TRIGGER repositories_search_vector_update 
			BEFORE INSERT OR UPDATE OF path, nickname, description, hf_path, ms_path
			ON repositories
			FOR EACH ROW
			EXECUTE FUNCTION update_repo_search_vector();
		`); err != nil {
			return fmt.Errorf("failed to update search vector configuration: %w", err)
		}

		// init search_vector for existing data in batches
		// to avoid db timeout issue when updating a large number of repositories [10w+]
		if err := initRepositorySearchVectorInBatches(ctx, db); err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// check whether the database is PostgreSQL
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		if cfg.Database.Driver != "pg" {
			return nil
		}

		// check whether the search_vector column exists
		var searchVectorExists bool
		if err := db.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name = 'repositories' AND column_name = 'search_vector')
		`).Scan(&searchVectorExists); err != nil {
			return fmt.Errorf("failed to check whether the search_vector column exists: %w", err)
		}
		if !searchVectorExists {
			return nil
		}

		// update trigger function and trigger to remove hf_path and ms_path
		searchConfiguration := cfg.Database.SearchConfiguration
		if _, err := db.Exec(`
			-- Create or replace the trigger function
			CREATE OR REPLACE FUNCTION update_repo_search_vector() RETURNS trigger AS $$
			BEGIN
				NEW.search_vector := 
					setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.path, '[-/]', ' ', 'g'), '')), 'A')
					|| setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.nickname, '[-/]', ' ', 'g'), '')), 'B')
					|| setweight(to_tsvector('` + searchConfiguration + `', COALESCE(NEW.description, '')), 'C');
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;

			-- Create or replace the trigger
			CREATE OR REPLACE TRIGGER repositories_search_vector_update 
			BEFORE INSERT OR UPDATE OF path, nickname, description
			ON repositories
			FOR EACH ROW
			EXECUTE FUNCTION update_repo_search_vector();
		`); err != nil {
			return fmt.Errorf("failed to update search vector configuration: %w", err)
		}

		// init search_vector for existing data in batches
		// to avoid db timeout issue when updating a large number of repositories [10w+]
		if err := initRepositorySearchVectorInBatches(ctx, db); err != nil {
			return err
		}

		return nil
	})
}

func initRepositorySearchVectorInBatches(ctx context.Context, db *bun.DB) error {
	const batchSize = 1000
	offset := 0

	for {
		var batchIDs []int64
		err := db.NewSelect().
			Table("repositories").
			Column("id").
			Order("id").
			Limit(batchSize).
			Offset(offset).
			Scan(ctx, &batchIDs)

		if err != nil {
			return fmt.Errorf("failed to get batch IDs (offset %d): %w", offset, err)
		}

		if len(batchIDs) == 0 {
			break
		}

		_, err = db.NewUpdate().
			Table("repositories").
			Set("path = path").
			Where("id IN (?)", bun.In(batchIDs)).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to update batch (offset %d): %w", offset, err)
		}

		offset += batchSize
	}

	return nil
}
