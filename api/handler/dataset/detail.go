package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/dataset"
)

func HandleDetail(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		datasetDetail, err := datasetCtrl.Detail(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get dataset detail failed: %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    datasetDetail,
			"message": "Get dataset detail successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
