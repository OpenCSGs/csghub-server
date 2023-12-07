package member

import (
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/member"
	"github.com/gin-gonic/gin"
)

func HandleDelete(memberCtrl *member.Controller) func(*gin.Context) {
	return func(c *gin.Context) {
		err := memberCtrl.Delete(c)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": fmt.Sprintf("Delete member repository failed. %v", err),
			})
			return
		}

		respData := gin.H{
			"code":    200,
			"message": "member repository deleted.",
		}

		c.JSON(http.StatusOK, respData)
	}
}
