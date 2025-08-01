package gitaly

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

const (
	LFSPrefix             = "version https://git-lfs.github.com/spec/v1"
	NonLFSFileSizeLimit   = 10485760
	GitAttributesFileName = ".gitattributes"
	lfsMaxPointerSize     = 400
)

func (c *Client) GetRepoFileRaw(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (string, error) {
	var data []byte
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return "", err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	treeEntriesReq := &gitalypb.TreeEntryRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
		Path:       []byte(req.Path),
	}

	treeEntriesStream, err := c.commitClient.TreeEntry(ctx, treeEntriesReq)
	if err != nil {
		errCtx := errorx.Ctx().
			Set("path", fmt.Sprintf("%s/%s", req.Namespace, req.Name)).
			Set("branch", req.Ref).Set("path", req.Path)
		if status.Code(err) == codes.NotFound || status.Code(err) == codes.InvalidArgument {
			err = errorx.GitFileNotFound(err, errCtx)
		} else {
			err = errorx.ErrGitGetTreeEntryFailed(err, errCtx)
		}
		return "", err
	}

	for {
		treeEntriesResp, err := treeEntriesStream.Recv()
		if err != nil {
			grpcStatus, ok := status.FromError(err)
			if ok && grpcStatus.Code() == codes.FailedPrecondition && strings.Contains(grpcStatus.Message(), "bigger than the maximum allowed size") {
				return "", errorx.ErrFileTooLarge
			}
			if err == io.EOF {
				break
			}
			err = errorx.ErrGitGetTreeEntryFailed(err, errorx.Ctx().
				Set("path", fmt.Sprintf("%s/%s", req.Namespace, req.Name)).
				Set("branch", req.Ref).Set("path", req.Path))
			return "", err
		}
		if treeEntriesResp != nil {
			data = append(data, treeEntriesResp.Data...)
		}
	}

	return string(data), nil
}

func (c *Client) GetRepoFileReader(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (io.ReadCloser, int64, error) {
	var size int64
	sizeChan := make(chan int64, 1)
	pr, pw := io.Pipe()
	// if we add cancel function here, it will break the download stream
	// ctx, cancel := context.WithTimeout(ctx, c.timeoutTime)
	// defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, size, err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	treeEntriesReq := &gitalypb.TreeEntryRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
		Path:       []byte(req.Path),
	}

	treeEntriesStream, err := c.commitClient.TreeEntry(ctx, treeEntriesReq)
	if err != nil {
		return nil, 0, err
	}

	go func() {
		defer pw.Close()
		defer close(sizeChan)

		for {
			treeEntriesResp, err := treeEntriesStream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				pw.CloseWithError(fmt.Errorf("failed to receive data: %v", err))
				return
			}

			if treeEntriesResp.Size != 0 {
				sizeChan <- treeEntriesResp.Size
			}

			if len(treeEntriesResp.Data) > 0 {
				if _, err := pw.Write(treeEntriesResp.Data); err != nil {
					pw.CloseWithError(fmt.Errorf("failed to write data to pipe: %v", err))
					return
				}
			}
		}
	}()
	size, ok := <-sizeChan
	if !ok {
		size = 0
	}

	return pr, size, nil
}

