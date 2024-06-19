package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror_proxy/types"
)

type MirrorTokenComponent struct {
	mtStore *database.MirrorTokenStore
}

func NewMirrorTokenComponent(config *config.Config) (*MirrorTokenComponent, error) {
	return &MirrorTokenComponent{
		mtStore: database.NewMirrorTokenStore(),
	}, nil
}

func (c *MirrorTokenComponent) Create(ctx context.Context, req types.CreateMirrorTokenReq) (*database.MirrorToken, error) {
	exists, err := c.mtStore.MirrorTokenExists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check mirror token if exists, error: %w", err)
	}
	if exists {
		err := c.mtStore.DeleteAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing mirror token, error: %w", err)
		}
	}
	var mt database.MirrorToken
	mt.Token = req.Token
	mt.ConcurrentCount = req.ConcurrentCount
	mt.MaxBandwidth = req.MaxBandwidth
	res, err := c.mtStore.Create(ctx, &mt)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror token, error: %w", err)
	}
	return res, nil
}
