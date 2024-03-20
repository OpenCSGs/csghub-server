package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewOrganizationComponent(config *config.Config) (*OrganizationComponent, error) {
	c := &OrganizationComponent{}
	c.os = database.NewOrgStore()
	c.ns = database.NewNamespaceStore()
	c.us = database.NewUserStore()
	c.ds = database.NewDatasetStore()
	c.ms = database.NewModelStore()
	c.cs = database.NewCodeStore()
	c.ss = database.NewSpaceStore()
	var err error
	c.gs, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.msc, err = NewMemberComponent(config)
	if err != nil {
		newError := fmt.Errorf("fail to create membership component,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type OrganizationComponent struct {
	os *database.OrgStore
	ns *database.NamespaceStore
	us *database.UserStore
	ds *database.DatasetStore
	ms *database.ModelStore
	cs *database.CodeStore
	ss *database.SpaceStore
	gs gitserver.GitServer

	msc *MemberComponent
}

func (c *OrganizationComponent) FixOrgData(ctx context.Context, org *database.Organization) (*database.Organization, error) {
	user := org.User
	req := new(types.CreateOrgReq)
	req.Name = org.Name
	req.FullName = org.FullName
	req.Username = org.User.Username
	req.Description = org.Description
	err := c.gs.FixOrganization(req, *user)
	if err != nil {
		slog.Error("fix git org data has error", slog.Any("error", err))
	}
	// need to create roles for a new org before adding members
	err = c.msc.InitRoles(ctx, org)
	if err != nil {
		slog.Error("fix organization role has error", slog.String("error", err.Error()))
	}
	// org creator defaults to be admin role
	err = c.msc.SetAdmin(ctx, org, user)
	return org, err
}

func (c *OrganizationComponent) Create(ctx context.Context, req *types.CreateOrgReq) (*database.Organization, error) {
	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	es, err := c.ns.Exists(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	if es {
		return nil, errors.New("the name already exists")
	}

	org, err := c.gs.CreateOrganization(req, user)
	if err != nil {
		return nil, fmt.Errorf("failed create git organization, error: %w", err)
	}
	namespace := &database.Namespace{
		Path:   org.Name,
		UserID: user.ID,
	}
	err = c.os.Create(ctx, org, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed create database organization, error: %w", err)
	}
	// need to create roles for a new org before adding members
	err = c.msc.InitRoles(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed init roles for organization, error: %w", err)
	}
	// org creator defaults to be admin role
	err = c.msc.SetAdmin(ctx, org, &user)
	return org, err
}

func (c *OrganizationComponent) Index(ctx context.Context, username string) ([]database.Organization, error) {
	orgs, err := c.os.Index(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get organizations, error: %w", err)
	}
	return orgs, nil
}

func (c *OrganizationComponent) Delete(ctx context.Context, req *types.DeleteOrgReq) error {
	r, err := c.msc.GetMemberRole(ctx, req.Name, req.CurrentUser)
	if err != nil {
		slog.Error("faild to get member role",
			slog.String("org", req.Name), slog.String("user", req.CurrentUser),
			slog.String("error", err.Error()))
	}
	if !r.CanAdmin() {
		return fmt.Errorf("current user does not have permission to edit the organization, current user: %s", req.CurrentUser)
	}
	err = c.gs.DeleteOrganization(req.Name)
	if err != nil {
		return fmt.Errorf("failed to delete git organizations, error: %w", err)
	}
	err = c.os.Delete(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("failed to delete database organizations, error: %w", err)
	}
	return nil
}

func (c *OrganizationComponent) Update(ctx context.Context, req *types.EditOrgReq) (*database.Organization, error) {
	r, err := c.msc.GetMemberRole(ctx, req.Name, req.CurrentUser)
	if err != nil {
		slog.Error("faild to get member role",
			slog.String("org", req.Name), slog.String("user", req.CurrentUser),
			slog.String("error", err.Error()))
	}
	if !r.CanAdmin() {
		return nil, fmt.Errorf("current user does not have permission to edit the organization, current user: %s", req.CurrentUser)
	}
	org, err := c.os.FindByPath(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("organization does not exists, error: %w", err)
	}
	nOrg, err := c.gs.UpdateOrganization(req, &org)
	if err != nil {
		return nil, fmt.Errorf("failed to update git organization, error: %w", err)
	}
	err = c.os.Update(ctx, nOrg)
	if err != nil {
		return nil, fmt.Errorf("failed to update database organization, error: %w", err)
	}
	return nOrg, err
}

func (c *OrganizationComponent) Models(ctx context.Context, req *types.OrgModelsReq) ([]types.Model, int, error) {
	var resModels []types.Model
	r, err := c.msc.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
	// log error, and treat user as unkown role in org
	if err != nil {
		slog.Error("faild to get member role",
			slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
			slog.String("error", err.Error()))
	}
	onlyPublic := !r.CanRead()
	ms, total, err := c.ms.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range ms {
		resModels = append(resModels, types.Model{
			ID:           data.ID,
			Name:         data.Repository.Name,
			Nickname:     data.Repository.Nickname,
			Description:  data.Repository.Description,
			Likes:        data.Repository.Likes,
			Downloads:    data.Repository.DownloadCount,
			Path:         data.Repository.Path,
			RepositoryID: data.RepositoryID,
			Private:      data.Repository.Private,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.UpdatedAt,
		})
	}

	return resModels, total, nil
}

func (c *OrganizationComponent) Datasets(ctx context.Context, req *types.OrgDatasetsReq) ([]types.Dataset, int, error) {
	var resDatasets []types.Dataset
	r, err := c.msc.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
	// log error, and treat user as unkown role in org
	if err != nil {
		slog.Error("faild to get member role",
			slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
			slog.String("error", err.Error()))
	}
	onlyPublic := !r.CanRead()
	datasets, total, err := c.ds.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range datasets {
		resDatasets = append(resDatasets, types.Dataset{
			ID:           data.ID,
			Name:         data.Repository.Name,
			Nickname:     data.Repository.Nickname,
			Description:  data.Repository.Description,
			Likes:        data.Repository.Likes,
			Downloads:    data.Repository.DownloadCount,
			Path:         data.Repository.Path,
			RepositoryID: data.RepositoryID,
			Private:      data.Repository.Private,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.UpdatedAt,
		})
	}

	return resDatasets, total, nil
}

func (c *OrganizationComponent) Codes(ctx context.Context, req *types.OrgCodesReq) ([]types.Code, int, error) {
	var resCodes []types.Code
	r, err := c.msc.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
	// log error, and treat user as unkown role in org
	if err != nil {
		slog.Error("faild to get member role",
			slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
			slog.String("error", err.Error()))
	}
	onlyPublic := !r.CanRead()
	codes, total, err := c.cs.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get org codes,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range codes {
		resCodes = append(resCodes, types.Code{
			ID:           data.ID,
			Name:         data.Repository.Name,
			Nickname:     data.Repository.Nickname,
			Description:  data.Repository.Description,
			Likes:        data.Repository.Likes,
			Downloads:    data.Repository.DownloadCount,
			Path:         data.Repository.Path,
			RepositoryID: data.RepositoryID,
			Private:      data.Repository.Private,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.UpdatedAt,
		})
	}

	return resCodes, total, nil
}

func (c *OrganizationComponent) Spaces(ctx context.Context, req *types.OrgSpacesReq) ([]types.Space, int, error) {
	var resSpaces []types.Space
	r, err := c.msc.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
	// log error, and treat user as unkown role in org
	if err != nil {
		slog.Error("faild to get member role",
			slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
			slog.String("error", err.Error()))
	}
	onlyPublic := !r.CanRead()
	spaces, total, err := c.ss.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get org spaces,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range spaces {
		resSpaces = append(resSpaces, types.Space{
			ID:          data.ID,
			Name:        data.Repository.Name,
			Nickname:    data.Repository.Nickname,
			Description: data.Repository.Description,
			Likes:       data.Repository.Likes,
			Path:        data.Repository.Path,
			Private:     data.Repository.Private,
			CreatedAt:   data.CreatedAt,
			UpdatedAt:   data.UpdatedAt,
		})
	}

	return resSpaces, total, nil
}