func (c *Client) GetRepoLfsFileRaw(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (io.ReadCloser, error) {
	return nil, nil
}

func (c *Client) GetRepoFileContents(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (*types.File, error) {
	req.File = true
	files, err := c.GetRepoFileTree(ctx, req)
	if err != nil {
		return nil, err
	}
	file := files[0]
	content, err := c.GetRepoFileRaw(ctx, req)
	if err != nil {
		if errors.Is(err, errorx.ErrFileTooLarge) {
			// return file basic info, but not content
			return file, err
		}
		return nil, err
	}
	file.Content = base64.StdEncoding.EncodeToString([]byte(content))

	return file, nil
}

func (c *Client) CreateRepoFile(req *types.CreateFileReq) (err error) {
	ctx := context.Background()
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	if req.NewBranch == "" {
		req.NewBranch = req.Branch
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	userCommitFilesClient, err := c.operationClient.UserCommitFiles(ctx)
	if err != nil {
		return err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
		GlRepository: filepath.Join(repoType, req.Namespace, req.Name),
	}

	startRepo := repository

	if len(req.StartNamespace) > 0 && len(req.StartName) > 0 {
		startRepoType := fmt.Sprintf("%ss", string(req.StartRepoType))
		relativePath, err := c.BuildRelativePath(ctx, req.StartRepoType, req.StartNamespace, req.StartName)
		if err != nil {
			return err
		}
		startRepo = &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
			GlRepository: filepath.Join(startRepoType, req.StartNamespace, req.StartName),
		}
	}

	header := &gitalypb.UserCommitFilesRequestHeader{
		Repository: repository,
		User: &gitalypb.User{
			GlId:       "user-1",
			Name:       []byte(req.Username),
			GlUsername: req.Username,
			Email:      []byte(req.Email),
		},
		BranchName:        []byte(req.NewBranch),
		CommitMessage:     []byte(req.Message),
		CommitAuthorName:  []byte(req.Username),
		CommitAuthorEmail: []byte(req.Email),
		// StartRepository:   repository,
		Timestamp:       timestamppb.New(time.Now()),
		StartRepository: startRepo,
	}

	if req.Branch != "" {
		header.StartBranchName = []byte(req.Branch)
	}
	if req.StartSha != "" {
		header.StartSha = req.StartSha
		header.StartBranchName = []byte(req.StartBranch)
	}

	actions := []*gitalypb.UserCommitFilesRequest{
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Header{
				Header: header,
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Header{
						Header: &gitalypb.UserCommitFilesActionHeader{
							Action:        gitalypb.UserCommitFilesActionHeader_CREATE,
							Base64Content: true,
							FilePath:      []byte(req.FilePath),
						},
					},
				},
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Content{
						Content: []byte(req.Content),
					},
				},
			},
		},
	}
	err = userCommitFilesClient.Send(actions[0])
	if err != nil {
		return err
	}
	err = userCommitFilesClient.Send(actions[1])
	if err != nil {
		return err
	}
	err = userCommitFilesClient.Send(actions[2])
	if err != nil {
		return err
	}
	_, err = userCommitFilesClient.CloseAndRecv()
	if err != nil {
		return err
	}

	return err
}

func (c *Client) UpdateRepoFile(req *types.UpdateFileReq) (err error) {
	ctx := context.Background()
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	if req.NewBranch == "" {
		req.NewBranch = req.Branch
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	userCommitFilesClient, err := c.operationClient.UserCommitFiles(ctx)
	if err != nil {
		return err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
		GlRepository: filepath.Join(repoType, req.Namespace, req.Name),
	}
	header := &gitalypb.UserCommitFilesActionHeader{
		Action:        gitalypb.UserCommitFilesActionHeader_UPDATE,
		Base64Content: true,
		FilePath:      []byte(req.FilePath),
	}

	if req.OriginPath != "" {
		header.Action = gitalypb.UserCommitFilesActionHeader_MOVE
		header.PreviousPath = []byte(req.OriginPath)
	}

	startRepo := repository

	if len(req.StartNamespace) > 0 && len(req.StartName) > 0 {
		startRepoType := fmt.Sprintf("%ss", string(req.StartRepoType))
		relativePath, err := c.BuildRelativePath(ctx, req.StartRepoType, req.StartNamespace, req.StartName)
		if err != nil {
			return err
		}
		startRepo = &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
			GlRepository: filepath.Join(startRepoType, req.StartNamespace, req.StartName),
		}
	}

	fileReqHeader := &gitalypb.UserCommitFilesRequestHeader{
		Repository: repository,
		User: &gitalypb.User{
			GlId:       "user-1",
			Name:       []byte(req.Username),
			GlUsername: req.Username,
			Email:      []byte(req.Email),
		},
		BranchName:        []byte(req.Branch),
		CommitMessage:     []byte(req.Message),
		CommitAuthorName:  []byte(req.Username),
		CommitAuthorEmail: []byte(req.Email),
		// StartBranchName:   []byte(req.NewBranch),
		// StartRepository:   repository,
		Timestamp:       timestamppb.New(time.Now()),
		StartRepository: startRepo,
	}

	if req.Branch != "" {
		fileReqHeader.StartBranchName = []byte(req.Branch)
	}

	if req.StartSha != "" {
		fileReqHeader.StartSha = req.StartSha
		fileReqHeader.StartBranchName = []byte(req.StartBranch)
	}

	actions := []*gitalypb.UserCommitFilesRequest{
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Header{
				Header: fileReqHeader,
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Header{
						Header: header,
					},
				},
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Content{
						Content: []byte(req.Content),
					},
				},
			},
		},
	}
	err = userCommitFilesClient.Send(actions[0])
	if err != nil {
		return err
	}
	err = userCommitFilesClient.Send(actions[1])
	if err != nil {
		return err
	}
	err = userCommitFilesClient.Send(actions[2])
	if err != nil {
		return err
	}
	_, err = userCommitFilesClient.CloseAndRecv()
	if err != nil {
		return err
	}

	return err
}

