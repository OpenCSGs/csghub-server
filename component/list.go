package component

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewListComponent(config *config.Config) (*ListComponent, error) {
	c := &ListComponent{}
	c.ds = database.NewDatasetStore()
	c.ms = database.NewModelStore()
	c.ss = database.NewSpaceStore()
	return c, nil
}

type ListComponent struct {
	ms *database.ModelStore
	ds *database.DatasetStore
	ss *database.SpaceStore
}

func (c *ListComponent) ListModelsByPath(ctx context.Context, req *types.ListByPathReq) ([]*types.ModelResp, error) {
	var modelResp []*types.ModelResp

	models, err := c.ms.ListByPath(ctx, req.Paths)
	if err != nil {
		slog.Error("error listing models by path: %v", err, slog.Any("paths", req.Paths))
		return nil, err
	}
	for _, model := range models {
		var tags []types.RepoTag
		for _, tag := range model.Repository.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.ShowName,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		modelResp = append(modelResp, &types.ModelResp{
			Name:        model.Repository.Name,
			Path:        model.Repository.Path,
			Downloads:   model.Repository.DownloadCount,
			UpdatedAt:   model.UpdatedAt,
			Private:     model.Repository.Private,
			Nickname:    model.Repository.Nickname,
			Description: model.Repository.Description,
			Tags:        tags,
		})
	}

	return modelResp, nil
}

func (c *ListComponent) ListDatasetsByPath(ctx context.Context, req *types.ListByPathReq) ([]*types.DatasetResp, error) {
	var datasetResp []*types.DatasetResp

	datasets, err := c.ds.ListByPath(ctx, req.Paths)
	if err != nil {
		slog.Error("error listing datasets by path: %v", err, slog.Any("paths", req.Paths))
		return nil, err
	}
	for _, dataset := range datasets {
		var tags []types.RepoTag
		for _, tag := range dataset.Repository.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.ShowName,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		datasetResp = append(datasetResp, &types.ModelResp{
			Name:        dataset.Repository.Name,
			Path:        dataset.Repository.Path,
			Downloads:   dataset.Repository.DownloadCount,
			UpdatedAt:   dataset.UpdatedAt,
			Private:     dataset.Repository.Private,
			Nickname:    dataset.Repository.Nickname,
			Description: dataset.Repository.Description,
			Tags:        tags,
		})
	}
	return datasetResp, nil
}
