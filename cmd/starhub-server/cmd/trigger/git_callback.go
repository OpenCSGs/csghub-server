package trigger

import (
	"context"
	"fmt"
	"log/slog"
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

	repoPaths []string
)

func init() {
	repoPaths = make([]string, 0)
	gitCallbackCmd.Flags().StringSliceVar(&repoPaths, "repos", nil,
		"paths of repositories to trigger callback, path in format '[repo_type]/[owner]/[repo_name]', for example 'datasets/leida/stg-test-dataset,models/leida/stg-test-model'")
	Cmd.AddCommand(
		gitCallbackCmd,
	)
}

var Cmd = &cobra.Command{
	Use:   "trigger",
	Short: "trigger a specific command",
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

var gitCallbackCmd = &cobra.Command{
	Use:   "gitcallback",
	Short: "scan repository and trigger callback for meta tags re-processing",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var repos []*database.Repository
		if len(repoPaths) > 0 {
			for _, rp := range repoPaths {
				parts := strings.Split(rp, "/")
				if len(parts) != 3 {
					slog.Error(fmt.Sprintf("invalid repo path, skip: %s", rp))
					continue
				}
				repoType := parts[0]
				owner := parts[1]
				repoName := parts[2]
				repo, err := rs.Find(context.Background(), owner, repoType, repoName)
				if err != nil {
					slog.Error("fail to find repository, skip", slog.String("repo", rp), slog.Any("error", err))
					continue
				}
				if repo == nil {
					slog.Error("repo not found, skip", slog.String("repo", rp))
					continue
				}
				repos = append(repos, repo)
			}
		} else {
			repos, err = rs.All(context.Background())
		}

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
			err = callbackComponent.HandlePush(context.Background(), req)
			slog.Info("trigger complete", slog.String("repo", repo.Path), slog.String("type", string(repo.RepositoryType)), slog.Any("error", err))
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
			subFileNames, err := getFileNames(namespace, repoName, file.Name, gsTree)
			if err != nil {
				return fileNames, err
			}
			fileNames = append(fileNames, subFileNames...)
		} else {
			fileNames = append(fileNames, file.Path)
		}
	}
	return fileNames, nil
}
