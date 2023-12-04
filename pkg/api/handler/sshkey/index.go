package sshkey

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/sshkey"
	"github.com/gin-gonic/gin"
)

func HandleIndex(sshKeyCtrl *sshkey.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		users, err := sshKeyCtrl.Index(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Created failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "List users successfully",
			"data":    users,
		}

		c.JSON(http.StatusOK, respData)
	}
}
