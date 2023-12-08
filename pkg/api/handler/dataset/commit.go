package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/dataset"
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
