package migrations

import (
	"context"
	"database/sql"
	"fmt"

	_ "embed"

	"github.com/uptrace/bun"
	"gopkg.in/yaml.v2"
	"opencsg.com/csghub-server/builder/store/database"
)

//go:embed seeds/tags.yml
var tagsYmlData []byte

type Tags struct {
	Tags []database.Tag
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, db bun.Tx) (err error) {
			// Read the seeds file of tags
			var tags Tags
			err = yaml.Unmarshal(tagsYmlData, &tags)
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
