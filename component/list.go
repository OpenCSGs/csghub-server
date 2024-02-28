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
	return c, nil
}

type ListComponent struct {
	ms *database.ModelStore
	ds *database.DatasetStore
}

func (c *ListComponent) ListModelsByPath(ctx context.Context, req *types.ListByPathReq) ([]*types.ModelResp, error) {
	var modelResp []*types.ModelResp

	models, err := c.ms.ListByPath(ctx, req.Paths)
	if err != nil {
		slog.Error("error listing models by path: %v", err, slog.Any("paths", req.Paths))
		return nil, err
	}
	for _, model := range models {
		modelResp = append(modelResp, &types.ModelResp{
			Path:      model.Repository.Path,
			Downloads: model.Repository.DownloadCount,
			UpdatedAt: model.UpdatedAt,
			Private:   model.Repository.Private,
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
		datasetResp = append(datasetResp, &types.ModelResp{
			Path:      dataset.Repository.Path,
			Downloads: dataset.Repository.DownloadCount,
			UpdatedAt: dataset.UpdatedAt,
			Private:   dataset.Repository.Private,
		})
	}
	return datasetResp, nil
}
