package gitea

import (
	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

func (c *Client) GetModelCommits(namespace, name, ref string, per, page int) (commits []*types.Commit, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaCommits, _, err := c.giteaClient.ListRepoCommits(
		namespace,
		name,
		gitea.ListCommitOptions{
			ListOptions: gitea.ListOptions{
				PageSize: per,
				Page:     page,
			},
			SpeedUpOtions: gitea.SpeedUpOtions{
				DisableStat:         true,
				DisableVerification: true,
				DisableFiles:        true,
			},
			SHA: ref,
		},
	)

	if err != nil {
		return
	}

	for _, giteaCommit := range giteaCommits {
		commits = append(commits, &types.Commit{
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
	return
}

func (c *Client) GetModelLastCommit(namespace, name, ref string) (commit *types.Commit, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaCommit, _, err := c.giteaClient.GetSingleCommit(
		namespace,
		name,
		ref,
		gitea.SpeedUpOtions{
			DisableStat:         true,
			DisableVerification: true,
			DisableFiles:        true,
		},
	)
	if err != nil {
		return
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
	return
}

func (c *Client) GetDatasetCommits(namespace, name, ref string, per, page int) (commits []*types.Commit, err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaCommits, _, err := c.giteaClient.ListRepoCommits(
		namespace,
		name,
		gitea.ListCommitOptions{
			ListOptions: gitea.ListOptions{
				PageSize: per,
				Page:     page,
			},
			SpeedUpOtions: gitea.SpeedUpOtions{
				DisableStat:         true,
				DisableVerification: true,
				DisableFiles:        true,
			},
			SHA: ref,
		},
	)
	if err != nil {
		return
	}

	for _, giteaCommit := range giteaCommits {
		commits = append(commits, &types.Commit{
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
	return
}

func (c *Client) GetDatasetLastCommit(namespace, name, ref string) (commit *types.Commit, err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaCommit, _, err := c.giteaClient.GetSingleCommit(
		namespace,
		name,
		ref,
		gitea.SpeedUpOtions{
			DisableStat:         true,
			DisableVerification: true,
			DisableFiles:        true,
		},
	)
	if err != nil {
		return
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
	return
}
