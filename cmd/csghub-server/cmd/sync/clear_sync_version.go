package sync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/filter"
	"opencsg.com/csghub-server/common/types"
)

var cmdClearSyncVersion = &cobra.Command{
	Use:   "clear-sync-version",
	Short: "clear syncversion table",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config,%w", err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
		}
		ctx := context.WithValue(cmd.Context(), "config", config)
		cmd.SetContext(ctx)
		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		syncVersionStore := database.NewSyncVersionStore()
		repoStore := database.NewRepoStore()
		repoFilter := filter.NewRepoFilter()

		var (
			batch          int
			modelsToKeep   []string
			datasetsToKeep []string
		)

		slog.Info("begin deleting duplicate sync versions...")
		err := syncVersionStore.DeleteOldVersions(ctx)
		if err != nil {
			slog.Error("failed to delete duplicate sync versions", slog.Any("error", err))
		}

		slog.Info("begign deleting sync versions that do not conform to the ruls...")
		for _, repoType := range []types.RepositoryType{types.ModelRepo, types.DatasetRepo} {
			for {
				syncVesions, err := syncVersionStore.FindWithBatch(ctx, repoType, 1000, batch)
				if err != nil {
					slog.Error("failed to fetch sync versions", slog.Any("error", err))
					return
				}
				if len(syncVesions) == 0 {
					slog.Info("no more sync versions to find")
					break
				}

				var repoPaths []string
				for _, version := range syncVesions {
					repoPaths = append(repoPaths, version.RepoPath)
				}

				repos, err := repoStore.FindByRepoTypeAndPaths(ctx, repoType, repoPaths)
				if err != nil {
					slog.Error("failed to fetch repos", slog.Any("error", err))
					return
				}

				mMatched, dMatched, err := repoFilter.BatchMatch(ctx, repos)
				if err != nil {
					slog.Error("failed to find mismatch repos", slog.Any("error", err))
					return
				}

				modelsToKeep = append(modelsToKeep, mMatched...)
				datasetsToKeep = append(datasetsToKeep, dMatched...)

				batch++
			}
		}

		if len(modelsToKeep) > 0 {
			err = syncVersionStore.BatchDeleteOthers(ctx, types.ModelRepo, modelsToKeep)
			if err != nil {
				slog.Error("failed to delete model sync versions", slog.Any("error", err))
			}
		} else {
			err = syncVersionStore.DeleteAll(ctx, types.ModelRepo)
			if err != nil {
				slog.Error("failed to delete all model sync versions", slog.Any("error", err))
			}
		}

		if len(datasetsToKeep) > 0 {
			err = syncVersionStore.BatchDeleteOthers(ctx, types.DatasetRepo, datasetsToKeep)
			if err != nil {
				slog.Error("failed to delete dataset sync versions", slog.Any("error", err))
			}
		} else {
			err = syncVersionStore.DeleteAll(ctx, types.DatasetRepo)
			if err != nil {
				slog.Error("failed to delete all dataset sync versions", slog.Any("error", err))
			}
		}

		slog.Info("Done", slog.Any("keep model sync versions count", len(modelsToKeep)), slog.Any("keep dataset sync versions count", len(datasetsToKeep)))

	},
}
