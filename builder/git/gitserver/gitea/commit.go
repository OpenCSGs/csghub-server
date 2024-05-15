package gitea

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) GetRepoCommits(ctx context.Context, req gitserver.GetRepoCommitsReq) ([]types.Commit, *types.RepoPageOpts, error) {
	var commits []types.Commit
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaCommits, response, err := c.giteaClient.ListRepoCommits(
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
		return nil, nil, err
	}
	var commitPageOpt types.RepoPageOpts
	commitPageOpt.PageCount, err = strconv.Atoi(response.Header.Get(gitserver.Git_Header_X_Pagecount))
	if err != nil {
		return nil, nil, err
	}
	commitPageOpt.Total, err = strconv.Atoi(response.Header.Get(gitserver.Git_Header_X_Total))
	if err != nil {
		return nil, nil, err
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
	return commits, &commitPageOpt, nil
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

func (c *Client) GetCommitDiff(ctx context.Context, req gitserver.GetRepoLastCommitReq) ([]byte, error) {
	// namespace is user of gitea
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	data, response, err := c.giteaClient.GetCommitDiff(namespace, req.Name, req.Ref)
	// response is instance of *gitea.Response
	if err != nil {
		slog.Error("Fail to get commit diff", slog.Any("user", namespace), slog.Any("repo", req.Name), slog.Any("commit id", req.Ref), slog.Any("response", response))
	}
	return data, err
}

func (c *Client) GetSingleCommit(ctx context.Context, req gitserver.GetRepoLastCommitReq) (*gitea.Commit, error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	commit, response, err := c.giteaClient.GetSingleCommit(
		namespace,
		req.Name,
		req.Ref,
		gitea.SpeedUpOtions{
			DisableStat:         false,
			DisableVerification: false,
			DisableFiles:        false,
		},
	)
	if err != nil {
		slog.Error("Fail to get single commit", slog.Any("user", namespace), slog.Any("repo", req.Name), slog.Any("commit id", req.Ref), slog.Any("response", response))
	}
	return commit, err
}
