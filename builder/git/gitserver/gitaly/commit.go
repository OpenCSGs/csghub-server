package gitaly

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
)

const SHA1EmptyTreeID = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func (c *Client) GetRepoCommits(ctx context.Context, req gitserver.GetRepoCommitsReq) ([]types.Commit, *types.RepoPageOpts, error) {
	var commits []types.Commit
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	commitsReq := &gitalypb.FindCommitsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Revision: []byte(req.Ref),
		Limit:    int32(req.Per),
		Offset:   int32(req.Per * (req.Page - 1)),
	}
	stream, err := c.commitClient.FindCommits(ctx, commitsReq)
	if err != nil {
		return nil, nil, err
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, err
		}
		if resp != nil {
			for _, commit := range resp.Commits {
				commits = append(commits, types.Commit{
					ID:             string(commit.Id),
					CommitterName:  string(commit.Committer.Name),
					CommitterEmail: string(commit.Committer.Email),
					CommitterDate:  commit.Committer.Date.AsTime().Format(time.RFC3339),
					CreatedAt:      commit.Committer.Date.AsTime().Format(time.RFC3339),
					Message:        string(commit.Subject),
					AuthorName:     string(commit.Author.Name),
					AuthorEmail:    string(commit.Author.Email),
					AuthoredDate:   commit.Author.Date.AsTime().Format(time.RFC3339),
				})
			}
		}
	}

	countCommitsReq := &gitalypb.CountCommitsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Revision: []byte(req.Ref),
	}
	count, err := c.commitClient.CountCommits(ctx, countCommitsReq)
	if err != nil {
		return nil, nil, err
	}
	repoPageOpts := &types.RepoPageOpts{
		Total:     int(count.Count),
		PageCount: int(math.Ceil(float64(count.Count) / float64(req.Per))),
	}

	return commits, repoPageOpts, nil
}

func (c *Client) GetRepoLastCommit(ctx context.Context, req gitserver.GetRepoLastCommitReq) (*types.Commit, error) {
	var commit types.Commit
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	commitReq := &gitalypb.FindCommitRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Revision: []byte(req.Ref),
	}
	resp, err := c.commitClient.FindCommit(ctx, commitReq)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		commit = types.Commit{
			ID:             string(resp.Commit.Id),
			CommitterName:  string(resp.Commit.Committer.Name),
			CommitterEmail: string(resp.Commit.Committer.Email),
			CommitterDate:  resp.Commit.Committer.Date.AsTime().Format(time.RFC3339),
			CreatedAt:      resp.Commit.Committer.Date.AsTime().Format(time.RFC3339),
			Message:        string(resp.Commit.Subject),
			AuthorName:     string(resp.Commit.Author.Name),
			AuthorEmail:    string(resp.Commit.Author.Email),
			AuthoredDate:   resp.Commit.Author.Date.AsTime().Format(time.RFC3339),
		}
	}

	return &commit, nil
}

func (c *Client) GetCommitDiff(ctx context.Context, req gitserver.GetRepoLastCommitReq) ([]byte, error) {
	return nil, nil
}

