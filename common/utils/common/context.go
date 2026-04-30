package common

import (
	"context"

	"github.com/gin-gonic/gin"
)

// GinContextToStdContext converts a gin context to a standard context with user information
// It copies currentUserUUID, accessToken, ip_address, currentUser, and authType from gin context to standard context
func GinContextToStdContext(c *gin.Context) context.Context {
	// Get the base standard context from the request
	ctx := c.Request.Context()

	// Copy user UUID if exists
	if userUUID, exists := c.Get("currentUserUUID"); exists {
		ctx = context.WithValue(ctx, "currentUserUUID", userUUID)
	}

	// Copy access token if exists
	if token, exists := c.Get("accessToken"); exists {
		ctx = context.WithValue(ctx, "accessToken", token)
	}

	// Copy IP address if exists
	if ip, exists := c.Get("ip_address"); exists {
		ctx = context.WithValue(ctx, "ip_address", ip)
	}

	// Copy current user if exists
	if currentUser, exists := c.Get("currentUser"); exists {
		ctx = context.WithValue(ctx, "currentUser", currentUser)
	}

	// Copy auth type if exists
	if authType, exists := c.Get("authType"); exists {
		ctx = context.WithValue(ctx, "authType", authType)
	}

	return ctx
}
