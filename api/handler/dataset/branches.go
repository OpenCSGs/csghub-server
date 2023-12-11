package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/dataset"
)

func HandleBranches(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		branches, err := datasetCtrl.Branches(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get dataset branches failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    branches,
			"message": "Get dataset branches successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
