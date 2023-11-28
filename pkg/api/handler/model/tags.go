package model

import (
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
				"message": "Get model tags failed.",
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
