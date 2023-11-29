package model

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleTags(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		tags, err := modelCtrl.Tags(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get model tags failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    tags,
			"message": "Get model tags successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
