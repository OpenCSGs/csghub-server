package component

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceResourceComponent(config *config.Config) (*SpaceResourceComponent, error) {
	c := &SpaceResourceComponent{}
	c.srs = database.NewSpaceResourceStore()

	return c, nil
}

type SpaceResourceComponent struct {
	srs *database.SpaceResourceStore
}

func (c *SpaceResourceComponent) Index(ctx context.Context) ([]types.SpaceResource, error) {
	var result []types.SpaceResource
	databaseSpaceResources, err := c.srs.Index(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range databaseSpaceResources {
		result = append(result, types.SpaceResource{
			ID:   r.ID,
			Name: r.Name,
		})
	}

	return result, nil
}

func (c *SpaceResourceComponent) Update(ctx context.Context, req *types.UpdateSpaceResourceReq) (*types.SpaceResource, error) {
	sr, err := c.srs.FindByID(ctx, req.ID)
	if err != nil {
		slog.Error("error getting space resource", slog.Any("error", err))
		return nil, err
	}
	sr.Name = req.Name

	sr, err = c.srs.Update(ctx, *sr)
	if err != nil {
		slog.Error("error updating space resource", slog.Any("error", err))
		return nil, err
	}

	result := &types.SpaceResource{
		ID:   sr.ID,
		Name: sr.Name,
	}

	return result, nil
}

func (c *SpaceResourceComponent) Create(ctx context.Context, req *types.CreateSpaceResourceReq) (*types.SpaceResource, error) {
	sr := database.SpaceResource{
		Name: req.Name,
	}
	res, err := c.srs.Create(ctx, sr)
	if err != nil {
		slog.Error("error creating space resource", slog.Any("error", err))
		return nil, err
	}

	result := &types.SpaceResource{
		ID:   res.ID,
		Name: res.Name,
	}

	return result, nil
}

func (c *SpaceResourceComponent) Delete(ctx context.Context, id int64) error {
	sr, err := c.srs.FindByID(ctx, id)
	if err != nil {
		slog.Error("error finding space resource", slog.Any("error", err))
		return err
	}

	err = c.srs.Delete(ctx, *sr)
	if err != nil {
		slog.Error("error deleting space resource", slog.Any("error", err))
		return err
	}
	return nil
}
