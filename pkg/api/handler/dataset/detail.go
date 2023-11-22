package dataset

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleDetail(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		datasetDetail, err := datasetCtrl.Detail(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": "Request failed.",
			})
			return
		}

		respData := gin.H{
			"code":   200,
			"detail": datasetDetail,
		}

		c.JSON(http.StatusOK, respData)
	}
}
