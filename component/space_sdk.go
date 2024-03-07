package component

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceSdkComponent(config *config.Config) (*SpaceSdkComponent, error) {
	c := &SpaceSdkComponent{}
	c.sss = database.NewSpaceSdkStore()

	return c, nil
}

type SpaceSdkComponent struct {
	sss *database.SpaceSdkStore
}

func (c *SpaceSdkComponent) Index(ctx context.Context) ([]types.SpaceSdk, error) {
	var result []types.SpaceSdk
	databaseSpaceSdks, err := c.sss.Index(ctx)
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

func (c *SpaceSdkComponent) Update(ctx context.Context, req *types.UpdateSpaceSdkReq) (*types.SpaceSdk, error) {
	ss, err := c.sss.FindByID(ctx, req.ID)
	if err != nil {
		slog.Error("error getting space sdk", slog.Any("error", err))
		return nil, err
	}
	ss.Name = req.Name
	ss.Version = req.Version

	ss, err = c.sss.Update(ctx, *ss)
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

func (c *SpaceSdkComponent) Create(ctx context.Context, req *types.CreateSpaceSdkReq) (*types.SpaceSdk, error) {
	ss := database.SpaceSdk{
		Name:    req.Name,
		Version: req.Version,
	}
	res, err := c.sss.Create(ctx, ss)
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

func (c *SpaceSdkComponent) Delete(ctx context.Context, id int64) error {
	ss, err := c.sss.FindByID(ctx, id)
	if err != nil {
		slog.Error("error finding space sdk", slog.Any("error", err))
		return err
	}

	err = c.sss.Delete(ctx, *ss)
	if err != nil {
		slog.Error("error deleting space sdk", slog.Any("error", err))
		return err
	}
	return nil
}
