package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uptrace/bun"
	"gopkg.in/yaml.v2"
	"opencsg.com/csghub-server/builder/store/database"
)

type Tags struct {
	Tags []database.Tag
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, db bun.Tx) (err error) {
			// Read the seeds file of tags
			currentDir, err := filepath.Abs(filepath.Dir("."))
			if err != nil {
				fmt.Println("Error getting current directory:", err)
				return
			}
			yamlFilePath := filepath.Join(currentDir, "builder", "store", "database", "seeds", "tags.yml")
			yamlFile, err := os.ReadFile(yamlFilePath)
			if err != nil {
				fmt.Println("Error reading YAML file:", err)
				return
			}

			var tags Tags
			err = yaml.Unmarshal(yamlFile, &tags)
			if err != nil {
				fmt.Println("Error parsing YAML:", err)
				return
			}

			ts := tags.Tags
			_, err = db.NewInsert().Model(&ts).Exec(ctx)
			if err != nil {
				fmt.Println("Error inserting data:", err)
				return
			}
			fmt.Println("Insert data successfully")
			return
		})
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		return nil
	})
}
