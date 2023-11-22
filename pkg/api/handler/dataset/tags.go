package dataset

import (
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
				"message": "Get dataset tags failed.",
			})
			return
		}

		respData := gin.H{
			"code": 200,
			"tags": tags,
		}

		c.JSON(http.StatusOK, respData)
	}
}
