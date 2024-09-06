package gitea

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const (
	LFSPrefix           = "version https://git-lfs.github.com/spec/v1"
	NonLFSFileSizeLimit = 10485760
)

func (c *Client) GetRepoFileTree(ctx context.Context, req gitserver.GetRepoInfoByPathReq) ([]*types.File, error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	return c.getRepoDir(namespace, req.Name, req.Ref, req.Path)
}

func (c *Client) getRepoDir(namespace, name, ref, path string) (files []*types.File, err error) {
	giteaEntries, _, err := c.giteaClient.GetDir(namespace, name, ref, path)
	if err != nil {
		return
	}
	for _, entry := range giteaEntries {
		f := &types.File{
			Name:            entry.Name,
			Path:            strings.TrimPrefix(entry.Path, "/"),
			Type:            entry.Type,
			Lfs:             entry.IsLfs,
			LfsRelativePath: entry.LfsRelativePath,
			Size:            entry.Size,
			Commit: types.Commit{
				Message: entry.CommitMsg, ID: entry.SHA, CommitterDate: entry.CommitterDate.String(),
			},
			Mode:          entry.Mode,
			SHA:           entry.SHA,
			URL:           entry.URL,
			LastCommitSHA: entry.LastCommitSHA,
		}
		if entry.Type == "tree" {
			f.Type = "dir"
		} else {
			f.Type = "file"
		}

		files = append(files, f)

	}

	return files, nil
}

