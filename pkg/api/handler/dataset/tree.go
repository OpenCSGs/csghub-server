package dataset

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"github.com/gin-gonic/gin"
)

func HandleTree(datasetCtrl *dataset.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		tree, err := datasetCtrl.Tree(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Get dataset repo tree failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    tree,
			"message": "Get dataset repo tree successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
