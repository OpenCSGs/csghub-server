package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var (
	// callbackComponent *callback.GitCallbackComponent
	rs database.RepoStore
	gs gitserver.GitServer

	repoPaths []string
)

func init() {
	repoPaths = make([]string, 0)
	gitCallbackCmd.Flags().StringSliceVar(&repoPaths, "repos", nil,
		"paths of repositories to trigger callback, path in format '[repo_type]/[owner]/[repo_name]', for example 'datasets/leida/stg-test-dataset,models/leida/stg-test-model'")
}

var gitCallbackCmd = &cobra.Command{
	Use:   "gitcallback",
	Short: "scan repository and trigger callback for meta tags re-processing",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var repos []*database.Repository
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		err = workflow.StartWorkflow(config)
		if err != nil {
			return err
		}

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
			// file paths relative to repository root
			var filePaths []string
			filePaths, err = getFilePaths(namespace, repoName, "", repo.RepositoryType, gs.GetRepoFileTree)
			if err != nil {
				slog.Error("failed to get file names", slog.String("repo", repo.Path), slog.Any("error", err))
				continue
			}
			slog.Info("file names from git server ", "fileNames", filePaths)
			req.Repository.FullName = repo.GitPath
			req.Commits = append(req.Commits, types.GiteaCallbackPushReq_Commit{})
			req.Commits[0].Added = append(req.Commits[0].Added, filePaths...)
			//start workflow to handle push request
			workflowClient := temporal.GetClient()
			workflowOptions := client.StartWorkflowOptions{
				TaskQueue: workflow.HandlePushQueueName,
			}

			we, err := workflowClient.ExecuteWorkflow(context.Background(), workflowOptions, workflow.HandlePushWorkflow,
				req,
				config,
			)
			if err != nil {
				slog.Error("failed to handle git push callback", slog.String("repo", repo.Path), slog.Any("error", err))
				return err
			}

			slog.Info("start handle push workflow", slog.String("workflow_id", we.GetID()), slog.String("run_id", we.GetRunID()), slog.Any("req", &req))
			slog.Info("trigger complete", slog.String("repo", repo.Path), slog.String("type", string(repo.RepositoryType)), slog.Any("error", err))
		}
		return nil
	},
}

func getFilePaths(namespace, repoName, folder string, repoType types.RepositoryType, gsTree func(ctx context.Context, req gitserver.GetRepoInfoByPathReq) ([]*types.File, error)) ([]string, error) {
	var filePaths []string
	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      repoName,
		Ref:       "",
		Path:      folder,
		RepoType:  repoType,
	}
	gitFiles, err := gsTree(context.Background(), getRepoFileTree)
	if err != nil {
		slog.Error("Failed to get dataset file contents", slog.String("path", folder), slog.Any("error", err))
		return filePaths, err
	}
	for _, file := range gitFiles {
		if file.Type == "dir" {
			subFileNames, err := getFilePaths(namespace, repoName, file.Path, repoType, gsTree)
			if err != nil {
				return filePaths, err
			}
			filePaths = append(filePaths, subFileNames...)
		} else {
			filePaths = append(filePaths, file.Path)
		}
	}
	return filePaths, nil
}
