package dataset

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/dataset"
)

func HandleTags(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		tags, err := datasetCtrl.Tags(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get dataset tags failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    tags,
			"message": "Get dataset tags successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
