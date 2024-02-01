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

type Categories struct {
	Categories []database.TagCategory
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
			yamlFilePath := filepath.Join(currentDir, "builder", "store", "database", "seeds", "tag_categories.yml")
			yamlFile, err := os.ReadFile(yamlFilePath)
			if err != nil {
				fmt.Println("Error reading YAML file:", err)
				return
			}

			var tagCategory Categories
			err = yaml.Unmarshal(yamlFile, &tagCategory)
			if err != nil {
				fmt.Println("Error parsing YAML:", err)
				return
			}

			tcs := tagCategory.Categories
			_, err = db.NewInsert().Model(&tcs).Exec(ctx)
			if err != nil {
				fmt.Println("Error inserting data:", err)
				return
			}
			fmt.Println("Insert data successfully")
			return
		})
	}, func(context.Context, *bun.DB) error {
		fmt.Print(" [down migration] ")
		return nil
	})
}
