package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/dataset"
)

func HandleFileCreate(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := datasetCtrl.FileCreate(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Create dataset file failed: %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Create dataset file successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}

func HandleFileUpdate(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := datasetCtrl.FileUpdate(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Update dataset file failed: %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "Update dataset file successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
