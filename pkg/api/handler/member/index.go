package member

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/api/controller/member"
)

func HandleIndex(memberCtrl *member.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		members, err := memberCtrl.Index(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Created member repository failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "member repository created.",
			"data":    members,
		}

		c.JSON(http.StatusOK, respData)
	}
}