func (c *Client) GetRepoFileRaw(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (string, error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaFileData, _, err := c.giteaClient.GetFile(namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return "", err
	}
	return string(giteaFileData), nil
}

func (c *Client) GetRepoFileReader(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (io.ReadCloser, int64, error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	entries, _, err := c.giteaClient.GetDir(namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return nil, 0, err
	}
	if entries[0].Size > NonLFSFileSizeLimit {
		return nil, 0, errors.New("file is larger than 10MB")
	}
	giteaFileReader, _, err := c.giteaClient.GetFileReader(namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return nil, 0, err
	}
	return giteaFileReader, entries[0].Size, nil
}

func (c *Client) GetRepoLfsFileRaw(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (io.ReadCloser, error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	r, _, err := c.giteaClient.GetFileReader(namespace, req.Name, req.Ref, req.Path, true)
	return r, err
}

func (c *Client) GetRepoFileContents(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (*types.File, error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	file, err := c.getFileContents(namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		if err.Error() == "GetContentsOrList" {
			// not found message of gitea
			return nil, err
		}
		return nil, errors.New("failed to get repo file contents")
	}
	commit, _, err := c.giteaClient.GetSingleCommit(namespace, req.Name, file.LastCommitSHA, gitea.SpeedUpOtions{
		DisableStat:         true,
		DisableVerification: true,
		DisableFiles:        true,
	})
	if err != nil {
		return nil, errors.New("failed to get repo file last commit")
	}

	file.Commit = types.Commit{
		ID:             commit.SHA,
		CommitterName:  commit.RepoCommit.Committer.Name,
		CommitterEmail: commit.RepoCommit.Committer.Email,
		CommitterDate:  commit.RepoCommit.Committer.Date,
		CreatedAt:      commit.Created.Format("2024-02-26T15:05:35+08:00"),
		Message:        commit.RepoCommit.Message,
		AuthorName:     commit.RepoCommit.Author.Name,
		AuthorEmail:    commit.RepoCommit.Author.Email,
		AuthoredDate:   commit.RepoCommit.Author.Date,
	}
	return file, nil
}

func (c *Client) CreateRepoFile(req *types.CreateFileReq) (err error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	_, _, err = c.giteaClient.CreateFile(namespace, req.Name, req.FilePath, gitea.CreateFileOptions{
		FileOptions: gitea.FileOptions{
			Message:       req.Message,
			BranchName:    req.Branch,
			NewBranchName: req.NewBranch,
			Author: gitea.Identity{
				Name:  req.Username,
				Email: req.Email,
			},
			Committer: gitea.Identity{
				Name:  req.Username,
				Email: req.Email,
			},
			Dates: gitea.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		},
		Content: req.Content,
	})
	return
}

func (c *Client) UpdateRepoFile(req *types.UpdateFileReq) (err error) {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	_, _, err = c.giteaClient.UpdateFile(namespace, req.Name, req.FilePath, gitea.UpdateFileOptions{
		FileOptions: gitea.FileOptions{
			Message:       req.Message,
			BranchName:    req.Branch,
			NewBranchName: req.NewBranch,
			Author: gitea.Identity{
				Name:  req.Username,
				Email: req.Email,
			},
			Committer: gitea.Identity{
				Name:  req.Username,
				Email: req.Email,
			},
			Dates: gitea.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		},
		SHA:      req.SHA,
		Content:  req.Content,
		FromPath: req.OriginPath,
	})
	return
}

func (c *Client) getFileContents(owner, repo, ref, path string) (*types.File, error) {
	var content string
	/* Example file content from gitea
		{
	  "name": "model-00001-of-00002.safetensors",
	  "path": "model-00001-of-00002.safetensors",
	  "sha": "5824b5e840050f193d3d9091a6b5dbb3c33fb0f3",
	  "last_commit_sha": "d682aedec5182964a9c17e9dd9d5f437847a8c43",
	  "type": "file",
	  "size": 135,
	  "encoding": "base64",
	  "content": "dmVyc2lvbiBodHRwczovL2dpdC1sZnMuZ2l0aHViLmNvbS9zcGVjL3YxCm9pZCBzaGEyNTY6ODYwMmQ0ZDNiMTRmNTg2NTIwZDFmNzY1MDkxZDlmM2E0ZmViMWM1Nzg2NDQ4YzYwMGQwMThkYjcyMTZmNzIzNQpzaXplIDQ5ODI0Njc4NjQK",
	  "target": null,
	  "url": "http://portal-stg.opencsg.com:3001/api/v1/repos/models_wayne/phi-2/contents/model-00001-of-00002.safetensors?ref=main",
	  "html_url": "http://portal-stg.opencsg.com:3001/models_wayne/phi-2/src/branch/main/model-00001-of-00002.safetensors",
	  "git_url": "http://portal-stg.opencsg.com:3001/api/v1/repos/models_wayne/phi-2/git/blobs/5824b5e840050f193d3d9091a6b5dbb3c33fb0f3",
	  "download_url": "http://portal-stg.opencsg.com:3001/models_wayne/phi-2/raw/branch/main/model-00001-of-00002.safetensors",
	  "submodule_git_url": null,
	  "_links": {
	    "self": "http://portal-stg.opencsg.com:3001/api/v1/repos/models_wayne/phi-2/contents/model-00001-of-00002.safetensors?ref=main",
	    "git": "http://portal-stg.opencsg.com:3001/api/v1/repos/models_wayne/phi-2/git/blobs/5824b5e840050f193d3d9091a6b5dbb3c33fb0f3",
	    "html": "http://portal-stg.opencsg.com:3001/models_wayne/phi-2/src/branch/main/model-00001-of-00002.safetensors"
	  }
	}
	*/
	fileContent, _, err := c.giteaClient.GetContents(owner, repo, ref, path)
	if err != nil {
		slog.Error("Failed to get contents from gitea", slog.Any("error", err), slog.String("owner", owner), slog.String("repo", repo), slog.String("ref", ref), slog.String("path", path))
		return nil, err
	}
	if fileContent.Content != nil {
		content = *fileContent.Content
	}

	f := &types.File{
		Name:          fileContent.Name,
		Type:          fileContent.Type,
		Size:          fileContent.Size,
		SHA:           fileContent.SHA,
		Path:          fileContent.Path,
		Content:       content,
		LastCommitSHA: fileContent.LastCommitSHA,
	}

	// base64 decode
	contentDecoded, _ := base64.StdEncoding.DecodeString(f.Content)
	lfsPointer, err := ReadPointerFromBuffer(contentDecoded)
	// not a lfs pointer, return file content directly
	if err != nil || !lfsPointer.IsValid() {
		slog.Info("Failed to parse lsf pointer", slog.Any("error", err), slog.Bool("isValidPointer", lfsPointer.IsValid()))
		return f, nil
	}

	f.Lfs = true
	f.LfsRelativePath = lfsPointer.RelativePath()
	// file content is the lfs pointer if the file is a lfs file
	f.LfsPointerSize = int(fileContent.Size)
	f.Size = lfsPointer.Size
	return f, nil
}

func (c *Client) GetRepoAllFiles(ctx context.Context, req gitserver.GetRepoAllFilesReq) ([]*types.File, error) {
	return nil, nil
}

func (c *Client) GetRepoAllLfsPointers(ctx context.Context, req gitserver.GetRepoAllFilesReq) ([]*types.LFSPointer, error) {
	return nil, nil
}
