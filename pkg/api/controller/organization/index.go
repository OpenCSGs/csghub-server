package organization

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/store/database"
)

func (c *Controller) Index(ctx *gin.Context) (orgs []database.Organization, err error) {
	username := ctx.Query("username")
	orgs, err = c.orgStore.Index(ctx, username)
	return
}
