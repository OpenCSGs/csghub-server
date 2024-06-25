package syncversion

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "init syncversion table",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config,%w", err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		database.InitDB(dbConfig)
		if err != nil {
			return fmt.Errorf("initializing DB connection: %w", err)
		}
		ctx := context.WithValue(cmd.Context(), "config", config)
		cmd.SetContext(ctx)
		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		config, ok := ctx.Value("config").(*config.Config)
		if !ok {
			slog.Error("config not found in context")
			return
		}
		var versions []database.SyncVersion
		repoComponent, err := component.NewRepoComponent(config)
		if err != nil {
			slog.Error("failed to create repository component: %v", err)
			return
		}
		mirrorRepo := database.NewMirrorStore()
		syncVersionStore := database.NewSyncVersionStore()

		mirrors, err := mirrorRepo.Finished(ctx)
		if err != nil {
			slog.Error("error finding mirror repositories: %v", err)
			return
		}
		for _, mirror := range mirrors {
			repo := mirror.Repository
			if repo == nil {
				continue
			}
			if repo.Private {
				continue
			}
			namespace, name := repo.NamespaceAndName()
			req := &types.GetCommitsReq{
				Namespace: namespace,
				Name:      name,
				Ref:       repo.DefaultBranch,
				RepoType:  repo.RepositoryType,
			}
			commit, err := repoComponent.LastCommit(ctx, req)
			if err != nil {
				slog.Error("error getting repository last commit: %v", err)
				continue
			}

			versions = append(versions, database.SyncVersion{
				SourceID:       types.SyncVersionSourceOpenCSG,
				RepoPath:       repo.Path,
				RepoType:       repo.RepositoryType,
				LastModifiedAt: repo.UpdatedAt,
				ChangeLog:      commit.Message,
			})
		}
		if len(versions) == 0 {
			slog.Error("there are no finished mirror repositories")
			return
		}
		err = syncVersionStore.BatchCreate(ctx, versions)
		if err != nil {
			slog.Error("failed to init sync version error: %v", err)
			return
		}
		slog.Info("sync versions successfully")
	},
}
