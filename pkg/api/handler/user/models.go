package user

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/user"
	"github.com/gin-gonic/gin"
)

func HandleModels(userCtrl *user.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		models, err := userCtrl.Models(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get user models failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Get user models successfully",
			"data":    models,
		}

		c.JSON(http.StatusOK, respData)
	}
}
