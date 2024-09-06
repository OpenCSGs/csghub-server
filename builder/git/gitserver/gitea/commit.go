package gitea

import (
	"context"
	"fmt"
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

func (c *Client) GetSingleCommit(ctx context.Context, req gitserver.GetRepoLastCommitReq) (*types.CommitResponse, error) {
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

	diff, err := c.GetCommitDiff(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get git %s repository %s commit id '%s' diff, error: %w", req.RepoType, req.Name, req.Ref, err)
	}

	commitFiles := []string{}
	if commit.Files != nil {
		for _, file := range commit.Files {
			commitFiles = append(commitFiles, file.Filename)
		}
	}
	commitParents := []*types.CommitMeta{}
	if commit.Parents != nil {
		for _, parent := range commit.Parents {
			commitParents = append(commitParents, &types.CommitMeta{
				SHA: parent.SHA,
			})
		}
	}
	commitStats := &types.CommitStats{}
	if commit.Stats != nil {
		commitStats.Total = commit.Stats.Total
		commitStats.Additions = commit.Stats.Additions
		commitStats.Deletions = commit.Stats.Deletions
	}

	commitResponse := &types.CommitResponse{
		Commit: &types.Commit{
			ID:             commit.SHA,
			AuthorName:     commit.RepoCommit.Author.Name,
			AuthorEmail:    commit.RepoCommit.Author.Email,
			AuthoredDate:   commit.RepoCommit.Author.Date,
			CommitterName:  commit.RepoCommit.Committer.Name,
			CommitterEmail: commit.RepoCommit.Committer.Email,
			CommitterDate:  commit.RepoCommit.Committer.Date,
			Message:        commit.RepoCommit.Message,
			CreatedAt:      commit.CommitMeta.Created.Format("2006-01-02 15:04:05"),
		},
		Files:   commitFiles,
		Parents: commitParents,
		Diff:    diff,
		Stats:   commitStats,
	}
	return commitResponse, err
}

func (c *Client) GetDiffBetweenTwoCommits(ctx context.Context, req gitserver.GetDiffBetweenTwoCommitsReq) (*types.GiteaCallbackPushReq, error) {
	return nil, nil
}
