package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/dataset"
)

func HandleUpdate(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		dataset, err := datasetCtrl.Update(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Updated failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Dataset repository updated.",
			"data":    dataset,
		}

		c.JSON(http.StatusOK, respData)
	}
}
