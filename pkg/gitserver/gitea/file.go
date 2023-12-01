package gitea

import (
	"encoding/base64"
	"strings"
	"sync"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/pulltheflower/gitea-go-sdk/gitea"
)

const LFSPrefix = "version https://git-lfs.github.com/spec/v1"

func (c *Client) GetModelFileTree(namespace, name, ref, path string) (tree []*types.File, err error) {
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
	giteaFileData, _, err := c.giteaClient.GetFile(namespace, name, ref, path)
	if err != nil {
		return
	}
	data = string(giteaFileData)
	return
}

func (c *Client) GetModelFileRaw(namespace, name, ref, path string) (data string, err error) {
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
