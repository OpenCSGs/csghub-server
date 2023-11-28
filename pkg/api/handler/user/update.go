package user

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/user"
	"github.com/gin-gonic/gin"
)

func HandleUpdate(userCtrl *user.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		user, err := userCtrl.Update(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Update failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": fmt.Sprintf("User #%d update", user.ID),
			"user":    user,
		}

		c.JSON(http.StatusOK, respData)
	}
}
