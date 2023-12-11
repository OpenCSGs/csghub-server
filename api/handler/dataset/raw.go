package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/dataset"
)

func HandleFileRaw(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		content, err := datasetCtrl.FileRaw(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get dataset file content failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    content,
			"message": "Get dataset file content successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
