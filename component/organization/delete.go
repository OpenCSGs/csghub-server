package organization

import (
	"github.com/gin-gonic/gin"
)

func (c *Controller) Delete(ctx *gin.Context) (err error) {
	name := ctx.Param("name")
	err = c.gitServer.DeleteOrganization(name)
	if err == nil {
		c.orgStore.Delete(ctx, name)
	}

	return
}
