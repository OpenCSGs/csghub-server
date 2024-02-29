package gitea

import (
	"context"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) GetRepoCommits(ctx context.Context, req gitserver.GetRepoCommitsReq) ([]types.Commit, error) {
	var commits []types.Commit
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaCommits, _, err := c.giteaClient.ListRepoCommits(
		namespace,
		req.Name,
		gitea.ListCommitOptions{
			ListOptions: gitea.ListOptions{
				PageSize: req.Per,
				Page:     req.Page,
			},
			SpeedUpOtions: gitea.SpeedUpOtions{
				DisableStat:         true,
				DisableVerification: true,
				DisableFiles:        true,
			},
			SHA: req.Ref,
		},
	)

	if err != nil {
		return nil, err
	}

	for _, giteaCommit := range giteaCommits {
		commits = append(commits, types.Commit{
			ID:             giteaCommit.SHA,
			CommitterName:  giteaCommit.RepoCommit.Committer.Name,
			CommitterEmail: giteaCommit.RepoCommit.Committer.Email,
			CommitterDate:  giteaCommit.RepoCommit.Committer.Date,
			CreatedAt:      giteaCommit.CommitMeta.Created.String(),
			Message:        giteaCommit.RepoCommit.Message,
			AuthorName:     giteaCommit.RepoCommit.Author.Name,
			AuthorEmail:    giteaCommit.RepoCommit.Author.Email,
			AuthoredDate:   giteaCommit.RepoCommit.Author.Date,
		})
	}
	return commits, nil
}

func (c *Client) GetRepoLastCommit(ctx context.Context, req gitserver.GetRepoLastCommitReq) (*types.Commit, error) {
	var commit *types.Commit
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaCommit, _, err := c.giteaClient.GetSingleCommit(
		namespace,
		req.Name,
		req.Ref,
		gitea.SpeedUpOtions{
			DisableStat:         true,
			DisableVerification: true,
			DisableFiles:        true,
		},
	)
	if err != nil {
		return nil, err
	}

	commit = &types.Commit{
		ID:             giteaCommit.SHA,
		CommitterName:  giteaCommit.RepoCommit.Committer.Name,
		CommitterEmail: giteaCommit.RepoCommit.Committer.Email,
		CommitterDate:  giteaCommit.RepoCommit.Committer.Date,
		CreatedAt:      giteaCommit.CommitMeta.Created.String(),
		Message:        giteaCommit.RepoCommit.Message,
		AuthorName:     giteaCommit.RepoCommit.Author.Name,
		AuthorEmail:    giteaCommit.RepoCommit.Author.Email,
		AuthoredDate:   giteaCommit.RepoCommit.Author.Date,
	}
	return commit, nil
}