func (c *Client) GetSingleCommit(ctx context.Context, req gitserver.GetRepoLastCommitReq) (*types.CommitResponse, error) {
	var (
		result    types.CommitResponse
		commit    types.Commit
		parents   []*types.CommitMeta
		files     []string
		diff      []byte
		additions int
		deletions int
	)

	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	commitReq := &gitalypb.FindCommitRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Revision: []byte(req.Ref),
	}
	commitResp, err := c.commitClient.FindCommit(ctx, commitReq)
	if err != nil {
		return nil, err
	}
	if commitResp != nil && commitResp.Commit != nil {
		commit = types.Commit{
			ID:             string(commitResp.Commit.Id),
			CommitterName:  string(commitResp.Commit.Committer.Name),
			CommitterEmail: string(commitResp.Commit.Committer.Email),
			CommitterDate:  commitResp.Commit.Committer.Date.AsTime().Format(time.RFC3339),
			CreatedAt:      commitResp.Commit.Committer.Date.AsTime().Format(time.RFC3339),
			Message:        string(commitResp.Commit.Subject),
			AuthorName:     string(commitResp.Commit.Author.Name),
			AuthorEmail:    string(commitResp.Commit.Author.Email),
			AuthoredDate:   commitResp.Commit.Author.Date.AsTime().Format(time.RFC3339),
		}
		for _, id := range commitResp.Commit.ParentIds {
			parents = append(parents, &types.CommitMeta{
				SHA: id,
			})
		}

	} else {
		return nil, errors.New("commit not found")
	}
	result = types.CommitResponse{
		Commit:  &commit,
		Parents: parents,
	}

	diffCtx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	filesReq := &gitalypb.DiffStatsRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		RightCommitId: req.Ref,
	}
	if len(parents) > 0 {
		filesReq.LeftCommitId = parents[0].SHA
	} else {
		filesReq.LeftCommitId = SHA1EmptyTreeID
	}
	fileStream, err := c.diffClient.DiffStats(ctx, filesReq)
	if err != nil {
		return nil, err
	}
	for {
		data, err := fileStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if data != nil {
			for _, stat := range data.Stats {
				files = append(files, string(stat.Path))
				additions += int(stat.Additions)
				deletions += int(stat.Deletions)
			}
		}
	}
	result.Files = files
	result.Stats = &types.CommitStats{
		Additions: additions,
		Deletions: deletions,
		Total:     additions + deletions,
	}

	diffReq := &gitalypb.RawDiffRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		RightCommitId: req.Ref,
	}
	if len(parents) > 0 {
		diffReq.LeftCommitId = parents[0].SHA
	} else {
		diffReq.LeftCommitId = SHA1EmptyTreeID
	}
	diffStream, err := c.diffClient.RawDiff(diffCtx, diffReq)
	if err != nil {
		return nil, err
	}
	for {
		data, err := diffStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if data != nil {
			diff = data.Data
		}
	}
	result.Diff = diff

	return &result, nil
}

func (c *Client) GetDiffBetweenTwoCommits(ctx context.Context, req gitserver.GetDiffBetweenTwoCommitsReq) (*types.GiteaCallbackPushReq, error) {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()
	callback := &types.GiteaCallbackPushReq{
		Ref: req.Ref,
		Repository: types.GiteaCallbackPushReq_Repository{
			FullName: fmt.Sprintf("%s_%s/%s", repoType, req.Namespace, req.Name),
			Private:  req.Private,
		},
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storge,
		RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
	}
	treeReq := &gitalypb.FindChangedPathsRequest_Request_TreeRequest{
		RightTreeRevision: req.RightCommitId,
	}
	if req.LeftCommitId != "0000000000000000000000000000000000000000" {
		treeReq.LeftTreeRevision = req.LeftCommitId
	} else {
		treeReq.LeftTreeRevision = SHA1EmptyTreeID
	}

	commitReq := &gitalypb.FindChangedPathsRequest{
		Repository:          repository,
		MergeCommitDiffMode: gitalypb.FindChangedPathsRequest_MERGE_COMMIT_DIFF_MODE_UNSPECIFIED,
		Requests: []*gitalypb.FindChangedPathsRequest_Request{
			{
				Type: &gitalypb.FindChangedPathsRequest_Request_TreeRequest_{
					TreeRequest: treeReq,
				},
			},
		},
	}
	commitResp, err := c.diffClient.FindChangedPaths(ctx, commitReq)
	if err != nil {
		return nil, err
	}

	if commitResp != nil {
		var commits []types.GiteaCallbackPushReq_Commit
		for {
			data, err := commitResp.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			if data != nil {
				var (
					modified []string
					added    []string
					deleted  []string
				)
				for _, path := range data.Paths {
					if path.Status == gitalypb.ChangedPaths_ADDED {
						added = append(added, string(path.Path))
					} else if path.Status == gitalypb.ChangedPaths_DELETED {
						deleted = append(deleted, string(path.Path))
					} else if path.Status == gitalypb.ChangedPaths_MODIFIED {
						modified = append(modified, string(path.Path))
					}
				}
				commits = append(commits, types.GiteaCallbackPushReq_Commit{
					Added:    added,
					Removed:  deleted,
					Modified: modified,
				})
			}
		}
		callback.Commits = commits
	}

	findCommitReq := &gitalypb.FindCommitRequest{
		Repository: repository,
		Revision:   []byte(req.RightCommitId),
	}

	findCommitResp, err := c.commitClient.FindCommit(ctx, findCommitReq)
	if err != nil {
		return nil, err
	}

	if findCommitResp != nil {
		callback.HeadCommit = types.GiteaCallbackPushReq_HeadCommit{
			Timestamp:      findCommitResp.Commit.Committer.Date.AsTime().Format(time.RFC3339),
			Message:        string(findCommitResp.Commit.Subject),
			LastModifyTime: findCommitResp.Commit.Committer.Date.AsTime(),
		}
	}
	return callback, nil
}
