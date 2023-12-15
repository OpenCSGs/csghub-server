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

func NewUserComponent(config *config.Config) (*UserComponent, error) {
	c := &UserComponent{}
	c.ms = database.NewModelStore()
	c.us = database.NewUserStore()
	c.ds = database.NewDatasetStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type UserComponent struct {
	us *database.UserStore
	ms *database.ModelStore
	ds *database.DatasetStore
	gs gitserver.GitServer
}

func (c *UserComponent) Create(ctx context.Context, req *types.CreateUserRequest) (*database.User, error) {
	userExists, err := c.us.IsExist(ctx, req.Username)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	if userExists {
		return nil, errors.New("user already exists")
	}
	user, err := c.gs.CreateUser(req)
	if err != nil {
		newError := fmt.Errorf("failed to create gitserver user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	namespace := &database.Namespace{
		Path: user.Username,
	}
	err = c.us.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	return user, nil
}

func (c *UserComponent) Update(ctx context.Context, req *types.UpdateUserRequest) (*database.User, error) {
	userExists, err := c.us.IsExist(ctx, req.Username)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	if !userExists {
		return nil, errors.New("user does not exists")
	}

	user, err := c.gs.UpdateUser(req)
	if err != nil {
		newError := fmt.Errorf("failed to update user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	return user, nil
}

func (c *UserComponent) Datasets(ctx context.Context, req *types.UserDatasetsReq) ([]database.Dataset, int, error) {
	userExists, err := c.us.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	cuserExists, err := c.us.IsExist(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !cuserExists {
		return nil, 0, errors.New("current user not exists")
	}

	onlyPublic := req.Owner != req.CurrentUser
	ds, total, err := c.ds.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	return ds, total, nil
}

func (c *UserComponent) Models(ctx context.Context, req *types.UserModelsReq) ([]database.Model, int, error) {
	userExists, err := c.us.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	cuserExists, err := c.us.IsExist(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !cuserExists {
		return nil, 0, errors.New("current user not exists")
	}

	op := req.Owner != req.CurrentUser
	ms, total, err := c.ms.ByUsername(ctx, req.Owner, req.PageSize, req.Page, op)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	return ms, total, nil
}
