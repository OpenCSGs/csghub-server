package reposyncer

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
)

// PublishCommitAndTriggerCallback points HEAD to the synced commit and starts
// the git push callback workflow. It is shared by the repo-sync no-LFS fast
// path and the LFS sync finish step so both publish the new commit identically.
//
// newCommitID is the commit the sync produced; pointing HEAD there lets users
// clone the synchronized state. The callback workflow replays the repository
// diff so downstream handlers (space/agent/package watchers) run as if a push
// had landed.
func PublishCommitAndTriggerCallback(
	ctx context.Context,
	git gitserver.GitServer,
	workflowClient temporal.Client,
	repo *database.Repository,
	namespace, name, relativePath, newCommitID string,
) error {
	branch := repo.DefaultBranch

	// Point HEAD to the new commit so users can clone the changes.
	if err := git.UpdateRef(ctx, gitserver.UpdateRefReq{
		Namespace:    namespace,
		Name:         name,
		Ref:          fmt.Sprintf("refs/heads/%s", branch),
		RepoType:     repo.RepositoryType,
		NewObjectId:  newCommitID,
		RelativePath: relativePath,
	}); err != nil {
		return fmt.Errorf("failed to point HEAD to new commit: %w", err)
	}

	callback, err := git.GetDiffBetweenTwoCommits(ctx, gitserver.GetDiffBetweenTwoCommitsReq{
		Namespace:     namespace,
		Name:          name,
		RepoType:      repo.RepositoryType,
		Ref:           branch,
		LeftCommitId:  gitaly.SHA1EmptyTreeID,
		RightCommitId: newCommitID,
		Private:       repo.Private,
		RelativePath:  relativePath,
	})
	if err != nil {
		return fmt.Errorf("failed to get diff between two commits: %w", err)
	}
	callback.Ref = branch

	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
		ID:        fmt.Sprintf("mirror-%s-%s-%s-%s", repo.RepositoryType, namespace, name, newCommitID),
	}
	if _, err := workflowClient.ExecuteWorkflow(
		ctx, workflowOptions, workflow.HandlePushWorkflow, callback,
	); err != nil {
		return fmt.Errorf("failed to handle git push callback: %w", err)
	}
	return nil
}