func (c *Client) DeleteRepoFile(req *types.DeleteFileReq) (err error) {
	ctx := context.Background()
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	conn, err := grpc.NewClient(
		c.config.GitalyServer.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	userCommitFilesClient, err := c.operationClient.UserCommitFiles(ctx)
	if err != nil {
		return err
	}
	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
		GlRepository: filepath.Join(repoType, req.Namespace, req.Name),
	}
	actions := []*gitalypb.UserCommitFilesRequest{
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Header{
				Header: &gitalypb.UserCommitFilesRequestHeader{
					Repository: repository,
					User: &gitalypb.User{
						GlId:       "user-1",
						Name:       []byte(req.Username),
						GlUsername: req.Username,
						Email:      []byte(req.Email),
					},
					BranchName:        []byte(req.Branch),
					CommitMessage:     []byte(req.Message),
					CommitAuthorName:  []byte(req.Username),
					CommitAuthorEmail: []byte(req.Email),
					StartBranchName:   []byte(req.Branch),
					StartRepository:   repository,
					Timestamp:         timestamppb.New(time.Now()),
				},
			},
		},
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
				Action: &gitalypb.UserCommitFilesAction{
					UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Header{
						Header: &gitalypb.UserCommitFilesActionHeader{
							Action:        gitalypb.UserCommitFilesActionHeader_DELETE,
							Base64Content: true,
							FilePath:      []byte(req.FilePath),
						},
					},
				},
			},
		},
	}
	err = userCommitFilesClient.Send(actions[0])
	if err != nil {
		return err
	}
	err = userCommitFilesClient.Send(actions[1])
	if err != nil {
		return err
	}
	_, err = userCommitFilesClient.CloseAndRecv()
	if err != nil {
		return err
	}

	return err
}

func (c *Client) getBlobInfo(ctx context.Context, repo *gitalypb.Repository, paths []*gitalypb.GetBlobsRequest_RevisionPath) ([]*types.File, error) {

	var files []*types.File
	listBlobsReq := &gitalypb.GetBlobsRequest{
		Repository:    repo,
		RevisionPaths: paths,
		Limit:         0,
	}

	listBlobsStream, err := c.blobClient.GetBlobs(ctx, listBlobsReq)
	if err != nil {
		return nil, err
	}
	oidFiles := map[string][]*types.File{}
	for {
		listBlobResp, err := listBlobsStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if listBlobResp != nil {
			var (
				fileType        string
				fileSize        int64
				isLfs           bool
				lfsPointerSize  int
				LfsRelativePath string
			)
			filename := filepath.Base(string(listBlobResp.Path))
			if listBlobResp.Type == gitalypb.ObjectType_BLOB {
				fileType = "file"
			} else {
				fileType = "dir"
			}
			fileSize = listBlobResp.Size
			file := &types.File{
				Name:            filename,
				Type:            fileType,
				Size:            fileSize,
				Lfs:             isLfs,
				Path:            string(listBlobResp.Path),
				Mode:            strconv.Itoa(int(listBlobResp.Mode)),
				SHA:             listBlobResp.Oid,
				LfsPointerSize:  lfsPointerSize,
				LfsRelativePath: LfsRelativePath,
			}
			if listBlobResp.Oid != "" && fileSize < lfsMaxPointerSize {
				oidFiles[listBlobResp.Oid] = append(oidFiles[listBlobResp.Oid], file)
			}
			files = append(files, file)
		}
	}

	// get lfs data
	oids := []string{}
	for oid := range oidFiles {
		oids = append(oids, oid)
	}
	slices.Sort(oids)
	if len(oids) > 0 {
		listLfsStream, err := c.blobClient.GetLFSPointers(ctx, &gitalypb.GetLFSPointersRequest{
			BlobIds:    oids,
			Repository: repo,
		})
		if err != nil {
			return nil, err
		}
		for {
			lfsResp, err := listLfsStream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			if lfsResp != nil {
				pointers := lfsResp.GetLfsPointers()
				for _, pointer := range pointers {
					p, _ := ReadPointerFromBuffer(pointer.Data)
					if p.Valid() {
						for _, file := range oidFiles[string(pointer.Oid)] {
							file.Size = p.Size
							file.Lfs = true
							file.LfsSHA256 = p.Oid
							file.LfsRelativePath = p.RelativePath()
							file.LfsPointerSize = int(pointer.Size)
						}
					}
				}
			}
		}
	}

	return files, nil
}

