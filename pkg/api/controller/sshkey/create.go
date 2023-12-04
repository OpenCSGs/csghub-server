package sshkey

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Create(ctx *gin.Context) (*database.SSHKey, error) {
	var req types.CreateSSHKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	username := ctx.Param("username")
	// Check if username exists
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("User does not exists")
	}
	req.Username = username
	respSSHkey, err := c.gitServer.CreateSSHKey(&req)
	if err == nil {
		c.sshKeyStore.Create(ctx, respSSHkey, user)
	}

	return respSSHkey, err
}
