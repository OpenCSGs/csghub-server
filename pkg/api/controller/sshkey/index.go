package sshkey

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Index(ctx *gin.Context) (sshKeys []*database.SSHKey, err error) {
	username := ctx.Param("username")
	// Check if username exists
	_, err = c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return nil, err
	}

	sshKeys, err = c.sshKeyStore.Index(ctx, username, per, page)
	// respSSHkey, err := c.gitServer.ListSSHkeys(username, per, page)

	return
}
