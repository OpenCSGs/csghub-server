package component

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
)

func NewSSHKeyComponent(config *config.Config) (*SSHKeyComponent, error) {
	c := &SSHKeyComponent{}
	c.ss = database.NewSSHKeyStore()
	c.us = database.NewUserStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type SSHKeyComponent struct {
	ss *database.SSHKeyStore
	us *database.UserStore
	gs gitserver.GitServer
}

func (c *SSHKeyComponent) Create(ctx context.Context, req *types.CreateSSHKeyRequest) (*database.SSHKey, error) {
	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user,error:%w", err)
	}
	sk, err := c.gs.CreateSSHKey(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create git SSH key,error:%w", err)
	}
	resSk, err := c.ss.Create(ctx, sk, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create database SSH key,error:%w", err)
	}
	return resSk, nil
}

func (c *SSHKeyComponent) Index(ctx context.Context, username string, per, page int) ([]database.SSHKey, error) {
	sks, err := c.ss.Index(ctx, username, per, page)
	if err != nil {
		return nil, fmt.Errorf("failed to get database SSH keys,error:%w", err)
	}
	return sks, nil
}

func (c *SSHKeyComponent) Delete(ctx context.Context, gid int64) error {
	err := c.gs.DeleteSSHKey(int(gid))
	if err != nil {
		return fmt.Errorf("failed to delete git SSH keys,error:%w", err)
	}
	err = c.ss.Delete(ctx, gid)
	if err != nil {
		return fmt.Errorf("failed to delete database SSH keys,error:%w", err)
	}
	return nil
}
