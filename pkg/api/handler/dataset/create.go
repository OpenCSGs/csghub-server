package dataset

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleCreate(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		dataset, err := datasetCtrl.Create(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": "Created failed.",
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Dataset repository created.",
			"dataset": dataset,
		}

		c.JSON(http.StatusOK, respData)
	}
}
