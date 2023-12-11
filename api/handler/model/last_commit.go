package model

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/model"
)

func HandleLastCommit(modelCtrl *model.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		commit, err := modelCtrl.LastCommit(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Get model last commit failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"data":    commit,
			"message": "Get model last commit successfully.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
