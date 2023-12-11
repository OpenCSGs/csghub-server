package gitea

import (
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/pulltheflower/gitea-go-sdk/gitea"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

const LFSPrefix = "version https://git-lfs.github.com/spec/v1"

func (c *Client) GetModelFileTree(namespace, name, ref, path string) (tree []*types.File, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaEntries, _, err := c.giteaClient.ListContents(namespace, name, ref, path)
	if err != nil {
		return
	}
	fileChan := make(chan *types.File, len(giteaEntries))
	var wg sync.WaitGroup
	for _, entry := range giteaEntries {
		wg.Add(1)
		c.getFileFromEntry(namespace, name, ref, entry, fileChan, &wg)
	}

	go func() {
		wg.Wait()
		close(fileChan)
	}()

	for file := range fileChan {
		tree = append(tree, file)
	}

	return
}

func (c *Client) GetDatasetFileTree(namespace, name, ref, path string) (tree []*types.File, err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaEntries, _, err := c.giteaClient.ListContents(namespace, name, ref, path)
	if err != nil {
		return
	}
	fileChan := make(chan *types.File, len(giteaEntries))
	var wg sync.WaitGroup
	for _, entry := range giteaEntries {
		wg.Add(1)
		c.getFileFromEntry(namespace, name, ref, entry, fileChan, &wg)
	}

	go func() {
		wg.Wait()
		close(fileChan)
	}()

	for file := range fileChan {
		tree = append(tree, file)
	}

	return
}

func (c *Client) GetDatasetFileRaw(namespace, name, ref, path string) (data string, err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaFileData, _, err := c.giteaClient.GetFile(namespace, name, ref, path)
	if err != nil {
		return
	}
	data = string(giteaFileData)
	return
}

func (c *Client) GetModelFileRaw(namespace, name, ref, path string) (data string, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaFileData, _, err := c.giteaClient.GetFile(namespace, name, ref, path)
	if err != nil {
		return
	}
	data = string(giteaFileData)
	return
}

func (c *Client) getFileFromEntry(namespace, name, ref string, entry *gitea.ContentsResponse, ch chan<- *types.File, wg *sync.WaitGroup) {
	defer wg.Done()
	file := &types.File{
		Name: entry.Name,
		Type: entry.Type,
		Lfs:  false,
		Size: int(entry.Size),
		SHA:  entry.SHA,
		Path: entry.Path,
	}
	if entry.DownloadURL != nil {
		file.DownloadURL = *entry.DownloadURL
	}

	commit, _, err := c.giteaClient.GetSingleCommit(
		namespace,
		name,
		entry.LastCommitSHA,
		gitea.SpeedUpOtions{
			DisableStat:         true,
			DisableVerification: true,
			DisableFiles:        true,
		},
	)
	if err != nil {
		return
	}

	file.Commit = types.Commit{
		ID:             commit.SHA,
		CommitterName:  commit.RepoCommit.Committer.Name,
		CommitterEmail: commit.RepoCommit.Committer.Email,
		CommitterDate:  commit.RepoCommit.Committer.Date,
		CreatedAt:      commit.CommitMeta.Created.String(),
		Message:        commit.RepoCommit.Message,
		AuthorName:     commit.RepoCommit.Author.Name,
		AuthorEmail:    commit.RepoCommit.Author.Email,
		AuthoredDate:   commit.RepoCommit.Author.Date,
	}
	if file.Type == "file" {
		fileContent, _, err := c.giteaClient.GetContents(namespace, name, ref, file.Path)
		if err != nil {
			return
		}
		fc, err := base64.StdEncoding.DecodeString(*fileContent.Content)
		if err != nil {
			return
		}
		if strings.HasPrefix(string(fc), LFSPrefix) {
			file.Lfs = true
			file.DownloadURL = strings.Replace(*entry.DownloadURL, "/raw/", "/media/", 1)
		}
	}
	ch <- file
}

func (c *Client) CreateModelFile(namespace, name, path string, req *types.CreateFileReq) (err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	_, _, err = c.giteaClient.CreateFile(namespace, name, path, gitea.CreateFileOptions{
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

func (c *Client) UpdateModelFile(namespace, name, path string, req *types.UpdateFileReq) (err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	_, _, err = c.giteaClient.UpdateFile(namespace, name, path, gitea.UpdateFileOptions{
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

func (c *Client) CreateDatasetFile(namespace, name, path string, req *types.CreateFileReq) (err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	_, _, err = c.giteaClient.CreateFile(namespace, name, path, gitea.CreateFileOptions{
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

func (c *Client) UpdateDatasetFile(namespace, name, path string, req *types.UpdateFileReq) (err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	_, _, err = c.giteaClient.UpdateFile(namespace, name, path, gitea.UpdateFileOptions{
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
