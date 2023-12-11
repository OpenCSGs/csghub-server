package organization

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/builder/store/database"
)

func (c *Controller) Index(ctx *gin.Context) (orgs []database.Organization, err error) {
	username := ctx.Query("username")
	orgs, err = c.orgStore.Index(ctx, username)
	return
}