func (c *Client) GetRepoFileTree(ctx context.Context, req gitserver.GetRepoInfoByPathReq) ([]*types.File, error) {
	var files []*types.File
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	if !req.File {
		req.Path = req.Path + "/"
	}

	if req.Ref == "" {
		req.Ref = "main"
	}

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	var revision_paths []*gitalypb.GetBlobsRequest_RevisionPath

	// Get last commit
	pathCommitMap := make(map[string]*gitalypb.GitCommit)
	gitalyReq := &gitalypb.ListLastCommitsForTreeRequest{
		Repository:      repository,
		Revision:        req.Ref,
		Path:            []byte(req.Path),
		Limit:           1000,
		LiteralPathspec: true,
	}
	commitStream, err := c.commitClient.ListLastCommitsForTree(ctx, gitalyReq)
	if err != nil {
		return nil, err
	}
	for {
		commitResp, err := commitStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		if commitResp == nil {
			return nil, errorx.ErrGitCommitNotFound
		}
		commits := commitResp.Commits
		if len(commits) > 0 {
			for _, c := range commits {
				pathCommitMap[string(c.PathBytes)] = c.Commit
				revision_paths = append(revision_paths, &gitalypb.GetBlobsRequest_RevisionPath{
					Revision: req.Ref,
					Path:     c.PathBytes,
				})
			}
		}
	}

	// Get blobs with file size
	listBlobsReq := &gitalypb.GetBlobsRequest{
		Repository:    repository,
		RevisionPaths: revision_paths,
		Limit:         1024,
	}

	listBlobsStream, err := c.blobClient.GetBlobs(ctx, listBlobsReq)
	if err != nil {
		return nil, err
	}
	for {
		listBlobResp, err := listBlobsStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			errCtx := errorx.Ctx().
				Set("path", fmt.Sprintf("%s/%s", req.Namespace, req.Name)).
				Set("branch", req.Ref).Set("path", req.Path)
			if status.Code(err) == codes.NotFound || status.Code(err) == codes.InvalidArgument {
				err = errorx.GitFileNotFound(err, errCtx)
			} else {
				err = errorx.ErrGitGetBlobsFailed(err, errCtx)
			}
			return nil, err
		}
		if listBlobResp != nil {
			var (
				fileType        string
				fileSize        int64
				isLfs           bool
				lfsPointerSize  int
				LfsRelativePath string
				lfsSHA256       string
			)
			filename := filepath.Base(string(listBlobResp.Path))
			if listBlobResp.Type == gitalypb.ObjectType_BLOB {
				fileType = "file"
			} else {
				fileType = "dir"
			}
			fileSize = listBlobResp.Size
			if listBlobResp.Size <= 1024 {
				p, _ := ReadPointerFromBuffer(listBlobResp.Data)
				if p.Valid() {
					fileSize = p.Size
					isLfs = true
					LfsRelativePath = p.RelativePath()
					lfsPointerSize = int(listBlobResp.Size)
					lfsSHA256 = p.Oid
				}
			}
			file := &types.File{
				Name:            filename,
				Type:            fileType,
				Size:            fileSize,
				Lfs:             isLfs,
				Path:            string(listBlobResp.Path),
				Mode:            strconv.Itoa(int(listBlobResp.Mode)),
				SHA:             listBlobResp.Oid,
				LfsPointerSize:  lfsPointerSize,
				LfsRelativePath: LfsRelativePath,
				LfsSHA256:       lfsSHA256,
			}
			commit := pathCommitMap[string(listBlobResp.Path)]
			if commit != nil {
				file.Commit = types.Commit{
					ID:             commit.Id,
					CommitterName:  string(commit.Committer.Name),
					CommitterEmail: string(commit.Committer.Email),
					CommitterDate:  commit.Committer.Date.AsTime().Format(time.RFC3339),
					CreatedAt:      commit.Committer.Date.AsTime().Format(time.RFC3339),
					Message:        string(commit.Subject),
					AuthorName:     string(commit.Author.Name),
					AuthorEmail:    string(commit.Author.Email),
					AuthoredDate:   commit.Author.Date.AsTime().Format(time.RFC3339),
				}
				file.LastCommitSHA = commit.Id
			}

			files = append(files, file)
		}
	}

	return files, nil
}

