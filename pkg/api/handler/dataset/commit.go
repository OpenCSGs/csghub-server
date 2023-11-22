package dataset

import (
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
				"message": "Get dataset commits failed.",
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"commits": commits,
		}

		c.JSON(http.StatusOK, respData)
	}
}
