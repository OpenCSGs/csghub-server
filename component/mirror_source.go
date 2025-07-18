package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type mirrorSourceComponentImpl struct {
	mirrorSourceStore database.MirrorSourceStore
	userStore         database.UserStore
}

type MirrorSourceComponent interface {
	Create(ctx context.Context, req types.CreateMirrorSourceReq) (*database.MirrorSource, error)
	Get(ctx context.Context, id int64) (*database.MirrorSource, error)
	Index(ctx context.Context) ([]database.MirrorSource, error)
	Update(ctx context.Context, req types.UpdateMirrorSourceReq) (*database.MirrorSource, error)
	Delete(ctx context.Context, id int64) error
}

func NewMirrorSourceComponent(config *config.Config) (MirrorSourceComponent, error) {
	return &mirrorSourceComponentImpl{
		mirrorSourceStore: database.NewMirrorSourceStore(),
		userStore:         database.NewUserStore(),
	}, nil
}

func (c *mirrorSourceComponentImpl) Create(ctx context.Context, req types.CreateMirrorSourceReq) (*database.MirrorSource, error) {
	var ms database.MirrorSource
	ms.SourceName = req.SourceName
	ms.InfoAPIUrl = req.InfoAPiUrl
	res, err := c.mirrorSourceStore.Create(ctx, &ms)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror source, error: %w", err)
	}
	return res, nil
}

func (c *mirrorSourceComponentImpl) Get(ctx context.Context, id int64) (*database.MirrorSource, error) {
	ms, err := c.mirrorSourceStore.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror source, error: %w", err)
	}
	return ms, nil
}

func (c *mirrorSourceComponentImpl) Index(ctx context.Context) ([]database.MirrorSource, error) {
	ms, err := c.mirrorSourceStore.Index(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror source, error: %w", err)
	}
	return ms, nil
}
func (c *mirrorSourceComponentImpl) Update(ctx context.Context, req types.UpdateMirrorSourceReq) (*database.MirrorSource, error) {
	var ms database.MirrorSource
	ms.ID = req.ID
	ms.SourceName = req.SourceName
	ms.InfoAPIUrl = req.InfoAPiUrl
	err := c.mirrorSourceStore.Update(ctx, &ms)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror source, error: %w", err)
	}
	return &ms, nil
}

func (c *mirrorSourceComponentImpl) Delete(ctx context.Context, id int64) error {
	ms, err := c.mirrorSourceStore.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find mirror source, error: %w", err)
	}
	err = c.mirrorSourceStore.Delete(ctx, ms)
	if err != nil {
		return fmt.Errorf("failed to delete mirror source, error: %w", err)
	}
	return nil
}
