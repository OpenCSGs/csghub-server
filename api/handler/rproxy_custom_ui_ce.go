//go:build !ee && !saas

package handler

import "github.com/gin-gonic/gin"

const rproxyCustomUIEnabled = false

func (r *RProxyHandler) initCustomUIComponent() {}

func (r *RProxyHandler) tryCustomUI(_ *gin.Context, _ string) bool {
	return false
}