func (c *Client) GetTree(ctx context.Context, req types.GetTreeRequest) (*types.GetRepoFileTreeResp, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	req.Path = strings.TrimPrefix(req.Path, "/")

	if req.Path == "" || req.Path == "/" {
		req.Path = "."
	}

	if req.Ref == "" {
		req.Ref = "main"
	}

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	var revisionPaths []*gitalypb.GetBlobsRequest_RevisionPath

	gitalyReq := &gitalypb.GetTreeEntriesRequest{
		Repository: repository,
		Revision:   []byte(req.Ref),
		Path:       []byte(req.Path),
		Sort:       gitalypb.GetTreeEntriesRequest_TREES_FIRST,
		PaginationParams: &gitalypb.PaginationParameter{
			PageToken: req.Cursor,
			Limit:     int32(req.Limit),
		},
		Recursive: req.Recursive,
	}

	treeStream, err := c.commitClient.GetTreeEntries(ctx, gitalyReq)
	if err != nil {
		return nil, err
	}
	cursor := ""
	for {
		treeEntries, err := treeStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else if status.Code(err) == codes.NotFound || status.Code(err) == codes.InvalidArgument {
				errCtx := errorx.Ctx().
					Set("path", fmt.Sprintf("%s/%s", req.Namespace, req.Name)).
					Set("branch", req.Ref).Set("path", req.Path)
				err = errorx.GitFileNotFound(err, errCtx)
			} else {
				errCtx := errorx.Ctx().
					Set("path", fmt.Sprintf("%s/%s", req.Namespace, req.Name)).
					Set("branch", req.Ref).Set("path", req.Path)
				err = errorx.ErrGitGetTreeEntryFailed(err, errCtx)
			}
			return nil, err
		}
		if treeEntries == nil {
			return nil, errors.New("GetTreeEntries API invalid response")
		}
		cursor = treeEntries.PaginationCursor.GetNextCursor()
		entries := treeEntries.Entries
		if len(entries) > 0 {
			for _, e := range entries {
				revisionPaths = append(revisionPaths, &gitalypb.GetBlobsRequest_RevisionPath{
					Revision: req.Ref,
					Path:     e.Path,
				})
			}
		}
	}

	files, err := c.getBlobInfo(ctx, repository, revisionPaths)
	if err != nil {
		return nil, err
	}
	return &types.GetRepoFileTreeResp{
		Files:  files,
		Cursor: cursor,
	}, nil
}

func (c *Client) GetLogsTree(ctx context.Context, req types.GetLogsTreeRequest) (*types.LogsTreeResp, error) {
	var resp []*types.CommitForTree
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	req.Path = strings.TrimPrefix(req.Path, "/")

	if req.Ref == "" {
		req.Ref = "main"
	}
	if !strings.HasSuffix(req.Path, "/") {
		req.Path += "/"
	}

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}

	gitalyReq := &gitalypb.ListLastCommitsForTreeRequest{
		Repository: repository,
		Revision:   req.Ref,
		Path:       []byte(req.Path),
		Offset:     int32(req.Offset),
		Limit:      int32(req.Limit),
	}
	commitStream, err := c.commitClient.ListLastCommitsForTree(ctx, gitalyReq)
	if err != nil {
		return nil, err
	}
	for {
		commitResp, err := commitStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		if commitResp == nil {
			return nil, errors.New("bad request")
		}
		commits := commitResp.Commits
		if len(commits) > 0 {
			for _, c := range commits {
				commit := c.Commit
				if commit == nil {
					continue
				}
				resp = append(resp, &types.CommitForTree{
					Name:           filepath.Base(string(c.PathBytes)),
					Path:           string(c.PathBytes),
					ID:             commit.Id,
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
	return &types.LogsTreeResp{Commits: resp}, nil

}

func (c *Client) GetRepoAllFiles(ctx context.Context, req gitserver.GetRepoAllFilesReq) ([]*types.File, error) {
	var files []*types.File
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}

	allFilesReq := &gitalypb.ListFilesRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
		Revision: []byte(req.Ref),
	}

	allFilesStream, err := c.commitClient.ListFiles(ctx, allFilesReq)
	if err != nil {
		return nil, err
	}

	for {
		allFilesResp, err := allFilesStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if allFilesResp != nil {
			for _, path := range allFilesResp.Paths {
				files = append(files, &types.File{
					Name: filepath.Base(string(path)),
					Path: string(path),
				})
			}

		}
	}
	return files, nil
}

func (c *Client) GetRepoAllLfsPointers(ctx context.Context, req gitserver.GetRepoAllFilesReq) ([]*types.LFSPointer, error) {
	var pointers []*types.LFSPointer
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}

	allPointersReq := &gitalypb.ListAllLFSPointersRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
	}

	allPointersStream, err := c.blobClient.ListAllLFSPointers(ctx, allPointersReq)
	if err != nil {
		return nil, err
	}
	for {
		allPointersResp, err := allPointersStream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if allPointersResp != nil {
			for _, pointer := range allPointersResp.LfsPointers {
				var p types.Pointer
				p, _ = ReadPointerFromBuffer(pointer.Data)
				pointers = append(pointers, &types.LFSPointer{
					Oid:      pointer.Oid,
					Size:     pointer.Size,
					FileOid:  string(p.Oid),
					FileSize: p.Size,
					Data:     string(pointer.Data),
				})
			}
		}
	}
	return pointers, nil
}

