package sshkey

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/sshkey"
	"github.com/gin-gonic/gin"
)

func HandleCreate(sshKeyCtrl *sshkey.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		user, err := sshKeyCtrl.Create(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Created failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": fmt.Sprintf("User #%d created", user.ID),
			"data":    user,
		}

		c.JSON(http.StatusOK, respData)
	}
}
