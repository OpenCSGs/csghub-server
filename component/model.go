package component

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
)

func NewModelComponent(config *config.Config) (*ModelComponent, error) {
	c := &ModelComponent{}
	c.us = database.NewUserStore()
	c.ms = database.NewModelStore()
	c.os = database.NewOrgStore()
	c.ns = database.NewNamespaceStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type ModelComponent struct {
	us *database.UserStore
	ms *database.ModelStore
	os *database.OrgStore
	ns *database.NamespaceStore
	gs gitserver.GitServer
}

func (c *ModelComponent) Index(ctx context.Context, search, sort, tag string, per, page int) ([]database.Model, int, error) {
	models, total, err := c.ms.Public(ctx, search, sort, tag, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public models,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}
	return models, total, nil
}

func (c *ModelComponent) Create(ctx context.Context, req *types.CreateModelReq) (*database.Model, error) {
	_, err := c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	model, repo, err := c.gs.CreateModelRepo(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create git model repository, error: %w", err)
	}

	model, err = c.ms.Create(ctx, model, repo, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create database model, error: %w", err)
	}

	err = c.gs.CreateModelFile(createModelGitattributesReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	err = c.gs.CreateModelFile(createModelReadmeReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	return model, nil
}

func createModelGitattributesReq(r *types.CreateModelReq, user database.User) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    r.DefaultBranch,
		Content:   base64.StdEncoding.EncodeToString([]byte(datasetGitattributesContent)),
		NewBranch: r.DefaultBranch,
		NameSpace: r.Namespace,
		Name:      r.Name,
		FilePath:  ".gitattributes",
	}
}

func createModelReadmeReq(r *types.CreateModelReq, user database.User) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    r.DefaultBranch,
		Content:   base64.StdEncoding.EncodeToString([]byte(generateReadmeData(r.License))),
		NewBranch: r.DefaultBranch,
		NameSpace: r.Namespace,
		Name:      r.Name,
		FilePath:  "README.md",
	}
}

func (c *ModelComponent) Update(ctx context.Context, req *types.UpdateModelReq) (*database.Model, error) {
	_, err := c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	model, err := c.ms.FindyByPath(ctx, req.Namespace, req.OriginName)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	err = c.gs.UpdateModelRepo(req.Namespace, req.OriginName, model, model.Repository, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update git model repository, error: %w", err)
	}

	err = c.ms.Update(ctx, model, model.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to update database model, error: %w", err)
	}

	return model, nil
}

func (c *ModelComponent) Delete(ctx context.Context, namespace, name string) error {
	_, err := c.ms.FindyByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}
	err = c.gs.DeleteModelRepo(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete git model repository, error: %w", err)
	}

	err = c.ms.Delete(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete database model, error: %w", err)
	}
	return nil
}

func (c *ModelComponent) Detail(ctx context.Context, namespace, name string) (*types.ModelDetail, error) {
	_, err := c.ms.FindyByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	detail, err := c.gs.GetModelDetail(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model detail, error: %w", err)
	}

	return detail, nil
}

func (c *ModelComponent) Show(ctx context.Context, namespace, name, current_user string) (*database.Model, error) {
	model, err := c.ms.FindyByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	if model.Private {
		if model.User.Username != current_user {
			return nil, fmt.Errorf("failed to find model, error: %w", errors.New("the private model is not accessible to the current user"))
		}
	}

	return model, nil
}

func (c *ModelComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) error {
	_, err := c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return fmt.Errorf("failed to find username, error: %w", err)
	}
	err = c.gs.CreateModelFile(req)
	if err != nil {
		return fmt.Errorf("failed to create model file, error: %w", err)
	}

	return nil
}

func (c *ModelComponent) UpdateFile(ctx context.Context, req *types.UpdateFileReq) error {
	_, err := c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return fmt.Errorf("failed to find username, error: %w", err)
	}
	err = c.gs.UpdateModelFile(req.NameSpace, req.Name, req.FilePath, req)
	if err != nil {
		return fmt.Errorf("failed to create model file, error: %w", err)
	}

	return nil
}

func (c *ModelComponent) Commits(ctx context.Context, req *types.GetCommitsReq) ([]*types.Commit, error) {
	model, err := c.ms.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	commits, err := c.gs.GetModelCommits(req.Namespace, req.Name, req.Ref, req.Per, req.Page)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository commits, error: %w", err)
	}
	return commits, nil
}

func (c *ModelComponent) LastCommit(ctx context.Context, req *types.GetCommitsReq) (*types.Commit, error) {
	model, err := c.ms.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	commit, err := c.gs.GetModelLastCommit(req.Namespace, req.Name, req.Ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository last commit, error: %w", err)
	}
	return commit, nil
}

func (c *ModelComponent) FileRaw(ctx context.Context, req *types.GetFileReq) (string, error) {
	model, err := c.ms.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return "", fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	raw, err := c.gs.GetModelFileRaw(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get git model repository file raw, error: %w", err)
	}
	return raw, nil
}

func (c *ModelComponent) Branches(ctx context.Context, req *types.GetBranchesReq) ([]*types.ModelBranch, error) {
	_, err := c.ms.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	bs, err := c.gs.GetModelBranches(req.Namespace, req.Name, req.Per, req.Page)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository branches, error: %w", err)
	}
	return bs, nil
}

func (c *ModelComponent) Tags(ctx context.Context, req *types.GetTagsReq) ([]database.Tag, error) {
	_, err := c.ms.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	tags, err := c.ms.Tags(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get model tags, error: %w", err)
	}
	return tags, nil
}

func (c *ModelComponent) Tree(ctx context.Context, req *types.GetFileReq) ([]*types.File, error) {
	model, err := c.ms.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	tree, err := c.gs.GetModelFileTree(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository file tree, error: %w", err)
	}
	return tree, nil
}
