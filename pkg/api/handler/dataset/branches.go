package dataset

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleBranches(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		branches, err := datasetCtrl.Branches(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": "Get dataset branches failed.",
			})
			return
		}

		respData := gin.H{
			"code":     200,
			"branches": branches,
		}

		c.JSON(http.StatusOK, respData)
	}
}
