package utils

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetSceneFromContext(ctx *gin.Context) (int, error) {
	str := ctx.Query("scene")
	if str == "" {
		return 0, fmt.Errorf("bad request scene format")
	}
	scene, err := strconv.Atoi(str)
	return scene, err
}
