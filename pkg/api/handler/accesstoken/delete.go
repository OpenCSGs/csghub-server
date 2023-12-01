package accesstoken

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/accesstoken"
	"github.com/gin-gonic/gin"
)

func HandleDelete(acCtrl *accesstoken.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		err := acCtrl.Delete(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Delete token failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Delete token successfully",
		}

		c.JSON(http.StatusOK, respData)
	}
}
