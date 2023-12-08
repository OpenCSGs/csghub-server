package organization

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/store/database"
	"opencsg.com/starhub-server/pkg/types"
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
