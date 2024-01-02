package tag

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/component/callback"
)

var (
	callbackComponent *callback.GitCallbackComponent
	rs                *database.RepoStore
	gs                gitserver.GitServer
)

func init() {
	Cmd.AddCommand(
		initCmd,
	)
}

var Cmd = &cobra.Command{
	Use:   "trigger-callback",
	Short: "scan repository and trigger callback for meta tags re-processing",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		database.InitDB(dbConfig)
		if err != nil {
			err = fmt.Errorf("initializing DB connection: %w", err)
			return
		}
		rs = database.NewRepoStore()
		gs, err = gitserver.NewGitServer(config)
		if err != nil {
			return
		}
		callbackComponent, err = callback.NewGitCallback(config)
		if err != nil {
			return
		}

		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var initCmd = &cobra.Command{
	Use:   "run",
	Short: "scan repository and trigger callback for meta tags re-processing",
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := rs.All(context.Background())
		if err != nil {
			return err
		}
		for _, repo := range repos {
			splits := strings.Split(repo.Path, "/")
			namespace, repoName := splits[0], splits[1]
			req := &types.GiteaCallbackPushReq{}
			var err error
			var fileNames []string
			if repo.RepositoryType == "dataset" {
				fileNames, err = getFileNames(namespace, repoName, "", gs.GetDatasetFileTree)
			} else {
				fileNames, err = getFileNames(namespace, repoName, "", gs.GetModelFileTree)
			}
			if err != nil {
				slog.Error("failed to get file names", slog.String("repo", repo.Path), slog.Any("error", err))
				continue
			}
			slog.Info("file names from git server ", "fileNames", fileNames)
			req.Repository.FullName = repo.GitPath
			req.Commits = append(req.Commits, types.GiteaCallbackPushReq_Commit{})
			req.Commits[0].Added = append(req.Commits[0].Added, fileNames...)
			callbackComponent.HandlePush(context.Background(), req)
		}
		return nil
	},
}

func getFileNames(namespace, repoName, folder string, gsTree func(namespce, repoName, ref, path string) ([]*types.File, error)) ([]string, error) {
	var fileNames []string
	gitFiles, err := gsTree(namespace, repoName, "", folder)
	if err != nil {
		slog.Error("Failed to get dataset file contents", slog.String("path", folder), slog.Any("error", err))
		return fileNames, err
	}
	for _, file := range gitFiles {
		if file.Type == "dir" {
			subFileNames, err := getFileNames(namespace, repoName, path.Join(folder, file.Name), gsTree)
			if err != nil {
				return fileNames, err
			}
			fileNames = append(fileNames, subFileNames...)
		}
		fileNames = append(fileNames, path.Join(folder, file.Path))
	}
	return fileNames, nil
}
