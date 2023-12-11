package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/model"
)

func HandleTree(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		tree, err := modelCtrl.Tree(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get model repo tree failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    tree,
			"message": "Get model repo tree successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
