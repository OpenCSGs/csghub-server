package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/dataset"
)

func HandleIndex(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		datasets, total, err := datasetCtrl.Index(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get dataset list failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":        200,
			"total_count": total,
			"data":        datasets,
			"message":     "Get dataset list successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
