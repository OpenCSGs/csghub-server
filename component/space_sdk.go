package component

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SpaceSdkComponent interface {
	Index(ctx context.Context) ([]types.SpaceSdk, error)
	Update(ctx context.Context, req *types.UpdateSpaceSdkReq) (*types.SpaceSdk, error)
	Create(ctx context.Context, req *types.CreateSpaceSdkReq) (*types.SpaceSdk, error)
	Delete(ctx context.Context, id int64) error
}

func NewSpaceSdkComponent(config *config.Config) (SpaceSdkComponent, error) {
	c := &spaceSdkComponentImpl{}
	c.spaceSdkStore = database.NewSpaceSdkStore()

	return c, nil
}

type spaceSdkComponentImpl struct {
	spaceSdkStore database.SpaceSdkStore
}

func (c *spaceSdkComponentImpl) Index(ctx context.Context) ([]types.SpaceSdk, error) {
	var result []types.SpaceSdk
	databaseSpaceSdks, err := c.spaceSdkStore.Index(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range databaseSpaceSdks {
		result = append(result, types.SpaceSdk{
			ID:      r.ID,
			Name:    r.Name,
			Version: r.Version,
		})
	}

	return result, nil
}

func (c *spaceSdkComponentImpl) Update(ctx context.Context, req *types.UpdateSpaceSdkReq) (*types.SpaceSdk, error) {
	ss, err := c.spaceSdkStore.FindByID(ctx, req.ID)
	if err != nil {
		slog.Error("error getting space sdk", slog.Any("error", err))
		return nil, err
	}
	ss.Name = req.Name
	ss.Version = req.Version

	ss, err = c.spaceSdkStore.Update(ctx, *ss)
	if err != nil {
		slog.Error("error getting space sdk", slog.Any("error", err))
		return nil, err
	}

	result := &types.SpaceSdk{
		ID:      ss.ID,
		Name:    ss.Name,
		Version: ss.Version,
	}

	return result, nil
}

func (c *spaceSdkComponentImpl) Create(ctx context.Context, req *types.CreateSpaceSdkReq) (*types.SpaceSdk, error) {
	ss := database.SpaceSdk{
		Name:    req.Name,
		Version: req.Version,
	}
	res, err := c.spaceSdkStore.Create(ctx, ss)
	if err != nil {
		slog.Error("error creating space sdk", slog.Any("error", err))
		return nil, err
	}

	result := &types.SpaceSdk{
		ID:      res.ID,
		Name:    res.Name,
		Version: res.Version,
	}

	return result, nil
}

func (c *spaceSdkComponentImpl) Delete(ctx context.Context, id int64) error {
	ss, err := c.spaceSdkStore.FindByID(ctx, id)
	if err != nil {
		slog.Error("error finding space sdk", slog.Any("error", err))
		return err
	}

	err = c.spaceSdkStore.Delete(ctx, *ss)
	if err != nil {
		slog.Error("error deleting space sdk", slog.Any("error", err))
		return err
	}
	return nil
}
