package organization

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Update(ctx *gin.Context) (org *database.Organization, err error) {
	var req types.EditOrgReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	name := ctx.Param("name")
	o, err := c.orgStore.FindByName(ctx, name)
	if err != nil {
		return nil, errors.New("Organization not found")
	}

	org, err = c.gitServer.UpdateOrganization(name, &req, &o)
	return
}
