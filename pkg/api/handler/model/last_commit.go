package model

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

func HandleLastCommit(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		commit, err := modelCtrl.LastCommit(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": "Get model branches failed.",
			})
			return
		}

		respData := gin.H{
			"code":        200,
			"last_commit": commit,
		}

		c.JSON(http.StatusOK, respData)
	}
}
