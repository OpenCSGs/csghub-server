package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type HFDatasetComponent interface {
	GetPathsInfo(ctx context.Context, req types.PathReq) ([]types.HFDSPathInfo, error)
	GetDatasetTree(ctx context.Context, req types.PathReq) ([]types.HFDSPathInfo, error)
}

func NewHFDatasetComponent(config *config.Config) (HFDatasetComponent, error) {
	c := &hFDatasetComponentImpl{}
	c.tagStore = database.NewTagStore()
	c.datasetStore = database.NewDatasetStore()
	c.repoStore = database.NewRepoStore()
	var err error
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server, error: %w", err)
	}
	c.gitServer = gs
	return c, nil
}

type hFDatasetComponentImpl struct {
	repoComponent RepoComponent
	tagStore      database.TagStore
	datasetStore  database.DatasetStore
	repoStore     database.RepoStore
	gitServer     gitserver.GitServer
}

func convertFilePathFromRoute(path string) string {
	return strings.TrimLeft(path, "/")
}

func (h *hFDatasetComponentImpl) GetPathsInfo(ctx context.Context, req types.PathReq) ([]types.HFDSPathInfo, error) {
	ds, err := h.datasetStore.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	allow, err := h.repoComponent.AllowReadAccessRepo(ctx, ds.Repository, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check dataset permission, error: %w", err)
	}
	if !allow {
		return nil, errorx.ErrUnauthorized
	}

	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      convertFilePathFromRoute(req.Path),
		RepoType:  types.DatasetRepo,
	}
	file, _ := h.gitServer.GetRepoFileContents(ctx, getRepoFileTree)
	if file == nil {
		return []types.HFDSPathInfo{}, nil
	}
	slog.Debug("get file info", slog.Any("req", req), slog.Any("file", file))

	paths := []types.HFDSPathInfo{
		{
			Type: "file",
			Path: file.Path,
			Size: file.Size,
			OID:  file.SHA,
		},
	}

	return paths, nil
}

func (h *hFDatasetComponentImpl) GetDatasetTree(ctx context.Context, req types.PathReq) ([]types.HFDSPathInfo, error) {
	ds, err := h.datasetStore.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset tree, error: %w", err)
	}

	allow, err := h.repoComponent.AllowReadAccessRepo(ctx, ds.Repository, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check dataset permission, error: %w", err)
	}
	if !allow {
		return nil, errorx.ErrUnauthorized
	}

	if req.Recursive {
		return h.getRecursiveDatasetTree(ctx, req)
	}

	var treeFiles []types.HFDSPathInfo

	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Path:      req.Path,
		RepoType:  types.DatasetRepo,
		Ref:       req.Ref,
	}
	tree, err := h.gitServer.GetRepoFileTree(ctx, getRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset repo file tree, error: %w", err)
	}

	for _, item := range tree {
		treeFiles = append(treeFiles, types.HFDSPathInfo{Type: item.Type, OID: item.SHA, Size: item.Size, Path: item.Path})
	}
	return treeFiles, nil
}


// getRecursiveDatasetTree returns all files in the dataset repo recursively using cursor-based pagination.
func (h *hFDatasetComponentImpl) getRecursiveDatasetTree(ctx context.Context, req types.PathReq) ([]types.HFDSPathInfo, error) {
	var treeFiles []types.HFDSPathInfo
	var cursor string
	for {
		resp, err := h.gitServer.GetTree(ctx, types.GetTreeRequest{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  types.DatasetRepo,
			Recursive: true,
			Limit:     types.MaxFileTreeSize,
			Cursor:    cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get recursive dataset tree, error: %w", err)
		}
		if resp == nil {
			break
		}
		for _, item := range resp.Files {
			if item.Type == "dir" {
				continue
			}
			treeFiles = append(treeFiles, types.HFDSPathInfo{
				Type: item.Type,
				OID:  item.SHA,
				Size: item.Size,
				Path: item.Path,
			})
		}
		cursor = resp.Cursor
		if cursor == "" {
			break
		}
	}
	return treeFiles, nil
}
