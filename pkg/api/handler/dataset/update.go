package dataset

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleUpdate(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		dataset, err := datasetCtrl.Update(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": "Ureated failed.",
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Dataset repository updated.",
			"dataset": dataset,
		}

		c.JSON(http.StatusOK, respData)
	}
}
