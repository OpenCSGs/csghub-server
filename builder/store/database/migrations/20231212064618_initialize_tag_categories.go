package migrations

import (
	"context"
	"database/sql"
	"fmt"

	_ "embed"

	"github.com/uptrace/bun"
	"gopkg.in/yaml.v2"
)

//go:embed seeds/tag_categories.yml
var tagsCategoryYmlData []byte

type Categories struct {
	Categories []TagCategory
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, db bun.Tx) (err error) {
			// Read the seeds file of tags
			var tagCategory Categories
			err = yaml.Unmarshal(tagsCategoryYmlData, &tagCategory)
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
