package migrations

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

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
				repository.Likes = model.Likes
				repository.Nickname = model.UrlSlug
				repository.DownloadCount = model.Downloads
				repositories = append(repositories, repository)
			}

			for _, dataset := range datasets {
				repository := dataset.Repository
				repository.Likes = dataset.Likes
				repository.Nickname = dataset.UrlSlug
				repository.DownloadCount = dataset.Downloads
				repositories = append(repositories, repository)
			}
			if len(repositories) == 0 {
				return nil
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
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, db bun.Tx) (err error) {
			var resModels []*Model
			var resDatasets []*Dataset
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
				model.Name = repository.Name
				model.Description = repository.Description
				model.Path = repository.Path
				model.GitPath = repository.GitPath
				model.UserID = repository.UserID
				model.Private = repository.Private
				model.Likes = repository.Likes
				model.UrlSlug = repository.Nickname
				model.Downloads = repository.DownloadCount
				resModels = append(resModels, &model)
			}

			for _, dataset := range datasets {
				repository := dataset.Repository
				dataset.Name = repository.Name
				dataset.Description = repository.Description
				dataset.Path = repository.Path
				dataset.GitPath = repository.GitPath
				dataset.UserID = repository.UserID
				dataset.Private = repository.Private
				dataset.Likes = repository.Likes
				dataset.UrlSlug = repository.Nickname
				dataset.Downloads = repository.DownloadCount
				resDatasets = append(resDatasets, &dataset)
			}

			if len(resModels) > 0 {
				values := db.NewValues(&resModels)

				_, err = db.NewUpdate().
					With("_data", values).
					Model(&Model{}).
					TableExpr("_data").
					Set("name = _data.name").
					Set("description = _data.description").
					Set("path = _data.path").
					Set("git_path = _data.git_path").
					Set("user_id = _data.user_id").
					Set("private = _data.private").
					Set("likes = _data.likes").
					Set("url_slug = _data.url_slug").
					Set("downloads = _data.downloads").
					Where("model.id = _data.id").
					Exec(ctx)
				if err != nil {
					return err
				}
			}

			if len(resDatasets) > 0 {
				values := db.NewValues(&resDatasets)

				_, err = db.NewUpdate().
					With("_data", values).
					Model(&Dataset{}).
					TableExpr("_data").
					Set("name = _data.name").
					Set("description = _data.description").
					Set("path = _data.path").
					Set("git_path = _data.git_path").
					Set("user_id = _data.user_id").
					Set("private = _data.private").
					Set("likes = _data.likes").
					Set("url_slug = _data.url_slug").
					Set("downloads = _data.downloads").
					Where("dataset.id = _data.id").
					Exec(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
}
