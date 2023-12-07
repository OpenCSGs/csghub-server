package organization

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Index(ctx *gin.Context) (orgs []database.Organization, err error) {
	username := ctx.Query("username")
	orgs, err = c.orgStore.Index(ctx, username)
	return
}
