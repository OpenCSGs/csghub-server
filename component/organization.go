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

func NewOrganizationComponent(config *config.Config) (*OrganizationComponent, error) {
	c := &OrganizationComponent{}
	c.os = database.NewOrgStore()
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

type OrganizationComponent struct {
	os *database.OrgStore
	ns *database.NamespaceStore
	us *database.UserStore
	gs gitserver.GitServer
}

func (c *OrganizationComponent) Create(ctx context.Context, req *types.CreateOrgReq) (*database.Organization, error) {
	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	es, err := c.ns.Exists(ctx, req.Name)
	if es {
		return nil, fmt.Errorf("the name already exists, error: %w", err)
	}

	req.User = user
	org, err := c.gs.CreateOrganization(req)
	if err != nil {
		return nil, fmt.Errorf("failed create git organization, error: %w", err)
	}
	namespace := &database.Namespace{
		Path:   org.Path,
		UserID: user.ID,
	}
	err = c.os.Create(ctx, org, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed create database organization, error: %w", err)
	}
	return org, nil
}

func (c *OrganizationComponent) Index(ctx context.Context, username string) ([]database.Organization, error) {
	orgs, err := c.os.Index(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get organizations, error: %w", err)
	}
	return orgs, nil
}

func (c *OrganizationComponent) Delete(ctx context.Context, name string) error {
	err := c.gs.DeleteOrganization(name)
	if err != nil {
		return fmt.Errorf("failed to delete git organizations, error: %w", err)
	}
	err = c.os.Delete(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to delete database organizations, error: %w", err)
	}
	return nil
}

func (c *OrganizationComponent) Update(ctx context.Context, req *types.EditOrgReq) (*database.Organization, error) {
	org, err := c.os.FindByPath(ctx, req.Path)
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
