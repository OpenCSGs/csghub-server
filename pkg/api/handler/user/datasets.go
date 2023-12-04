package user

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/user"
	"github.com/gin-gonic/gin"
)

func HandleDatasets(userCtrl *user.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		datasets, err := userCtrl.Datasets(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Update failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Get datasets successfully",
			"data":    datasets,
		}

		c.JSON(http.StatusOK, respData)
	}
}
