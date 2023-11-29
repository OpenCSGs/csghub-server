package dataset

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleLastCommit(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		commit, err := datasetCtrl.LastCommit(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get last commit failed: %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    commit,
			"message": "Get last commit successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
