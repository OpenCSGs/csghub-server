package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func NewSSHKeyComponent(config *config.Config) (*SSHKeyComponent, error) {
	c := &SSHKeyComponent{}
	c.ss = database.NewSSHKeyStore()
	c.us = database.NewUserStore()
	var err error
	c.gs, err = git.NewGitServer(config)
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
	nameExistsKey, err := c.ss.FindByNameAndUserID(ctx, req.Name, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to find if ssh key exists,error:%w", err)
	}
	if nameExistsKey.ID != 0 {
		return nil, fmt.Errorf("ssh key name already exists")
	}

	contentExistsKey, err := c.ss.FindByKeyContent(ctx, req.Content)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to find if ssh key exists,error:%w", err)
	}
	if contentExistsKey.ID != 0 {
		return nil, fmt.Errorf("ssh key already exists")
	}

	sk, err := c.gs.CreateSSHKey(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create git SSH key,error:%w", err)
	}
	fingerprint, err := common.CalculateSSHKeyFingerprint(req.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate ssh key fingerprint,error:%w", err)
	}
	if sk == nil {
		sk = &database.SSHKey{
			GitID:   0,
			Name:    req.Name,
			Content: req.Content,
			UserID:  user.ID,
		}
	}
	sk.UserID = user.ID
	sk.FingerprintSHA256 = fingerprint
	resSk, err := c.ss.Create(ctx, sk)
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

func (c *SSHKeyComponent) Delete(ctx context.Context, username, name string) error {
	sshKey, err := c.ss.FindByUsernameAndName(ctx, username, name)
	if err != nil {
		return fmt.Errorf("failed to get database SSH keys,error:%w", err)
	}
	err = c.gs.DeleteSSHKey(int(sshKey.GitID))
	if err != nil {
		return fmt.Errorf("failed to delete git SSH keys,error:%w", err)
	}
	err = c.ss.Delete(ctx, sshKey.GitID)
	if err != nil {
		return fmt.Errorf("failed to delete database SSH keys,error:%w", err)
	}
	return nil
}
