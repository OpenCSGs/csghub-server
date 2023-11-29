package dataset

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleCommits(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		commits, err := datasetCtrl.Commits(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get dataset commits failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    commits,
			"message": "Get dataset commits successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
