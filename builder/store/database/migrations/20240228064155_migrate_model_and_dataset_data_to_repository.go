package migrations

import (
	"context"
	"database/sql"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

type Model struct {
	ID            int64                `bun:",pk,autoincrement" json:"id"`
	UrlSlug       string               `bun:",notnull" json:"nickname"`
	Likes         int64                `bun:",notnull" json:"likes"`
	Downloads     int64                `bun:",notnull" json:"downloads"`
	RepositoryID  int64                `bun:",notnull" json:"repository_id"`
	Repository    *database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time            `bun:",notnull" json:"last_updated_at"`
	times
}

type Dataset struct {
	ID            int64                `bun:",pk,autoincrement" json:"id"`
	UrlSlug       string               `bun:",notnull" json:"nickname"`
	Likes         int64                `bun:",notnull" json:"likes"`
	Downloads     int64                `bun:",notnull" json:"downloads"`
	RepositoryID  int64                `bun:",notnull" json:"repository_id"`
	Repository    *database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time            `bun:",notnull" json:"last_updated_at"`
	times
}

func getModels(ctx context.Context, db bun.Tx) (models []Model, err error) {
	err = db.NewSelect().
		Model(&models).
		Relation("Repository").
		Scan(ctx)
	return
}

func getDatasets(ctx context.Context, db bun.Tx) (datasets []Dataset, err error) {
	err = db.NewSelect().
		Model(&datasets).
		Relation("Repository").
		Scan(ctx)
	return
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, db bun.Tx) (err error) {
			var repositories []*database.Repository
			models, err := getModels(ctx, db)
			if err != nil {
				return
			}
			datasets, err := getDatasets(ctx, db)
			if err != nil {
				return
			}

			for _, model := range models {
				repository := model.Repository
				repository.Nickname = model.UrlSlug
				repository.DownloadCount = model.Downloads
				repositories = append(repositories, repository)
			}

			for _, dataset := range datasets {
				repository := dataset.Repository
				repository.Nickname = dataset.UrlSlug
				repository.DownloadCount = dataset.Downloads
				repositories = append(repositories, repository)
			}

			values := db.NewValues(&repositories)

			_, err = db.NewUpdate().
				With("_data", values).
				Model(&database.Repository{}).
				TableExpr("_data").
				Set("nickname = _data.nickname").
				Set("download_count = _data.download_count").
				Where("repository.id = _data.id").
				Exec(ctx)
			if err != nil {
				return err
			}

			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		return nil
	})
}
