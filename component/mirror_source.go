package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MirrorSourceComponent struct {
	msStore   *database.MirrorSourceStore
	userStore *database.UserStore
}

func NewMirrorSourceComponent(config *config.Config) (*MirrorSourceComponent, error) {
	return &MirrorSourceComponent{
		msStore:   database.NewMirrorSourceStore(),
		userStore: database.NewUserStore(),
	}, nil
}

func (c *MirrorSourceComponent) Create(ctx context.Context, req types.CreateMirrorSourceReq) (*database.MirrorSource, error) {
	var ms database.MirrorSource
	ms.SourceName = req.SourceName
	ms.InfoAPIUrl = req.InfoAPiUrl
	res, err := c.msStore.Create(ctx, &ms)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror source, error: %w", err)
	}
	return res, nil
}

func (c *MirrorSourceComponent) Get(ctx context.Context, id int64) (*database.MirrorSource, error) {
	ms, err := c.msStore.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror source, error: %w", err)
	}
	return ms, nil
}

func (c *MirrorSourceComponent) Index(ctx context.Context) ([]database.MirrorSource, error) {
	ms, err := c.msStore.Index(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror source, error: %w", err)
	}
	return ms, nil
}
func (c *MirrorSourceComponent) Update(ctx context.Context, req types.UpdateMirrorSourceReq) (*database.MirrorSource, error) {
	var ms database.MirrorSource
	ms.ID = req.ID
	ms.SourceName = req.SourceName
	ms.InfoAPIUrl = req.InfoAPiUrl
	err := c.msStore.Update(ctx, &ms)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror source, error: %w", err)
	}
	return &ms, nil
}

func (c *MirrorSourceComponent) Delete(ctx context.Context, id int64) error {
	ms, err := c.msStore.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find mirror source, error: %w", err)
	}
	err = c.msStore.Delete(ctx, ms)
	if err != nil {
		return fmt.Errorf("failed to delete mirror source, error: %w", err)
	}
	return nil
}
