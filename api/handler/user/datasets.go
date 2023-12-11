package user

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/user"
)

func HandleDatasets(userCtrl *user.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		datasets, err := userCtrl.Datasets(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get user datasets failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Get user datasets successfully",
			"data":    datasets,
		}

		c.JSON(http.StatusOK, respData)
	}
}
