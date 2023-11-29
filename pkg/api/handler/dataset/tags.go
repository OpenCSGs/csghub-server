package dataset

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
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
