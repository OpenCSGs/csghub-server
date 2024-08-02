package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type NamespaceComponent struct {
	ns *database.NamespaceStore
	os *database.OrgStore
}

func NewNamespaceComponent(config *config.Config) (*NamespaceComponent, error) {
	return &NamespaceComponent{
		ns: database.NewNamespaceStore(),
		os: database.NewOrgStore(),
	}, nil
}

func (c *NamespaceComponent) GetInfo(ctx context.Context, path string) (*types.Namespace, error) {
	dbns, err := c.ns.FindByPath(ctx, path)
	ns := &types.Namespace{
		Path: dbns.Path,
		Type: string(dbns.NamespaceType),
	}
	if dbns.NamespaceType == database.UserNamespace {
		ns.Avatar = dbns.User.Avatar
	} else if dbns.NamespaceType == database.OrgNamespace {
		org, err := c.os.FindByPath(ctx, dbns.Path)
		if err != nil {
			return nil, fmt.Errorf("fail to get org info, path: %s, error: %w", path, err)
		}
		//overwrite namespace type with org type
		ns.Type = string(org.OrgType)
		ns.Avatar = org.Logo
	} else {
		return nil, fmt.Errorf("invalid namespace type: %s", dbns.NamespaceType)
	}
	return ns, err

}
