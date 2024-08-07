package router

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/handler"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log())
	r.Use(middleware.Authenticator(config))

	userHandler, err := handler.NewUserHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user handler:%w", err)
	}
	acHandler, err := handler.NewAccessTokenHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating token handler:%w", err)
	}
	orgHandler, err := handler.NewOrganizationHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	// Member
	memberCtrl, err := handler.NewMemberHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	//namespace
	nsCtrl, err := handler.NewNamespaceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating namespace controller:%w", err)
	}

	apiV1Group := r.Group("/api/v1")
	jwtGroup := apiV1Group.Group("/jwt")
	userGroup := apiV1Group.Group("/user")
	tokenGroup := apiV1Group.Group("/token")

	needAPIKey := middleware.OnlyAPIKeyAuthenticator(config)
	jwtHandler, err := handler.NewJWTHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating jwt handler:%w", err)
	}

	//don't need login
	{
		//casdoor
		apiV1Group.GET("/callback/casdoor", userHandler.Casdoor)
		//user
		userGroup.GET("/:username", userHandler.Get)
		// org and members
		apiV1Group.GET("/organizations", orgHandler.Index)
		apiV1Group.GET("/organization/:namespace", orgHandler.Get)
		apiV1Group.GET("/organization/:namespace/members", memberCtrl.OrgMembers)
		// Organization assets
		apiV1Group.GET("/organization/:namespace/models", orgHandler.Models)
		apiV1Group.GET("/organization/:namespace/datasets", orgHandler.Datasets)
		apiV1Group.GET("/organization/:namespace/codes", orgHandler.Codes)
		apiV1Group.GET("/organization/:namespace/spaces", orgHandler.Spaces)
		apiV1Group.GET("/organization/:namespace/collections", orgHandler.Collections)
	}

	//internal only
	{
		//organization
		//namespace
		apiV1Group.GET("/namespace/:path", needAPIKey, nsCtrl.GetInfo)
		//jwt
		jwtGroup.POST("/token", needAPIKey, jwtHandler.Create)
		jwtGroup.GET("/:token", needAPIKey, jwtHandler.Verify)
		// check token info
		tokenGroup.GET("/:token_value", needAPIKey, acHandler.Get)
	}

	apiV1Group.Use(mustLogin())
	userMatch := userMatch()

	// routers for users
	{
		// userGroup.POST("", userHandler.Create)
		// user self or admin
		userGroup.PUT("/:username", mustLogin(), userHandler.Update)
		//TODO:
		// userGroup.DELETE("/:username", userMatch, userHandler.Delete)
		// get user's all tokens
		userGroup.GET("/:username/tokens", userMatch, acHandler.GetUserTokens)

	}
	// routers for organizations
	{
		apiV1Group.POST("/organizations", orgHandler.Create)
		apiV1Group.PUT("/organization/:namespace", orgHandler.Update)
		apiV1Group.DELETE("/organization/:namespace", orgHandler.Delete)
	}
	// routers for members
	{
		apiV1Group.GET("/organization/:namespace/members/:username", userMatch, memberCtrl.GetMemberRole)
		apiV1Group.POST("/organization/:namespace/members", memberCtrl.Create)
		apiV1Group.PUT("/organization/:namespace/members/:username", memberCtrl.Update)
		apiV1Group.DELETE("/organization/:namespace/members/:username", memberCtrl.Delete)
	}
	// routers for access tokens
	{
		tokenGroup.POST("/:app/:token_name", acHandler.CreateAppToken)
		tokenGroup.PUT("/:app/:token_name", acHandler.Refresh)
		tokenGroup.DELETE("/:app/:token_name", acHandler.DeleteAppToken)
	}

	return r, nil
}

func userMatch() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentUser := httpbase.GetCurrentUser(ctx)
		if currentUser == "" {
			httpbase.UnauthorizedError(ctx, errors.New("unknown user, please login first"))
			ctx.Abort()
			return
		}

		userName := ctx.Param("username")
		if userName != currentUser {
			httpbase.UnauthorizedError(ctx, errors.New("user not match, try to query user account not owned"))
			slog.Error("user not match, try to query user account not owned", "currentUser", currentUser, "userName", userName)
			ctx.Abort()
			return
		}
	}
}

func mustLogin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentUser := httpbase.GetCurrentUser(ctx)
		if currentUser == "" {
			httpbase.UnauthorizedError(ctx, errors.New("unknown user, please login first"))
			ctx.Abort()
			return
		}
	}
}
