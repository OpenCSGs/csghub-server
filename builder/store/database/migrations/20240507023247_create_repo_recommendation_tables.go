package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

/* tables for recommendations */

// RecomWeight are recommendation weight settings
type RecomWeight struct {
	Name string `bun:",pk" json:"name"`
	//the expression to calculate weight
	WeightExp string `bun:",notnull" json:"weight_exp" `
	times
}

// RecomOpWeight are the special weights of a repository manually set by operator
type RecomOpWeight struct {
	RepositoryID int64 `bun:",pk" json:"repository_id"`
	Weight       int   `bun:",notnull" json:"weight" `
	times
}

// RecomRepoScore is the recommendation score of a repository
type RecomRepoScore struct {
	RepositoryID int64 `bun:",pk" json:"repository_id"`
	//the total recommendation score calculated by all the recommendation weights
	Score float64 `bun:",notnull" json:"score"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_ = dropTables(ctx, db, RecomOpWeight{}, RecomRepoScore{}, RecomWeight{})
		err := createTables(ctx, db, RecomOpWeight{}, RecomRepoScore{}, RecomWeight{})
		if err != nil {
			return err
		}

		return initRecomWeights(db)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, RecomOpWeight{}, RecomRepoScore{}, RecomWeight{})
	})
}

func initRecomWeights(db *bun.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rw := &RecomWeight{
		Name:      "freshness",
		WeightExp: expFreshness,
	}
	_, err := db.NewInsert().Model(rw).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert freshness recom weight")
	}
	rw = &RecomWeight{
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
