package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type namespaceComponentImpl struct {
	ns database.NamespaceStore
	os database.OrgStore
}

type NamespaceComponent interface {
	GetInfo(ctx context.Context, path string) (*types.Namespace, error)
	GetInfoByUUID(ctx context.Context, uuid string) (*types.Namespace, error)
}

func NewNamespaceComponent(config *config.Config) (NamespaceComponent, error) {
	return &namespaceComponentImpl{
		ns: database.NewNamespaceStore(),
		os: database.NewOrgStore(),
	}, nil
}

func (c *namespaceComponentImpl) GetInfo(ctx context.Context, path string) (*types.Namespace, error) {
	dbns, err := c.ns.FindByPath(ctx, path)
	if err != nil {
		return nil, err
	}
	return c.buildNamespace(ctx, &dbns)
}

func (c *namespaceComponentImpl) GetInfoByUUID(ctx context.Context, uuid string) (*types.Namespace, error) {
	dbns, err := c.ns.FindByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}
	return c.buildNamespace(ctx, &dbns)
}

func (c *namespaceComponentImpl) buildNamespace(ctx context.Context, dbns *database.Namespace) (*types.Namespace, error) {
	ns := &types.Namespace{
		Path:   dbns.Path,
		Type:   string(dbns.NamespaceType),
		NSType: string(dbns.NamespaceType),
		UUID:   dbns.UUID,
	}
	switch dbns.NamespaceType {
	case database.UserNamespace:
		ns.Avatar = dbns.User.Avatar
	case database.OrgNamespace:
		org, err := c.os.FindByPath(ctx, dbns.Path)
		if err != nil {
			return nil, fmt.Errorf("fail to get org info, path: %s, error: %w", dbns.Path, err)
		}
		ns.Type = string(org.OrgType)
		ns.Avatar = org.Logo
	default:
		return nil, fmt.Errorf("invalid namespace type: %s", dbns.NamespaceType)
	}
	return ns, nil
}
