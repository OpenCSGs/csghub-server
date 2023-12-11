package accesstoken

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/component/accesstoken"
)

func HandleCreate(acCtrl *accesstoken.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		// TODO: Add parameter validation
		token, err := acCtrl.Create(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    "401",
				"message": fmt.Sprintf("Created token failed: %v", err.Error()),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": fmt.Sprintf("Token #%d created", token.ID),
			"data":    token,
		}

		c.JSON(http.StatusOK, respData)
	}
}
