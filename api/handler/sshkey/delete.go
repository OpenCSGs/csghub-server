package sshkey

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/sshkey"
)

func HandleDelete(sshKeyCtrl *sshkey.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		err := sshKeyCtrl.Delete(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Deleted failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": fmt.Sprintf("SSH key deleted"),
		}

		c.JSON(http.StatusOK, respData)
	}
}
