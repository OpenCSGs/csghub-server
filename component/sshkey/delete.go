package sshkey

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func (c *Controller) Delete(ctx *gin.Context) (err error) {
	id := ctx.Param("id")
	gid, err := strconv.Atoi(id)
	if err != nil {
		return
	}

	err = c.gitServer.DeleteSSHKey(gid)
	if err == nil {
		c.sshKeyStore.Delete(ctx, gid)
	}

	return
}
