package dataset

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleDelete(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := datasetCtrl.Delete(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Delete dataset failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Dataset deleted.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
