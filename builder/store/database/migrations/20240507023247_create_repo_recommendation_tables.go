package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		dropTables(ctx, db, database.RecomOpWeight{}, database.RecomRepoScore{}, database.RecomWeight{})
		err := createTables(ctx, db, database.RecomOpWeight{}, database.RecomRepoScore{}, database.RecomWeight{})
		if err != nil {
			return err
		}

		return initRecomWeights(db)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, database.RecomOpWeight{}, database.RecomRepoScore{}, database.RecomWeight{})
	})
}

func initRecomWeights(db *bun.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rw := &database.RecomWeight{
		Name:      "freshness",
		WeightExp: expFreshness,
	}
	_, err := db.NewInsert().Model(rw).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert freshness recom weight")
	}
	rw = &database.RecomWeight{
		Name:      "downloads",
		WeightExp: expDownloads,
	}
	_, err = db.NewInsert().Model(rw).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert downloads recom weight")
	}
	return nil
}

const expFreshness = `
if hours <= 48{
	score = 100 - 2.0/48.0 * hours
} else if hours <= 168{
	score = 98 - 38.0/120.0 * (hours - 48)
} else{
	score = 60 * (1 / (((hours - 168) / 48.0) + 1))
}
`

const expDownloads = `
if downloads <= 10{
    score = downloads * (10 / 10)
}else if downloads <= 100{
    score = 10 + (downloads - 10) * (40.0 / 90.0)
}else if downloads <= 1000{
    score = 50 + (downloads - 100) * (40.0 / 900.0)
}else{
    score = 90 + 10 * (1 - (1 / (downloads / 1000.0)))
}
`
