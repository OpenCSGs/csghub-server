package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
)

func NewDatasetComponent(config *config.Config) (*DatasetComponent, error) {
	c := &DatasetComponent{}
	c.ns = database.NewNamespaceStore()
	c.us = database.NewUserStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type DatasetComponent struct {
	// ds *database.DatasetStore
	ns *database.NamespaceStore
	us *database.UserStore
	gs gitserver.GitServer
}

func (c *DatasetComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) error {
	var err error
	_, err = c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return errors.New("Namespace does not exist")
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return errors.New("User does not exist")
	}
	err = c.gs.CreateDatasetFile(req)
	return err
}
