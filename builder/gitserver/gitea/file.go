package gitea

import (
	"encoding/base64"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pulltheflower/gitea-go-sdk/gitea"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

const LFSPrefix = "version https://git-lfs.github.com/spec/v1"

var fileSizeReg = regexp.MustCompile(`size (\d+)`)

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

func (c *Client) GetDatasetFileRaw(namespace, name, ref, path string) (string, error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaFileData, _, err := c.giteaClient.GetFile(namespace, name, ref, path)
	if err != nil {
		return "", err
	}
	return string(giteaFileData), nil
}

func (c *Client) GetDatasetFileReader(namespace, name, ref, path string, lfs bool) (io.ReadCloser, error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaFileReader, _, err := c.giteaClient.GetFileReader(namespace, name, ref, path, lfs)
	if err != nil {
		return nil, err
	}
	return giteaFileReader, nil
}

func (c *Client) GetDatasetLfsFileRaw(namespace, repoName, ref, filePath string) (io.ReadCloser, error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	r, _, err := c.giteaClient.GetFileReader(namespace, repoName, ref, filePath, true)
	return r, err
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

func (c *Client) GetModelFileReader(namespace, name, ref, path string, lfs bool) (giteaFileReader io.ReadCloser, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaFileReader, _, err = c.giteaClient.GetFileReader(namespace, name, ref, path, lfs)
	if err != nil {
		return
	}
	return
}

func (c *Client) GetModelLfsFileRaw(namespace, repoName, ref, filePath string) (io.ReadCloser, error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	r, _, err := c.giteaClient.GetFileReader(namespace, repoName, ref, filePath, true)
	return r, err
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
	if file.Type == "file" && file.Size < 1024 {
		fileContent, _, err := c.giteaClient.GetContents(namespace, name, ref, file.Path)
		if err != nil {
			return
		}
		fc, err := base64.StdEncoding.DecodeString(*fileContent.Content)
		if err != nil {
			return
		}
		if strings.HasPrefix(string(fc), LFSPrefix) {
			match := fileSizeReg.FindStringSubmatch(string(fc))
			if match != nil {
				size, err := strconv.ParseInt(match[1], 10, 64)
				if err != nil {
					return
				}
				file.Size = int(size)
			}
			file.Lfs = true
			file.DownloadURL = strings.Replace(*entry.DownloadURL, "/raw/", "/media/", 1)
		}
	}
	ch <- file
}

func (c *Client) GetDatasetFileContents(namespace, repo, ref, path string) (*types.File, error) {
	owner := common.WithPrefix(namespace, DatasetOrgPrefix)
	return c.getFileContents(owner, repo, ref, path)
}

func (c *Client) GetModelFileContents(namespace, repo, ref, path string) (*types.File, error) {
	owner := common.WithPrefix(namespace, ModelOrgPrefix)
	return c.getFileContents(owner, repo, ref, path)
}

func (c *Client) getFileContents(owner, repo, ref, path string) (*types.File, error) {
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
		slog.Error("Failed to get contents from gitea", slog.Any("error", err), slog.String("owner", owner), slog.String("repo", repo),
			slog.String("ref", ref), slog.String("path", path))
		return nil, err
	}
	f := &types.File{
		Name:        fileContent.Name,
		Type:        fileContent.Type,
		Size:        int(fileContent.Size),
		SHA:         fileContent.SHA,
		Path:        fileContent.Path,
		DownloadURL: *fileContent.DownloadURL,
		Content:     *fileContent.Content,
	}

	//base64 decode
	contentDecoded, _ := base64.RawStdEncoding.DecodeString(f.Content)
	lfsPointer, err := ReadPointerFromBuffer(contentDecoded)
	//not a lfs pointer, return file content directly
	if err != nil || !lfsPointer.IsValid() {
		slog.Info("Failed to parse lsf pointer", slog.Any("error", err), slog.Bool("isValidPointer", lfsPointer.IsValid()))
		return f, nil
	}

	f.DownloadURL = strings.Replace(f.DownloadURL, "/raw/", "/media/", 1)
	f.LfsRelativePath = lfsPointer.RelativePath()
	return f, nil
}

func (c *Client) CreateModelFile(req *types.CreateFileReq) (err error) {
	namespace := common.WithPrefix(req.NameSpace, ModelOrgPrefix)
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

func (c *Client) CreateDatasetFile(req *types.CreateFileReq) (err error) {
	namespace := common.WithPrefix(req.NameSpace, DatasetOrgPrefix)
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

func (c *Client) UpdateDatasetFile(req *types.UpdateFileReq) (err error) {
	namespace := common.WithPrefix(req.NameSpace, DatasetOrgPrefix)
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
