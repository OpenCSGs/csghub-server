package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewHFDatasetComponent(config *config.Config) (*HFDatasetComponent, error) {
	c := &HFDatasetComponent{}
	c.ts = database.NewTagStore()
	c.ds = database.NewDatasetStore()
	c.rs = database.NewRepoStore()
	var err error
	c.RepoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

type HFDatasetComponent struct {
	*RepoComponent
	ts *database.TagStore
	ds *database.DatasetStore
	rs *database.RepoStore
}

func (h *HFDatasetComponent) GetDatasetMeta(ctx context.Context, req types.HFDatasetReq) (*types.HFDatasetMeta, error) {
	ds, err := h.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	allow, err := h.AllowReadAccessRepo(ctx, ds.Repository, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check dataset permission, error: %w", err)
	}
	if !allow {
		return nil, ErrUnauthorized
	}

	var tags []string
	for _, tag := range ds.Repository.Tags {
		tags = append(tags, tag.Name)
	}

	filePaths, err := getFilePaths(req.Namespace, req.Name, "", types.DatasetRepo, h.git.GetRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get all %s %s/%s files, error: %w", types.DatasetRepo, req.Namespace, req.Name, err)
	}

	var sdkFiles []types.SDKFile
	for _, filePath := range filePaths {
		sdkFiles = append(sdkFiles, types.SDKFile{Filename: filePath})
	}

	lastCommit, err := h.LastCommit(ctx, &types.GetCommitsReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		RepoType:  types.DatasetRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest commit, error: %w", err)
	}
	meta := types.HFDatasetMeta{
		ID:           ds.Repository.Path,
		Author:       ds.Repository.User.Username,
		Sha:          lastCommit.ID,
		Private:      ds.Repository.Private,
		Disabled:     false,
		Gated:        nil,
		Downloads:    int(ds.Repository.DownloadCount),
		Likes:        int(ds.Repository.Likes),
		Tags:         tags,
		Siblings:     sdkFiles,
		CreatedAt:    ds.Repository.CreatedAt,
		LastModified: ds.Repository.UpdatedAt,
	}

	return &meta, nil
}

func convertFilePathFromRoute(path string) string {
	return strings.TrimLeft(path, "/")
}

func (h *HFDatasetComponent) GetPathsInfo(ctx context.Context, req types.PathReq) ([]types.HFDSPathInfo, error) {
	ds, err := h.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	allow, err := h.AllowReadAccessRepo(ctx, ds.Repository, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check dataset permission, error: %w", err)
	}
	if !allow {
		return nil, ErrUnauthorized
	}

	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      convertFilePathFromRoute(req.Path),
		RepoType:  types.DatasetRepo,
	}
	file, _ := h.git.GetRepoFileContents(ctx, getRepoFileTree)
	if file == nil {
		return []types.HFDSPathInfo{}, nil
	}
	slog.Debug("get file info", slog.Any("req", req), slog.Any("file", file))

	paths := []types.HFDSPathInfo{
		{
			Type: "file",
			Path: file.Path,
			Size: file.Size,
			OID:  file.LastCommitSHA,
		},
	}

	return paths, nil
}

func (h *HFDatasetComponent) GetDatasetTree(ctx context.Context, req types.PathReq) ([]types.HFDSPathInfo, error) {
	ds, err := h.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset tree, error: %w", err)
	}

	allow, err := h.AllowReadAccessRepo(ctx, ds.Repository, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check dataset permission, error: %w", err)
	}
	if !allow {
		return nil, ErrUnauthorized
	}

	var treeFiles []types.HFDSPathInfo

	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Path:      req.Path,
		RepoType:  types.DatasetRepo,
	}
	tree, err := h.git.GetRepoFileTree(ctx, getRepoFileTree)
	if err != nil {
		slog.Warn("failed to get repo file tree", slog.Any("getRepoFileTree", getRepoFileTree), slog.String("error", err.Error()))
		return []types.HFDSPathInfo{}, nil
	}
	slog.Debug("get tree", slog.Any("tree", tree))

	for _, item := range tree {
		treeFiles = append(treeFiles, types.HFDSPathInfo{Type: item.Type, OID: item.LastCommitSHA, Size: item.Size, Path: item.Path})
	}
	return treeFiles, nil
}
