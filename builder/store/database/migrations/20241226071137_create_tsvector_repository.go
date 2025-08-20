package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
)

func init() {
	Migrations.MustRegister(
		upAddSearchVector,
		downAddSearchVector,
	)
}

// Up Migration
func upAddSearchVector(ctx context.Context, db *bun.DB) error {
	// must use postgresql
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.Database.Driver != "pg" {
		return nil
	}

	searchConfiguration := cfg.Database.SearchConfiguration

	// try to create zhparser extension
	_, err = db.Exec(`CREATE EXTENSION IF NOT EXISTS zhparser;`)
	useZhparser := true
	if err != nil {
		if strings.Contains(err.Error(), `extension "zhparser" is not available`) {
			useZhparser = false
		} else {
			return fmt.Errorf("failed to create zhparser extension: %w", err)
		}
	}

	if useZhparser {
		if _, err := db.Exec(`
			CREATE TEXT SEARCH CONFIGURATION ` + searchConfiguration + ` (
				PARSER = zhparser
			);
		`); err != nil {
			return fmt.Errorf("failed to create text search configuration: %w", err)
		}

		// critical：mapping chinese lexical to simple dictionary
		if _, err := db.Exec(`
			ALTER TEXT SEARCH CONFIGURATION ` + searchConfiguration + `
			ADD MAPPING FOR 
				a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,x,y,z
			WITH simple;
		`); err != nil {
			return fmt.Errorf("failed to add mapping for zhparser: %w", err)
		}
	} else {
		// -----------------------
		// fallback pg_catalog.english
		// -----------------------
		if _, err := db.Exec(`
        CREATE TEXT SEARCH CONFIGURATION ` + searchConfiguration + `
        ( COPY = pg_catalog.english );
    `); err != nil {
			return fmt.Errorf("failed to create text search config (copy from pg_catalog.english): %w", err)
		}
	}

	// -------------------------
	// create search_vector
	// -------------------------
	if _, err := db.Exec(`
        ALTER TABLE repositories
        ADD COLUMN IF NOT EXISTS search_vector tsvector
    `); err != nil {
		return fmt.Errorf("failed to add 'search_vector' column: %w", err)
	}

	// -------------------------
	// creat GIN index
	// -------------------------
	if _, err := db.Exec(`
        CREATE INDEX IF NOT EXISTS idx_repositories_search_vector
        ON repositories
        USING GIN (search_vector)
    `); err != nil {
		return fmt.Errorf("failed to create GIN index on 'search_vector': %w", err)
	}

	// -------------------------
	// create trigger
	// -------------------------
	if _, err := db.Exec(`
        CREATE OR REPLACE FUNCTION update_repo_search_vector() RETURNS trigger AS $$
        BEGIN
            NEW.search_vector := 
            	  setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.path, '[-/]', ' ', 'g'), '')), 'A')
            	|| setweight(to_tsvector('` + searchConfiguration + `',COALESCE(regexp_replace(NEW.nickname, '[-/]', ' ', 'g'), '')), 'B')
            	|| setweight(to_tsvector('` + searchConfiguration + `', COALESCE(NEW.description, '')), 'C');
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
    `); err != nil {
		return fmt.Errorf("failed to create custom function update_repo_search_vector: %w", err)
	}

	// -------------------------
	// create trigger
	// -------------------------
	if _, err := db.Exec(`
		CREATE TRIGGER repositories_search_vector_update
		BEFORE INSERT OR UPDATE OF path, nickname, description
		ON repositories
		FOR EACH ROW
		EXECUTE FUNCTION update_repo_search_vector()
	`); err != nil {
		return fmt.Errorf("failed to create new trigger for 'search_vector': %w", err)
	}

	// -------------------------
	// init search_vector
	// -------------------------
	if _, err := db.Exec(`
		UPDATE repositories
		SET path = path
	`); err != nil {
		return fmt.Errorf("failed to init existing data for 'search_vector': %w", err)
	}

	return nil
}

// Down Migration
func downAddSearchVector(ctx context.Context, db *bun.DB) error {
	// must use postgresql
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.Database.Driver != "pg" {
		return nil
	}

	if _, err := db.Exec(`
        DROP TRIGGER IF EXISTS repositories_search_vector_update
        ON repositories
    `); err != nil {
		return fmt.Errorf("failed to drop trigger: %w", err)
	}

	if _, err := db.Exec(`
        DROP FUNCTION IF EXISTS update_repo_search_vector()
    `); err != nil {
		return fmt.Errorf("failed to drop function update_repo_search_vector: %w", err)
	}

	if _, err := db.Exec(`
        DROP INDEX IF EXISTS idx_repositories_search_vector
    `); err != nil {
		return fmt.Errorf("failed to drop 'idx_repositories_search_vector' index: %w", err)
	}

	if _, err := db.Exec(`
        ALTER TABLE repositories
        DROP COLUMN IF EXISTS search_vector
    `); err != nil {
		return fmt.Errorf("failed to drop 'search_vector' column: %w", err)
	}

	return nil
}