func (c *Client) CommitFiles(ctx context.Context, req gitserver.CommitFilesReq) error {
	repoType := fmt.Sprintf("%ss", req.RepoType)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	userCommitFilesClient, err := c.operationClient.UserCommitFiles(ctx)
	if err != nil {
		return err
	}

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	repository := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
		GlRepository: filepath.Join(repoType, req.Namespace, req.Name),
	}

	startRepo := repository

	header := &gitalypb.UserCommitFilesRequestHeader{
		Repository: repository,
		User: &gitalypb.User{
			GlId:       "user-1",
			Name:       []byte(req.Username),
			GlUsername: req.Username,
			Email:      []byte(req.Email),
		},
		BranchName:        []byte(req.Revision),
		CommitMessage:     []byte(req.Message),
		CommitAuthorName:  []byte(req.Username),
		CommitAuthorEmail: []byte(req.Email),
		Timestamp:         timestamppb.New(time.Now()),
		StartRepository:   startRepo,
		Force:             true,
	}

	allFileActions := []*gitalypb.UserCommitFilesRequest{
		{
			UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Header{
				Header: header,
			},
		},
	}
	for _, file := range req.Files {
		bodys := []*gitalypb.UserCommitFilesRequest{}
		for _, chunk := range chunkBytes([]byte(file.Content), 3<<20) {
			bodys = append(bodys, &gitalypb.UserCommitFilesRequest{
				UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
					Action: &gitalypb.UserCommitFilesAction{
						UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Content{
							Content: chunk,
						},
					},
				},
			})
		}

		var action gitalypb.UserCommitFilesActionHeader_ActionType
		if file.Action == "create" {
			action = gitalypb.UserCommitFilesActionHeader_CREATE
		} else if file.Action == "update" {
			action = gitalypb.UserCommitFilesActionHeader_UPDATE
		} else if file.Action == "delete" {
			action = gitalypb.UserCommitFilesActionHeader_DELETE
		} else {
			return fmt.Errorf("unknown action: %s", file.Action)
		}

		fileAction := []*gitalypb.UserCommitFilesRequest{
			{
				UserCommitFilesRequestPayload: &gitalypb.UserCommitFilesRequest_Action{
					Action: &gitalypb.UserCommitFilesAction{
						UserCommitFilesActionPayload: &gitalypb.UserCommitFilesAction_Header{
							Header: &gitalypb.UserCommitFilesActionHeader{
								Action:        action,
								Base64Content: true,
								FilePath:      []byte(file.Path),
							},
						},
					},
				},
			},
		}
		fileAction = append(fileAction, bodys...)
		allFileActions = append(allFileActions, fileAction...)
	}
	for _, action := range allFileActions {
		err = userCommitFilesClient.Send(action)
		if err != nil {
			return err
		}
	}
	_, err = userCommitFilesClient.CloseAndRecv()
	if err != nil {
		return err
	}

	return nil
}

func chunkBytes(data []byte, chunkSize int) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}
	return chunks
}
