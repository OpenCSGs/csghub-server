package middleware

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// BindResourceApplicationIdentity prevents clients from impersonating another
// user when publishing a resource-application notification.
func BindResourceApplicationIdentity() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		body, err := io.ReadAll(ctx.Request.Body)
		if err != nil {
			httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
			ctx.Abort()
			return
		}
		ctx.Request.Body = io.NopCloser(bytes.NewReader(body))

		var messageReq types.MessageRequest
		if err := json.Unmarshal(body, &messageReq); err != nil ||
			messageReq.Scenario != types.MessageScenarioResourceApplication {
			ctx.Next()
			return
		}

		userUUID := httpbase.GetCurrentUserUUID(ctx)
		if userUUID == "" {
			httpbase.UnauthorizedError(ctx, errorx.ErrUserNotFound)
			ctx.Abort()
			return
		}

		var resourceReq types.ResourceApplicationNotificationReq
		if err := json.Unmarshal([]byte(messageReq.Parameters), &resourceReq); err != nil {
			httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
			ctx.Abort()
			return
		}
		resourceReq.UserUUID = userUUID
		resourceReq.UserName = httpbase.GetCurrentUser(ctx)

		parameters, err := json.Marshal(resourceReq)
		if err != nil {
			httpbase.ServerError(ctx, err)
			ctx.Abort()
			return
		}
		messageReq.Parameters = string(parameters)

		body, err = json.Marshal(messageReq)
		if err != nil {
			httpbase.ServerError(ctx, err)
			ctx.Abort()
			return
		}
		ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
		ctx.Request.ContentLength = int64(len(body))
		ctx.Next()
	}
}
