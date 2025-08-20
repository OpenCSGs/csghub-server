package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	type RecomOpWeight struct {
		RepositoryID int64 `bun:",pk" json:"repository_id"`
		Weight       int   `bun:",notnull" json:"weight" `
		times
	}

	type RecomRepoScore struct {
		RepositoryID int64 `bun:",notnull" json:"repository_id"`
		// like freshess, downloads, quality, op, etc
		WeightName string `bun:",notnull" json:"weight_name"`
		//the recommendation score calculated for corresponding weights
		Score float64 `bun:",notnull" json:"score"`
		times
	}

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		//select all op weights from RecomOpWeight table
		weights := make([]*RecomOpWeight, 0)
		err := db.NewSelect().Model(&RecomOpWeight{}).Scan(ctx, &weights)
		if err != nil {
			return err
		}

		//insert op weights into RecomRepoScore table
		for _, weight := range weights {
			_, err := db.NewInsert().Model(&RecomRepoScore{
				RepositoryID: weight.RepositoryID,
				WeightName:   "op",
				Score:        float64(weight.Weight),
			}).Exec(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return nil
	})
}
