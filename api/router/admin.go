package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/types"
)

// createAdminAcademyRoutes registers fixed operations for each academy submodule.
// The downstream service retains the original credentials and must authorize them independently.
func createAdminAcademyRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, proxyToAPI func(string, ...string) gin.HandlerFunc) {
	course := apiGroup.Group("/admin/academy/course", mc.Auth.PlatformAdmin(types.AdminAcademyCourse))
	course.GET("/courses", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/courses")}).AcademyCourses)
	course.POST("/courses", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/courses")}).AcademyCreateCourse)
	course.GET("/courses/:id", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/courses/%s", "id")}).AcademyCourse)
	course.PUT("/courses/:id", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/courses/%s", "id")}).AcademyUpdateCourse)
	course.DELETE("/courses/:id", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/courses/%s", "id")}).AcademyDeleteCourse)
	course.GET("/collections", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/collections")}).AcademyCourseCollections)

	collection := apiGroup.Group("/admin/academy/collection", mc.Auth.PlatformAdmin(types.AdminAcademyCollection))
	collection.GET("/collections", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/collections")}).AcademyCollections)
	collection.POST("/collections", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/collections")}).AcademyCreateCollection)
	collection.PUT("/collections/:id", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/collections/%s", "id")}).AcademyUpdateCollection)
	collection.DELETE("/collections/:id", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/collections/%s", "id")}).AcademyDeleteCollection)

	permission := apiGroup.Group("/admin/academy/user_permission", mc.Auth.PlatformAdmin(types.AdminAcademyUserPermission))
	permission.GET("/courses", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/courses")}).AcademyPermissionCourses)
	permission.GET("/course_grants", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/users")}).AcademyCourseGrants)
	permission.POST("/users/:user_id/permissions", (&adminSharedRouteHandler{target: proxyToAPI("/api/v1/academy/admin/users/%s/permissions", "user_id")}).AcademyUpdateCourseGrant)
}

// createAdminAccountAdminRoutes separates role discovery from administrator assignment privileges.
func createAdminAccountAdminRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, admin *handler.AdminHandler) {
	group := apiGroup.Group("/admin/account/admin", mc.Auth.NeedLogin)
	// The handler checks requested grants; requiring account/admin or Any here would add a different gate.
	group.POST("/me/permissions/check", admin.CheckPermissions)
	group.GET("/roles", mc.Auth.PlatformAdmin(types.AdminPermissionAny), admin.Roles)
	group.GET("/me/roles", mc.Auth.PlatformAdmin(types.AdminPermissionAny), admin.CurrentRoles)
	group.GET("/users", mc.Auth.PlatformAdmin(types.AdminAccountAdmin), admin.Users)
	group.PUT("/users/:user_id/roles/:role_code", mc.Auth.PlatformAdmin(types.AdminAccountAdmin), admin.AssignRole)
	group.DELETE("/users/:user_id/roles/:role_code", mc.Auth.PlatformAdmin(types.AdminAccountAdmin), admin.RemoveRole)
}

// createAdminCommonReadRoutes exposes broadcast and version reads to every administrator role.
func createAdminCommonReadRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, broadcast *handler.BroadcastHandler, version *handler.VersionHandler) {
	group := apiGroup.Group("/admin/common", middlewareCollection.Auth.PlatformAdmin(types.AdminPermissionAny))
	group.GET("/broadcasts/active", (&adminSharedRouteHandler{target: broadcast.Active}).CommonActiveBroadcast)
	group.GET("/version", (&adminSharedRouteHandler{target: version.Version}).CommonVersion)
}

// createAdminCommonTagRoute exposes shared tag choices to every active administrator role.
func createAdminCommonTagRoute(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, allTags gin.HandlerFunc) {
	apiGroup.GET("/admin/common/tags", middlewareCollection.Auth.PlatformAdmin(types.AdminPermissionAny), (&adminSharedRouteHandler{target: allTags}).CommonTags)
}

// createAdminCommonUserRoutes forwards profile requests to independently authorized user-service administrator routes.
func createAdminCommonUserRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, user *handler.InternalServiceProxyHandler) {
	group := apiGroup.Group("/admin/common/user", middlewareCollection.Auth.PlatformAdmin(types.AdminPermissionAny))
	group.GET("/:identifier", (&adminSharedRouteHandler{target: user.ProxyToApi("/api/v1/admin/common/user/%s", "identifier")}).CommonUser)
	group.PUT("/:identifier", (&adminSharedRouteHandler{target: user.ProxyToApi("/api/v1/admin/common/user/%s", "identifier")}).CommonUpdateUser)
	group.POST("/tags", (&adminSharedRouteHandler{target: user.ProxyToApi("/api/v1/admin/common/user/tags")}).CommonUserTags)
	group.POST("/email-verification-code/:email", (&adminSharedRouteHandler{target: user.ProxyToApi("/api/v1/admin/common/user/email-verification-code/%s", "email")}).CommonEmailVerificationCode)
}

// createAdminCommonNotificationRoutes forwards personal requests to independently authorized notification-service administrator routes.
func createAdminCommonNotificationRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, notifications *handler.InternalServiceProxyHandler) {
	group := apiGroup.Group("/admin/common/notifications", middlewareCollection.Auth.PlatformAdmin(types.AdminPermissionAny))
	group.GET("/message-types", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications/message-types")}).CommonNotificationTypes)
	group.GET("/setting", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications/setting")}).CommonNotificationSetting)
	group.PUT("/setting", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications/setting")}).CommonUpdateNotificationSetting)
	group.GET("/count", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications/count")}).CommonNotificationCount)
	group.GET("/poll/:limit", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications/poll/%s", "limit")}).CommonNotificationPoll)
	group.GET("", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications")}).CommonNotifications)
	group.PUT("/read", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications/read")}).CommonReadNotifications)
	group.DELETE("", (&adminSharedRouteHandler{target: notifications.ProxyToApi("/api/v1/admin/common/notifications")}).CommonDeleteNotifications)
}

// CommonActiveBroadcast delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Get the active broadcast for the administrator navigation bar
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Success 200 {object} types.Response{data=types.Broadcast}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/broadcasts/active [get]
func (h *adminSharedRouteHandler) CommonActiveBroadcast(ctx *gin.Context) { h.target(ctx) }

// CommonVersion delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Get the server version for the administrator navigation bar
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/version [get]
func (h *adminSharedRouteHandler) CommonVersion(ctx *gin.Context) { h.target(ctx) }

// CommonTags delegates shared administrator tag choices to the original tag handler.
// @Security ApiKey
// @Summary Get tag choices for the administrator navigation bar
// @Description Requires any active administrator role. Industry choices use scope=model and category=industry.
// @Tags Admin Common
// @Produce json
// @Param category query string false "Category name"
// @Param scope query string false "Scope name" Enums(model, dataset, code, space, prompt, mcp, skill, organization)
// @Param built_in query bool false "Filter built-in tags"
// @Param search query string false "Search name and show_name"
// @Param per query int false "Page size, default 50, max 100. When omitted with scope set, all matching tags are returned" default(50)
// @Param page query int false "Page number" default(1)
// @Success 200 {object} types.Response{data=[]types.RepoTag}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/tags [get]
func (h *adminSharedRouteHandler) CommonTags(ctx *gin.Context) { h.target(ctx) }

// CommonUser delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Get a profile for the administrator navigation bar
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Param identifier path string true "Username or user UUID"
// @Param type query string false "Use uuid to resolve a user UUID" Enums(uuid)
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/user/{identifier} [get]
func (h *adminSharedRouteHandler) CommonUser(ctx *gin.Context) { h.target(ctx) }

// CommonUpdateUser delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Update a profile from the administrator navigation bar
// @Description Requires an active administrator role; original business authorization and data scope are preserved. Role assignments use administrator role APIs.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param identifier path string true "User UUID"
// @Param type query string false "Legacy parameter; updates resolve identifier as a UUID" Enums(uuid)
// @Param body body types.UpdateUserRequest true "Profile fields"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/user/{identifier} [put]
func (h *adminSharedRouteHandler) CommonUpdateUser(ctx *gin.Context) { h.target(ctx) }

// CommonUserTags delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Set the current administrator user's industry tags
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param tagIDs body []int64 true "Industry tag IDs"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/user/tags [post]
func (h *adminSharedRouteHandler) CommonUserTags(ctx *gin.Context) { h.target(ctx) }

// CommonEmailVerificationCode delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Send a profile email verification code
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param email path string true "Email address"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/user/email-verification-code/{email} [post]
func (h *adminSharedRouteHandler) CommonEmailVerificationCode(ctx *gin.Context) { h.target(ctx) }

// CommonNotificationTypes delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary List notification types for the administrator navigation bar
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Success 200 {object} types.Response{data=[]string}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/message-types [get]
func (h *adminSharedRouteHandler) CommonNotificationTypes(ctx *gin.Context) { h.target(ctx) }

// CommonNotificationSetting delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Get the current administrator user's notification settings
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Param timezone query string false "Timezone" default(Asia/Shanghai)
// @Success 200 {object} types.Response{data=types.NotificationSetting}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/setting [get]
func (h *adminSharedRouteHandler) CommonNotificationSetting(ctx *gin.Context) { h.target(ctx) }

// CommonUpdateNotificationSetting delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Update the current administrator user's notification settings
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.UpdateNotificationReq true "Notification settings"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/setting [put]
func (h *adminSharedRouteHandler) CommonUpdateNotificationSetting(ctx *gin.Context) { h.target(ctx) }

// CommonNotificationCount delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Count the current administrator user's unread notifications
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Success 200 {object} types.Response{data=int}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/count [get]
func (h *adminSharedRouteHandler) CommonNotificationCount(ctx *gin.Context) { h.target(ctx) }

// CommonNotificationPoll delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Poll the current administrator user's new notifications
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Param limit path int true "Maximum number of notifications"
// @Param timezone query string false "Timezone" default(Asia/Shanghai)
// @Success 200 {object} types.Response{data=types.NewNotifications}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/poll/{limit} [get]
func (h *adminSharedRouteHandler) CommonNotificationPoll(ctx *gin.Context) { h.target(ctx) }

// CommonNotifications delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary List the current administrator user's notifications
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param notification_type query string false "Notification type"
// @Param unread_only query bool false "Only unread notifications" default(false)
// @Param title query string false "Notification title"
// @Success 200 {object} types.Response{data=types.NotificationsResp}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications [get]
func (h *adminSharedRouteHandler) CommonNotifications(ctx *gin.Context) { h.target(ctx) }

// CommonReadNotifications delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Mark the current administrator user's notifications as read
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.BatchNotificationOperationReq true "Notifications to mark as read"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications/read [put]
func (h *adminSharedRouteHandler) CommonReadNotifications(ctx *gin.Context) { h.target(ctx) }

// CommonDeleteNotifications delegates a common administrator request to its original handler.
// @Security ApiKey
// @Summary Delete the current administrator user's notifications
// @Description Requires an active administrator role; original business authorization and data scope are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.BatchNotificationOperationReq true "Notifications to delete"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/notifications [delete]
func (h *adminSharedRouteHandler) CommonDeleteNotifications(ctx *gin.Context) { h.target(ctx) }

// createAdminBroadcastRoutes protects broadcast management without inheriting the legacy admin group gate.
func createAdminBroadcastRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, broadcastHandler *handler.BroadcastHandler) {
	adminBroadcast := apiGroup.Group("/admin/broadcasts", middlewareCollection.Auth.PlatformAdmin(types.AdminSystemBroadcast))
	adminBroadcast.GET("", (&adminSharedRouteHandler{target: broadcastHandler.Index}).Broadcasts)
	adminBroadcast.POST("", (&adminSharedRouteHandler{target: broadcastHandler.Create}).CreateBroadcast)
	adminBroadcast.GET("/:id", (&adminSharedRouteHandler{target: broadcastHandler.Show}).Broadcast)
	adminBroadcast.PUT("/:id", (&adminSharedRouteHandler{target: broadcastHandler.Update}).UpdateBroadcast)
}

// Broadcasts delegates broadcast listing after the route's administrator authorization.
// @Security ApiKey
// @Summary List broadcasts for system administration
// @Description Requires system/broadcast administrator permission.
// @Tags Admin System Broadcast
// @Produce json
// @Success 200 {object} types.Response{data=[]types.Broadcast}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/broadcasts [get]
func (h *adminSharedRouteHandler) Broadcasts(ctx *gin.Context) { h.target(ctx) }

// CreateBroadcast delegates broadcast creation without changing the existing request or response.
// @Security ApiKey
// @Summary Create a broadcast for system administration
// @Description Requires system/broadcast administrator permission. A successful creation returns no data payload.
// @Tags Admin System Broadcast
// @Accept json
// @Produce json
// @Param body body types.Broadcast true "Broadcast to create"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/broadcasts [post]
func (h *adminSharedRouteHandler) CreateBroadcast(ctx *gin.Context) { h.target(ctx) }

// Broadcast provides an administrator entry independent of the ordinary broadcast detail handler.
// @Security ApiKey
// @Summary Get a broadcast for system administration
// @Description Requires system/broadcast administrator permission.
// @Tags Admin System Broadcast
// @Produce json
// @Param id path int true "Broadcast ID"
// @Success 200 {object} types.Response{data=types.Broadcast}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/broadcasts/{id} [get]
func (h *adminSharedRouteHandler) Broadcast(ctx *gin.Context) { h.target(ctx) }

// UpdateBroadcast delegates updates and retains the existing active-broadcast validation.
// @Security ApiKey
// @Summary Update a broadcast for system administration
// @Description Requires system/broadcast administrator permission. The path ID identifies the broadcast to update.
// @Tags Admin System Broadcast
// @Accept json
// @Produce json
// @Param id path int true "Broadcast ID"
// @Param body body types.Broadcast true "Broadcast fields to update"
// @Success 200 {object} types.Response{data=types.Broadcast}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/broadcasts/{id} [put]
func (h *adminSharedRouteHandler) UpdateBroadcast(ctx *gin.Context) { h.target(ctx) }

// adminAccountUserHandler documents account user routes that reuse the user service.
type adminAccountUserHandler struct {
	list   gin.HandlerFunc
	show   gin.HandlerFunc
	update gin.HandlerFunc
	verify gin.HandlerFunc
}

// adminAccountOrganizationHandler documents account organization routes that reuse existing handlers.
type adminAccountOrganizationHandler struct {
	list    gin.HandlerFunc
	mine    gin.HandlerFunc
	show    gin.HandlerFunc
	verify  gin.HandlerFunc
	create  gin.HandlerFunc
	units   gin.HandlerFunc
	members gin.HandlerFunc
	users   gin.HandlerFunc
	tags    gin.HandlerFunc
}

// createAdminAccountUserRoutes registers user administration aliases for shared APIs.
func createAdminAccountUserRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, userProxy *handler.InternalServiceProxyHandler) {
	h := &adminAccountUserHandler{
		list:   userProxy.ProxyToApi("/api/v1/admin/account/user/users"),
		show:   userProxy.ProxyToApi("/api/v1/admin/account/user/user/%s", "identifier"),
		update: userProxy.ProxyToApi("/api/v1/admin/account/user/user/%s", "identifier"),
		verify: userProxy.ProxyToApi("/api/v1/admin/account/user/user/verify/%s", "uuid"),
	}
	user := apiGroup.Group("/admin/account/user", middlewareCollection.Auth.PlatformAdmin(types.AdminAccountUser))
	user.GET("/users", h.List)
	user.GET("/user/:identifier", h.Show)
	user.PUT("/user/:identifier", h.Update)
	user.GET("/user/verify/:uuid", h.Verify)
	apiGroup.GET("/admin/account/admin/users/search", middlewareCollection.Auth.PlatformAdmin(types.AdminAccountAdmin), (&adminSharedRouteHandler{target: userProxy.ProxyToApi("/api/v1/admin/account/admin/users/search")}).AdminCandidateUsers)
}

// createAdminAccountOrganizationRoutes registers organization administration aliases for shared APIs.
func createAdminAccountOrganizationRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, userProxy *handler.InternalServiceProxyHandler, tags *handler.TagsHandler) {
	h := &adminAccountOrganizationHandler{
		list:   userProxy.ProxyToApi("/api/v1/admin/account/organization/organizations"),
		show:   userProxy.ProxyToApi("/api/v1/admin/account/organization/organization/%s", "name"),
		verify: userProxy.ProxyToApi("/api/v1/admin/account/organization/organization/verify/%s", "name"),
		create: userProxy.ProxyToApi("/api/v1/admin/account/organization/organizations"),
		users:  userProxy.ProxyToApi("/api/v1/admin/account/organization/users"),
		tags:   tags.AllTags,
	}
	organization := apiGroup.Group("/admin/account/organization", middlewareCollection.Auth.PlatformAdmin(types.AdminAccountOrganization))
	organization.GET("/organizations", middlewareCollection.License.Check, h.List)
	organization.GET("/organization/:name", h.Show)
	organization.GET("/organization/verify/:name", h.Verify)
	organization.POST("/organizations", h.Create)
	organization.GET("/tags", h.Tags)
	organization.GET("/users", h.Users)
}

// createAdminAccountOrganizationHierarchyRoutes registers shared hierarchy read aliases when hierarchy is enabled.
func createAdminAccountOrganizationHierarchyRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, userProxy *handler.InternalServiceProxyHandler) {
	h := &adminAccountOrganizationHandler{
		mine:    userProxy.ProxyToApi("/api/v1/admin/account/organization/organizations/mine"),
		units:   userProxy.ProxyToApi("/api/v1/admin/account/organization/organizations/%s/units", "organization_uuid"),
		members: userProxy.ProxyToApi("/api/v1/admin/account/organization/organizations/%s/members", "organization_uuid"),
	}
	organization := apiGroup.Group("/admin/account/organization", middlewareCollection.Auth.PlatformAdmin(types.AdminAccountOrganization))
	organization.GET("/organizations/mine", h.Mine)
	organization.GET("/organizations/:organization_uuid/units", h.Units)
	organization.GET("/organizations/:organization_uuid/members", h.Members)
}

// List returns the user list through the existing user service.
// @Security ApiKey
// @Summary List users for account administration
// @Description Requires account/user administrator permission, independently checked by the user service.
// @Tags Admin Account User
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param verify_status query string false "Verification status"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/user/users [get]
func (h *adminAccountUserHandler) List(ctx *gin.Context) { h.list(ctx) }

// AdminCandidateUsers searches users eligible for administrator role assignment.
// @Security ApiKey
// @Summary Search users for administrator role assignment
// @Description Requires account/admin permission, independently checked by the user service. Searches all users, including users without administrator roles, and preserves the nested user-list response with numeric user IDs.
// @Tags Admin Account Admin
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param verify_status query string false "Verification status"
// @Param labels query []string false "User labels"
// @Param exact_match query bool false "Match the search text exactly"
// @Param sort_by query string false "Sort field"
// @Param sort_order query string false "Sort direction"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/admin/users/search [get]
func (h *adminSharedRouteHandler) AdminCandidateUsers(ctx *gin.Context) { h.target(ctx) }

// Show returns a user through the existing user service.
// @Security ApiKey
// @Summary Get a user for account administration
// @Description Requires account/user administrator permission.
// @Tags Admin Account User
// @Produce json
// @Param identifier path string true "Username"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/user/user/{identifier} [get]
func (h *adminAccountUserHandler) Show(ctx *gin.Context) { h.show(ctx) }

// Update changes a user through the existing user service.
// @Security ApiKey
// @Summary Update a user for account administration
// @Description Requires account/user administrator permission. Role assignments use administrator role APIs.
// @Tags Admin Account User
// @Accept json
// @Produce json
// @Param identifier path string true "User UUID"
// @Param body body types.UpdateUserRequest true "Profile fields"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/user/user/{identifier} [put]
func (h *adminAccountUserHandler) Update(ctx *gin.Context) { h.update(ctx) }

// Verify returns user verification details through the existing user service.
// @Security ApiKey
// @Summary Get user verification for account administration
// @Description Requires account/user administrator permission.
// @Tags Admin Account User
// @Produce json
// @Param uuid path string true "User UUID"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/user/user/verify/{uuid} [get]
func (h *adminAccountUserHandler) Verify(ctx *gin.Context) { h.verify(ctx) }

// List returns organizations through the independently authorized user service.
// @Security ApiKey
// @Summary List organizations for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param org_type query string false "Organization type filter"
// @Param verify_status query string false "Verification status filter"
// @Param tag query string false "Tag name filter"
// @Success 200 {object} types.Response{data=object}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/organization/organizations [get]
func (h *adminAccountOrganizationHandler) List(ctx *gin.Context) { h.list(ctx) }

// Mine returns active root organizations through the independently authorized user service.
// @Security ApiKey
// @Summary List active root organizations for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/organizations/mine [get]
func (h *adminAccountOrganizationHandler) Mine(ctx *gin.Context) { h.mine(ctx) }

// Show returns an organization through the existing user service.
// @Security ApiKey
// @Summary Get an organization for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Produce json
// @Param name path string true "Organization path"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/organization/{name} [get]
func (h *adminAccountOrganizationHandler) Show(ctx *gin.Context) { h.show(ctx) }

// Verify returns organization verification details through the existing user service.
// @Security ApiKey
// @Summary Get organization verification for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Produce json
// @Param name path string true "Organization path"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/organization/verify/{name} [get]
func (h *adminAccountOrganizationHandler) Verify(ctx *gin.Context) { h.verify(ctx) }

// Create creates an organization through the existing user service.
// @Security ApiKey
// @Summary Create an organization for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Accept json
// @Produce json
// @Param body body types.CreateOrgReq true "Organization fields"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/organizations [post]
func (h *adminAccountOrganizationHandler) Create(ctx *gin.Context) { h.create(ctx) }

// Units returns organization units through the existing user service.
// @Security ApiKey
// @Summary List organization units for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Produce json
// @Param organization_uuid path string true "Organization UUID"
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/organizations/{organization_uuid}/units [get]
func (h *adminAccountOrganizationHandler) Units(ctx *gin.Context) { h.units(ctx) }

// Members returns organization members through the existing user service.
// @Security ApiKey
// @Summary List organization members for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Produce json
// @Param organization_uuid path string true "Organization UUID"
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/organizations/{organization_uuid}/members [get]
func (h *adminAccountOrganizationHandler) Members(ctx *gin.Context) { h.members(ctx) }

// Users returns user candidates for organization membership.
// @Security ApiKey
// @Summary Search users for organization membership
// @Description Requires account/organization administrator permission. Returns only id, uuid, username, nickname, and avatar.
// @Tags Admin Account Organization
// @Produce json
// @Param search query string false "Username search"
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/users [get]
func (h *adminAccountOrganizationHandler) Users(ctx *gin.Context) { h.users(ctx) }

// Tags returns organization industry tags through the existing tag handler.
// @Security ApiKey
// @Summary List organization tags for account administration
// @Description Requires account/organization administrator permission.
// @Tags Admin Account Organization
// @Produce json
// @Param scope query string false "Tag scope" Enums(organization)
// @Param category query string false "Tag category"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/account/organization/tags [get]
func (h *adminAccountOrganizationHandler) Tags(ctx *gin.Context) { h.tags(ctx) }

// adminMarketingEmailNotificationHandler documents notification sending for email operations.
type adminMarketingEmailNotificationHandler struct {
	send gin.HandlerFunc
}

// createAdminMarketingEmailNotificationRoute gives email sending its own administrator permission.
func createAdminMarketingEmailNotificationRoute(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, notificationProxy *handler.InternalServiceProxyHandler) {
	h := &adminMarketingEmailNotificationHandler{send: notificationProxy.ProxyToApi("/api/v1/admin/marketing/email_sending/notifications")}
	email := apiGroup.Group("/admin/marketing/email_sending", middlewareCollection.Auth.PlatformAdmin(types.AdminMarketingEmailSending))
	email.POST("/notifications", middleware.BindResourceApplicationIdentity(), h.Send)
}

// Send forwards a recommendation notification to the notification service.
// @Security ApiKey
// @Summary Send an email operation notification
// @Description Requires marketing/email_sending administrator permission.
// @Tags Admin Marketing Email Sending
// @Accept json
// @Produce json
// @Param body body object true "Notification request"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/marketing/email_sending/notifications [post]
func (h *adminMarketingEmailNotificationHandler) Send(ctx *gin.Context) { h.send(ctx) }

// adminSharedRouteHandler documents one administrator alias for an existing business handler.
type adminSharedRouteHandler struct{ target gin.HandlerFunc }

// createAdminSharedResourceRoutes registers shared selectors with their calling submodule permissions.
func createAdminSharedResourceRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, resource *handler.SpaceResourceHandler) {
	apiGroup.GET("/admin/compute/overview/space_resources", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeOverview), (&adminSharedRouteHandler{target: resource.Index}).ComputeOverviewResources)
	apiGroup.GET("/admin/compute/spec/space_resources", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeSpec), (&adminSharedRouteHandler{target: resource.Index}).ComputeSpecResources)
	apiGroup.GET("/admin/compute/spec/space_resources/hardware_types", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeSpec), (&adminSharedRouteHandler{target: resource.ListHardwareTypes}).ComputeSpecHardware)
	apiGroup.GET("/admin/compute/spec/space_resources/scenarios", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeSpec), (&adminSharedRouteHandler{target: resource.ListScenarios}).ComputeSpecScenarios)
	apiGroup.GET("/admin/compute/scenario/space_resources/scenarios", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeScenario), (&adminSharedRouteHandler{target: resource.ListScenarios}).ComputeScenarioScenarios)
	apiGroup.GET("/admin/compute/node/space_resources", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeNode), (&adminSharedRouteHandler{target: resource.Index}).ComputeNodeResources)
	apiGroup.GET("/admin/apigateway/public_inference/space_resources", middlewareCollection.Auth.PlatformAdmin(types.AdminAPIGatewayPublicInference), (&adminSharedRouteHandler{target: resource.Index}).PublicInferenceResources)
	createAdminBillingResourceRoutes(apiGroup, middlewareCollection, resource)
	apiGroup.GET("/admin/compute/node/space_resources/hardware_types", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeNode), (&adminSharedRouteHandler{target: resource.ListHardwareTypes}).ComputeNodeHardware)
}

// createAdminSharedClusterRoutes registers shared selectors with their calling submodule permissions.
func createAdminSharedClusterRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, cluster *handler.ClusterHandler) {
	apiGroup.GET("/admin/compute/overview/cluster/usage", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeOverview), (&adminSharedRouteHandler{target: cluster.GetClusterUsage}).ComputeOverviewUsage)
	apiGroup.GET("/admin/compute/spec/cluster", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeSpec), (&adminSharedRouteHandler{target: cluster.Index}).ComputeSpecCluster)
	apiGroup.GET("/admin/compute/spec/cluster/usage", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeSpec), (&adminSharedRouteHandler{target: cluster.GetClusterUsage}).ComputeSpecUsage)
	apiGroup.GET("/admin/compute/cluster/cluster/usage", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeCluster), (&adminSharedRouteHandler{target: cluster.GetClusterUsage}).ComputeClusterUsage)
	apiGroup.GET("/admin/compute/cluster/cluster/:id", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeCluster), (&adminSharedRouteHandler{target: cluster.GetClusterById}).ComputeClusterDetail)
	apiGroup.GET("/admin/apigateway/public_inference/cluster", middlewareCollection.Auth.PlatformAdmin(types.AdminAPIGatewayPublicInference), (&adminSharedRouteHandler{target: cluster.Index}).PublicInferenceCluster)
	apiGroup.GET("/admin/apigateway/public_inference/cluster/:id", middlewareCollection.Auth.PlatformAdmin(types.AdminAPIGatewayPublicInference), (&adminSharedRouteHandler{target: cluster.GetClusterById}).PublicInferenceClusterDetail)
	createAdminBillingClusterRoutes(apiGroup, middlewareCollection, cluster)
	apiGroup.GET("/admin/compute/node/cluster", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeNode), (&adminSharedRouteHandler{target: cluster.Index}).ComputeNodeCluster)
}

// createAdminSharedUserProxyRoutes registers shared selectors with their calling submodule permissions.
func createAdminSharedUserProxyRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, userProxy *handler.InternalServiceProxyHandler) {
	apiGroup.GET("/admin/compute/node/users", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeNode), (&adminSharedRouteHandler{target: userProxy.ProxyToApi("/api/v1/admin/compute/node/users")}).ComputeNodeUsers)
	apiGroup.GET("/admin/compute/node/organizations", middlewareCollection.Auth.PlatformAdmin(types.AdminComputeNode), middlewareCollection.License.Check, (&adminSharedRouteHandler{target: userProxy.ProxyToApi("/api/v1/admin/compute/node/organizations")}).ComputeNodeOrganizations)
	apiGroup.GET("/admin/asset/model/users", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetModel), (&adminSharedRouteHandler{target: userProxy.ProxyToApi("/api/v1/admin/asset/model/users")}).ModelOwnerUsers)
	apiGroup.GET("/admin/asset/dataset/users", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetDataset), (&adminSharedRouteHandler{target: userProxy.ProxyToApi("/api/v1/admin/asset/dataset/users")}).DatasetOwnerUsers)
	apiGroup.GET("/admin/academy/user_permission/users", middlewareCollection.Auth.PlatformAdmin(types.AdminAcademyUserPermission), (&adminSharedRouteHandler{target: userProxy.ProxyToApi("/api/v1/admin/academy/user_permission/users")}).AcademyPermissionUsers)
	createAdminBillingUserProxyRoutes(apiGroup, middlewareCollection, userProxy)
}

// ComputeOverviewResources serves the shared /space_resources operation for compute/overview.
// @Security ApiKey
// @Summary Read /space_resources for compute/overview administration
// @Description Requires compute/overview administrator permission; original query parameters are preserved.
// @Tags Admin Compute Overview
// @Produce json
// @Param        per query int false "page size" default(50)
// @Param        page query int false "per page" default(1)
// @Param        cluster_id query string false "cluster_id"
// @Param 		 deploy_type query int false "deploy type(0-space,1-inference,2-finetune,3-serverless,4-evaluation)" Enums(0, 1, 2, 3, 4) default(1)
// @Param resource_type query string false "Resource type"
// @Param hardware_type query string false "Hardware type"
// @Param is_available query bool false "Availability"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/overview/space_resources [get]
func (h *adminSharedRouteHandler) ComputeOverviewResources(ctx *gin.Context) { h.target(ctx) }

// ComputeSpecResources serves the shared /space_resources operation for compute/spec.
// @Security ApiKey
// @Summary Read /space_resources for compute/spec administration
// @Description Requires compute/spec administrator permission; original query parameters are preserved.
// @Tags Admin Compute Spec
// @Produce json
// @Param        per query int false "page size" default(50)
// @Param        page query int false "per page" default(1)
// @Param        cluster_id query string false "cluster_id"
// @Param 		 deploy_type query int false "deploy type(0-space,1-inference,2-finetune,3-serverless,4-evaluation)" Enums(0, 1, 2, 3, 4) default(1)
// @Param resource_type query string false "Resource type"
// @Param hardware_type query string false "Hardware type"
// @Param is_available query bool false "Availability"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/spec/space_resources [get]
func (h *adminSharedRouteHandler) ComputeSpecResources(ctx *gin.Context) { h.target(ctx) }

// ComputeSpecHardware serves the shared /space_resources/hardware_types operation for compute/spec.
// @Security ApiKey
// @Summary Read /space_resources/hardware_types for compute/spec administration
// @Description Requires compute/spec administrator permission; original query parameters are preserved.
// @Tags Admin Compute Spec
// @Produce json
// @Param cluster_id query string false "Cluster ID"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/spec/space_resources/hardware_types [get]
func (h *adminSharedRouteHandler) ComputeSpecHardware(ctx *gin.Context) { h.target(ctx) }

// ComputeSpecScenarios serves the shared /space_resources/scenarios operation for compute/spec.
// @Security ApiKey
// @Summary Read /space_resources/scenarios for compute/spec administration
// @Description Requires compute/spec administrator permission; original query parameters are preserved.
// @Tags Admin Compute Spec
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/spec/space_resources/scenarios [get]
func (h *adminSharedRouteHandler) ComputeSpecScenarios(ctx *gin.Context) { h.target(ctx) }

// ComputeScenarioScenarios serves the shared /space_resources/scenarios operation for compute/scenario.
// @Security ApiKey
// @Summary Read /space_resources/scenarios for compute/scenario administration
// @Description Requires compute/scenario administrator permission; original query parameters are preserved.
// @Tags Admin Compute Scenario
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/scenario/space_resources/scenarios [get]
func (h *adminSharedRouteHandler) ComputeScenarioScenarios(ctx *gin.Context) { h.target(ctx) }

// ComputeNodeResources serves the shared /space_resources operation for compute/node.
// @Security ApiKey
// @Summary Read /space_resources for compute/node administration
// @Description Requires compute/node administrator permission; original query parameters are preserved.
// @Tags Admin Compute Node
// @Produce json
// @Param        per query int false "page size" default(50)
// @Param        page query int false "per page" default(1)
// @Param        cluster_id query string false "cluster_id"
// @Param 		 deploy_type query int false "deploy type(0-space,1-inference,2-finetune,3-serverless,4-evaluation)" Enums(0, 1, 2, 3, 4) default(1)
// @Param resource_type query string false "Resource type"
// @Param hardware_type query string false "Hardware type"
// @Param is_available query bool false "Availability"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/node/space_resources [get]
func (h *adminSharedRouteHandler) ComputeNodeResources(ctx *gin.Context) { h.target(ctx) }

// PublicInferenceResources serves the shared /space_resources operation for apigateway/public_inference.
// @Security ApiKey
// @Summary Read /space_resources for apigateway/public_inference administration
// @Description Requires apigateway/public_inference administrator permission; original query parameters are preserved.
// @Tags Admin Apigateway Public Inference
// @Produce json
// @Param        per query int false "page size" default(50)
// @Param        page query int false "per page" default(1)
// @Param        cluster_id query string false "cluster_id"
// @Param 		 deploy_type query int false "deploy type(0-space,1-inference,2-finetune,3-serverless,4-evaluation)" Enums(0, 1, 2, 3, 4) default(1)
// @Param resource_type query string false "Resource type"
// @Param hardware_type query string false "Hardware type"
// @Param is_available query bool false "Availability"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/apigateway/public_inference/space_resources [get]
func (h *adminSharedRouteHandler) PublicInferenceResources(ctx *gin.Context) { h.target(ctx) }

// ComputeOverviewUsage serves the shared /cluster/usage operation for compute/overview.
// @Security ApiKey
// @Summary Read /cluster/usage for compute/overview administration
// @Description Requires compute/overview administrator permission; original query parameters are preserved.
// @Tags Admin Compute Overview
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/overview/cluster/usage [get]
func (h *adminSharedRouteHandler) ComputeOverviewUsage(ctx *gin.Context) { h.target(ctx) }

// ComputeSpecCluster serves the shared /cluster operation for compute/spec.
// @Security ApiKey
// @Summary Read /cluster for compute/spec administration
// @Description Requires compute/spec administrator permission; original query parameters are preserved.
// @Tags Admin Compute Spec
// @Produce json
// @Param        scope  query  types.ClusterScope  false  "Scope of cluster"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/spec/cluster [get]
func (h *adminSharedRouteHandler) ComputeSpecCluster(ctx *gin.Context) { h.target(ctx) }

// ComputeSpecUsage serves the shared /cluster/usage operation for compute/spec.
// @Security ApiKey
// @Summary Read /cluster/usage for compute/spec administration
// @Description Requires compute/spec administrator permission; original query parameters are preserved.
// @Tags Admin Compute Spec
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/spec/cluster/usage [get]
func (h *adminSharedRouteHandler) ComputeSpecUsage(ctx *gin.Context) { h.target(ctx) }

// ComputeClusterUsage serves the shared /cluster/usage operation for compute/cluster.
// @Security ApiKey
// @Summary Read /cluster/usage for compute/cluster administration
// @Description Requires compute/cluster administrator permission; original query parameters are preserved.
// @Tags Admin Compute Cluster
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/cluster/cluster/usage [get]
func (h *adminSharedRouteHandler) ComputeClusterUsage(ctx *gin.Context) { h.target(ctx) }

// ComputeClusterDetail serves the shared /cluster/:id operation for compute/cluster.
// @Security ApiKey
// @Summary Read /cluster/:id for compute/cluster administration
// @Description Requires compute/cluster administrator permission; original query parameters are preserved.
// @Tags Admin Compute Cluster
// @Produce json
// @Param id path string true "Id"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/cluster/cluster/{id} [get]
func (h *adminSharedRouteHandler) ComputeClusterDetail(ctx *gin.Context) { h.target(ctx) }

// PublicInferenceCluster serves the shared /cluster operation for apigateway/public_inference.
// @Security ApiKey
// @Summary Read /cluster for apigateway/public_inference administration
// @Description Requires apigateway/public_inference administrator permission; original query parameters are preserved.
// @Tags Admin Apigateway Public Inference
// @Produce json
// @Param        scope  query  types.ClusterScope  false  "Scope of cluster"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/apigateway/public_inference/cluster [get]
func (h *adminSharedRouteHandler) PublicInferenceCluster(ctx *gin.Context) { h.target(ctx) }

// PublicInferenceClusterDetail serves the shared /cluster/:id operation for apigateway/public_inference.
// @Security ApiKey
// @Summary Read /cluster/:id for apigateway/public_inference administration
// @Description Requires apigateway/public_inference administrator permission; original query parameters are preserved.
// @Tags Admin Apigateway Public Inference
// @Produce json
// @Param id path string true "Id"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/apigateway/public_inference/cluster/{id} [get]
func (h *adminSharedRouteHandler) PublicInferenceClusterDetail(ctx *gin.Context) { h.target(ctx) }

// ComputeNodeUsers serves the shared /users operation for compute/node.
// @Security ApiKey
// @Summary Read /users for compute/node administration
// @Description Requires compute/node administrator permission; original query parameters are preserved. Returns only id, uuid, username, nickname, and avatar.
// @Tags Admin Compute Node
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/node/users [get]
func (h *adminSharedRouteHandler) ComputeNodeUsers(ctx *gin.Context) { h.target(ctx) }

// ComputeNodeOrganizations serves the shared /organizations operation for compute/node.
// @Security ApiKey
// @Summary Read /organizations for compute/node administration
// @Description Requires compute/node administrator permission; original query parameters are preserved. Only selector identity, display fields, and namespace identifiers are returned; contact details, roles, verification metadata, and tags are excluded.
// @Tags Admin Compute Node
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param org_type query string false "Organization type filter"
// @Param verify_status query string false "Verification status filter"
// @Param tag query string false "Tag name filter"
// @Success 200 {object} types.Response{data=types.OrganizationChoiceList}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/compute/node/organizations [get]
func (h *adminSharedRouteHandler) ComputeNodeOrganizations(ctx *gin.Context) { h.target(ctx) }

// ModelOwnerUsers serves the shared /users operation for asset/model.
// @Security ApiKey
// @Summary Read /users for asset/model administration
// @Description Requires asset/model administrator permission; original query parameters are preserved. Returns only id, uuid, username, nickname, and avatar.
// @Tags Admin Asset Model
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/users [get]
func (h *adminSharedRouteHandler) ModelOwnerUsers(ctx *gin.Context) { h.target(ctx) }

// DatasetOwnerUsers serves the shared /users operation for asset/dataset.
// @Security ApiKey
// @Summary Read /users for asset/dataset administration
// @Description Requires asset/dataset administrator permission; original query parameters are preserved. Returns only id, uuid, username, nickname, and avatar.
// @Tags Admin Asset Dataset
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/dataset/users [get]
func (h *adminSharedRouteHandler) DatasetOwnerUsers(ctx *gin.Context) { h.target(ctx) }

// AcademyPermissionUsers serves the shared /users operation for academy/user_permission.
// @Security ApiKey
// @Summary Read /users for academy/user_permission administration
// @Description Requires academy/user_permission administrator permission; original query parameters are preserved. Returns only id, uuid, username, nickname, and avatar.
// @Tags Admin Academy User Permission
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/academy/user_permission/users [get]
func (h *adminSharedRouteHandler) AcademyPermissionUsers(ctx *gin.Context) { h.target(ctx) }

// AcademyCourses forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary List courses
// @Description Requires academy/course; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Course
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param q query string false "Course search query"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/course/courses [get]
func (h *adminSharedRouteHandler) AcademyCourses(ctx *gin.Context) { h.target(ctx) }

// AcademyCreateCourse forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Create a course
// @Description Requires academy/course; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Course
// @Produce json
// @Accept json
// @Param body body object true "Academy service operation payload"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/course/courses [post]
func (h *adminSharedRouteHandler) AcademyCreateCourse(ctx *gin.Context) { h.target(ctx) }

// AcademyCourse forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Get a course for administration
// @Description Requires academy/course; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Course
// @Produce json
// @Param id path string true "Resource identifier"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/course/courses/{id} [get]
func (h *adminSharedRouteHandler) AcademyCourse(ctx *gin.Context) { h.target(ctx) }

// AcademyUpdateCourse forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Update a course
// @Description Requires academy/course; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Course
// @Produce json
// @Param id path string true "Resource identifier"
// @Accept json
// @Param body body object true "Academy service operation payload"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/course/courses/{id} [put]
func (h *adminSharedRouteHandler) AcademyUpdateCourse(ctx *gin.Context) { h.target(ctx) }

// AcademyDeleteCourse forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Delete a course
// @Description Requires academy/course; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Course
// @Produce json
// @Param id path string true "Resource identifier"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/course/courses/{id} [delete]
func (h *adminSharedRouteHandler) AcademyDeleteCourse(ctx *gin.Context) { h.target(ctx) }

// AcademyCourseCollections forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary List collections for the course editor
// @Description Requires academy/course; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Course
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/course/collections [get]
func (h *adminSharedRouteHandler) AcademyCourseCollections(ctx *gin.Context) { h.target(ctx) }

// AcademyCollections forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary List collections
// @Description Requires academy/collection; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Collection
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/collection/collections [get]
func (h *adminSharedRouteHandler) AcademyCollections(ctx *gin.Context) { h.target(ctx) }

// AcademyCreateCollection forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Create a collection
// @Description Requires academy/collection; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Collection
// @Produce json
// @Accept json
// @Param body body object true "Academy service operation payload"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/collection/collections [post]
func (h *adminSharedRouteHandler) AcademyCreateCollection(ctx *gin.Context) { h.target(ctx) }

// AcademyUpdateCollection forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Update a collection
// @Description Requires academy/collection; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Collection
// @Produce json
// @Param id path string true "Resource identifier"
// @Accept json
// @Param body body object true "Academy service operation payload"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/collection/collections/{id} [put]
func (h *adminSharedRouteHandler) AcademyUpdateCollection(ctx *gin.Context) { h.target(ctx) }

// AcademyDeleteCollection forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Delete a collection
// @Description Requires academy/collection; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy Collection
// @Produce json
// @Param id path string true "Resource identifier"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/collection/collections/{id} [delete]
func (h *adminSharedRouteHandler) AcademyDeleteCollection(ctx *gin.Context) { h.target(ctx) }

// AcademyPermissionCourses forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary List courses for user permission assignment
// @Description Requires academy/user_permission; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy User Permission
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param q query string false "Course search query"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/user_permission/courses [get]
func (h *adminSharedRouteHandler) AcademyPermissionCourses(ctx *gin.Context) { h.target(ctx) }

// AcademyCourseGrants forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary List user course grants
// @Description Requires academy/user_permission; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy User Permission
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/user_permission/course_grants [get]
func (h *adminSharedRouteHandler) AcademyCourseGrants(ctx *gin.Context) { h.target(ctx) }

// AcademyUpdateCourseGrant forwards one authorized academy operation with its original payload.
// @Security ApiKey
// @Summary Grant or revoke a user course permission
// @Description Requires academy/user_permission; query parameters, body, and downstream responses are preserved.
// @Tags Admin Academy User Permission
// @Produce json
// @Param user_id path string true "User UUID"
// @Accept json
// @Param body body object true "Academy service operation payload"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/user_permission/users/{user_id}/permissions [post]
func (h *adminSharedRouteHandler) AcademyUpdateCourseGrant(ctx *gin.Context) { h.target(ctx) }

// adminAssetResourceAliasHandler reuses repository operations under asset permissions.
type adminAssetResourceAliasHandler struct{ index, show, update, size gin.HandlerFunc }

// createAdminAssetModelRoutes registers shared model repository operations.
func createAdminAssetModelRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, resource *handler.ModelHandler, repo *handler.RepoHandler) {
	h := &adminAssetResourceAliasHandler{index: resource.Index, show: resource.Show, update: resource.Update, size: repo.GetRepoSizeByBranch}
	group := apiGroup.Group("/admin/asset/model/models", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetModel), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists)
	group.GET("", h.ListModel)
	group.GET("/:namespace/:name", h.ShowModel)
	group.PUT("/:namespace/:name", h.UpdateModel)
	group.GET("/:namespace/:name/size/:branch", h.SizeModel)
}

// ListModel serves the shared model list operation for asset administration.
// @Security ApiKey
// @Summary List model resources for asset administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Produce json
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag, deprecated"
// @Param        framework_tag query string false "filter by framework tag, deprecated"
// @Param        license_tag query string false "filter by license tag, deprecated"
// @Param        language_tag query string false "filter by language tag, deprecated"
// @Param        tag_category query string false "filter by tag category"
// @Param        tag_name query string false "filter by tag name"
// @Param        tag_group query string false "filter by tag group"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        xnet_migration_status query string false "filter by xnet migration status" Enums(pending, running, completed, failed)
// @Param        compliance_status query string false "filter by compliance status" Enums(compliant, pending_review, non_compliant)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Param        model_tree query string false "example: base_model:finetune:1"
// @Param        list_serverless query bool false "list serverless" default(false)
// @Param        model_params_min query number false "minimum model parameters in billions"
// @Param        model_params_max query number false "maximum model parameters in billions"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models [get]
func (h *adminAssetResourceAliasHandler) ListModel(ctx *gin.Context) { h.index(ctx) }

// ShowModel serves the shared model show operation for asset administration.
// @Security ApiKey
// @Summary Show model resources for asset administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models/{namespace}/{name} [get]
func (h *adminAssetResourceAliasHandler) ShowModel(ctx *gin.Context) { h.show(ctx) }

// UpdateModel serves the shared model update operation for asset administration.
// @Security ApiKey
// @Summary Update model resources for asset administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Accept json
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateModelReq true "body"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models/{namespace}/{name} [put]
func (h *adminAssetResourceAliasHandler) UpdateModel(ctx *gin.Context) { h.update(ctx) }

// SizeModel serves the shared model size operation for asset administration.
// @Security ApiKey
// @Summary Size model resources for asset administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        branch path string true "branch name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models/{namespace}/{name}/size/{branch} [get]
func (h *adminAssetResourceAliasHandler) SizeModel(ctx *gin.Context) { h.size(ctx) }

// createAdminAssetDatasetRoutes registers shared dataset repository operations.
func createAdminAssetDatasetRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, resource *handler.DatasetHandler, repo *handler.RepoHandler) {
	h := &adminAssetResourceAliasHandler{index: resource.Index, show: resource.Show, update: resource.Update, size: repo.GetRepoSizeByBranch}
	group := apiGroup.Group("/admin/asset/dataset/datasets", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetDataset), middleware.RepoType(types.DatasetRepo), middlewareCollection.Repo.RepoExists)
	group.GET("", h.ListDataset)
	group.GET("/:namespace/:name", h.ShowDataset)
	group.PUT("/:namespace/:name", h.UpdateDataset)
	group.GET("/:namespace/:name/size/:branch", h.SizeDataset)
}

// ListDataset serves the shared dataset list operation for asset administration.
// @Security ApiKey
// @Summary List dataset resources for asset administration
// @Description Requires asset/dataset administrator permission.
// @Tags Admin Asset Dataset
// @Produce json
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        xnet_migration_status query string false "filter by xnet migration status" Enums(pending, running, completed, failed)
// @Param        compliance_status query string false "filter by compliance status" Enums(compliant, pending_review, non_compliant)
// @Param        dataset_type query string false "filter by dataset type" Enums(commercial, normal)
// @Param        user_purchased query bool false "filter by user purchased" default(false)
// @Param        repo_size_min query int false "minimum repository size in bytes"
// @Param        repo_size_max query int false "maximum repository size in bytes"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/dataset/datasets [get]
func (h *adminAssetResourceAliasHandler) ListDataset(ctx *gin.Context) { h.index(ctx) }

// ShowDataset serves the shared dataset show operation for asset administration.
// @Security ApiKey
// @Summary Show dataset resources for asset administration
// @Description Requires asset/dataset administrator permission.
// @Tags Admin Asset Dataset
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/dataset/datasets/{namespace}/{name} [get]
func (h *adminAssetResourceAliasHandler) ShowDataset(ctx *gin.Context) { h.show(ctx) }

// UpdateDataset serves the shared dataset update operation for asset administration.
// @Security ApiKey
// @Summary Update dataset resources for asset administration
// @Description Requires asset/dataset administrator permission.
// @Tags Admin Asset Dataset
// @Accept json
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateDatasetReq true "body"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/dataset/datasets/{namespace}/{name} [put]
func (h *adminAssetResourceAliasHandler) UpdateDataset(ctx *gin.Context) { h.update(ctx) }

// SizeDataset serves the shared dataset size operation for asset administration.
// @Security ApiKey
// @Summary Size dataset resources for asset administration
// @Description Requires asset/dataset administrator permission.
// @Tags Admin Asset Dataset
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        branch path string true "branch name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/dataset/datasets/{namespace}/{name}/size/{branch} [get]
func (h *adminAssetResourceAliasHandler) SizeDataset(ctx *gin.Context) { h.size(ctx) }

// createAdminAssetCodeRoutes registers shared code repository operations.
func createAdminAssetCodeRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, resource *handler.CodeHandler, repo *handler.RepoHandler) {
	h := &adminAssetResourceAliasHandler{index: resource.Index, show: resource.Show, update: resource.Update, size: repo.GetRepoSizeByBranch}
	group := apiGroup.Group("/admin/asset/code/codes", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetCode), middleware.RepoType(types.CodeRepo), middlewareCollection.Repo.RepoExists)
	group.GET("", h.ListCode)
	group.GET("/:namespace/:name", h.ShowCode)
	group.PUT("/:namespace/:name", h.UpdateCode)
	group.GET("/:namespace/:name/size/:branch", h.SizeCode)
}

// ListCode serves the shared code list operation for asset administration.
// @Security ApiKey
// @Summary List code resources for asset administration
// @Description Requires asset/code administrator permission.
// @Tags Admin Asset Code
// @Produce json
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/code/codes [get]
func (h *adminAssetResourceAliasHandler) ListCode(ctx *gin.Context) { h.index(ctx) }

// ShowCode serves the shared code show operation for asset administration.
// @Security ApiKey
// @Summary Show code resources for asset administration
// @Description Requires asset/code administrator permission.
// @Tags Admin Asset Code
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/code/codes/{namespace}/{name} [get]
func (h *adminAssetResourceAliasHandler) ShowCode(ctx *gin.Context) { h.show(ctx) }

// UpdateCode serves the shared code update operation for asset administration.
// @Security ApiKey
// @Summary Update code resources for asset administration
// @Description Requires asset/code administrator permission.
// @Tags Admin Asset Code
// @Accept json
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateCodeReq true "body"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/code/codes/{namespace}/{name} [put]
func (h *adminAssetResourceAliasHandler) UpdateCode(ctx *gin.Context) { h.update(ctx) }

// SizeCode serves the shared code size operation for asset administration.
// @Security ApiKey
// @Summary Size code resources for asset administration
// @Description Requires asset/code administrator permission.
// @Tags Admin Asset Code
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        branch path string true "branch name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/code/codes/{namespace}/{name}/size/{branch} [get]
func (h *adminAssetResourceAliasHandler) SizeCode(ctx *gin.Context) { h.size(ctx) }

// createAdminAssetSpaceRoutes registers shared space repository operations.
func createAdminAssetSpaceRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, resource *handler.SpaceHandler, repo *handler.RepoHandler) {
	h := &adminAssetResourceAliasHandler{index: resource.Index, show: resource.Show, update: resource.Update, size: repo.GetRepoSizeByBranch}
	group := apiGroup.Group("/admin/asset/space/spaces", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSpace), middleware.RepoType(types.SpaceRepo), middlewareCollection.Repo.RepoExists)
	group.GET("", h.ListSpace)
	group.GET("/:namespace/:name", h.ShowSpace)
	group.PUT("/:namespace/:name", h.UpdateSpace)
	group.GET("/:namespace/:name/size/:branch", h.SizeSpace)
}

// ListSpace serves the shared space list operation for asset administration.
// @Security ApiKey
// @Summary List space resources for asset administration
// @Description Requires asset/space administrator permission.
// @Tags Admin Asset Space
// @Produce json
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        sdk query string false "filter by space sdk"
// @Param        status query string false "filter by space status" Enums(Building, BuildingFailed, Deploying, DeployFailed, Running, RuntimeError, Stopped, Sleeping, NoAppFile, NoNGINXConf)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/space/spaces [get]
func (h *adminAssetResourceAliasHandler) ListSpace(ctx *gin.Context) { h.index(ctx) }

// ShowSpace serves the shared space show operation for asset administration.
// @Security ApiKey
// @Summary Show space resources for asset administration
// @Description Requires asset/space administrator permission.
// @Tags Admin Asset Space
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/space/spaces/{namespace}/{name} [get]
func (h *adminAssetResourceAliasHandler) ShowSpace(ctx *gin.Context) { h.show(ctx) }

// UpdateSpace serves the shared space update operation for asset administration.
// @Security ApiKey
// @Summary Update space resources for asset administration
// @Description Requires asset/space administrator permission.
// @Tags Admin Asset Space
// @Accept json
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateSpaceReq true "body"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/space/spaces/{namespace}/{name} [put]
func (h *adminAssetResourceAliasHandler) UpdateSpace(ctx *gin.Context) { h.update(ctx) }

// SizeSpace serves the shared space size operation for asset administration.
// @Security ApiKey
// @Summary Size space resources for asset administration
// @Description Requires asset/space administrator permission.
// @Tags Admin Asset Space
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        branch path string true "branch name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/space/spaces/{namespace}/{name}/size/{branch} [get]
func (h *adminAssetResourceAliasHandler) SizeSpace(ctx *gin.Context) { h.size(ctx) }

// createAdminAssetMCPRoutes registers shared mcp repository operations.
func createAdminAssetMCPRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, resource *handler.MCPServerHandler, repo *handler.RepoHandler) {
	h := &adminAssetResourceAliasHandler{index: resource.Index, show: resource.Show, update: resource.Update, size: repo.GetRepoSizeByBranch}
	group := apiGroup.Group("/admin/asset/mcp/mcps", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetMCP), middleware.RepoType(types.MCPServerRepo), middlewareCollection.Repo.RepoExists)
	group.GET("", h.ListMCP)
	group.GET("/:namespace/:name", h.ShowMCP)
	group.PUT("/:namespace/:name", h.UpdateMCP)
	group.GET("/:namespace/:name/size/:branch", h.SizeMCP)
}

// ListMCP serves the shared mcp list operation for asset administration.
// @Security ApiKey
// @Summary List mcp resources for asset administration
// @Description Requires asset/mcp administrator permission.
// @Tags Admin Asset MCP
// @Produce json
// @Param        tag_category query string false "filter by tag category"
// @Param        tag_name query string false "filter by tag name"
// @Param        tag_group query string false "filter by tag group"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        search query string false "search text"
// @Param        sort query string false "sort by"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/mcp/mcps [get]
func (h *adminAssetResourceAliasHandler) ListMCP(ctx *gin.Context) { h.index(ctx) }

// ShowMCP serves the shared mcp show operation for asset administration.
// @Security ApiKey
// @Summary Show mcp resources for asset administration
// @Description Requires asset/mcp administrator permission.
// @Tags Admin Asset MCP
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/mcp/mcps/{namespace}/{name} [get]
func (h *adminAssetResourceAliasHandler) ShowMCP(ctx *gin.Context) { h.show(ctx) }

// UpdateMCP serves the shared mcp update operation for asset administration.
// @Security ApiKey
// @Summary Update mcp resources for asset administration
// @Description Requires asset/mcp administrator permission.
// @Tags Admin Asset MCP
// @Accept json
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateMCPServerReq true "body"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/mcp/mcps/{namespace}/{name} [put]
func (h *adminAssetResourceAliasHandler) UpdateMCP(ctx *gin.Context) { h.update(ctx) }

// SizeMCP serves the shared mcp size operation for asset administration.
// @Security ApiKey
// @Summary Size mcp resources for asset administration
// @Description Requires asset/mcp administrator permission.
// @Tags Admin Asset MCP
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        branch path string true "branch name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/mcp/mcps/{namespace}/{name}/size/{branch} [get]
func (h *adminAssetResourceAliasHandler) SizeMCP(ctx *gin.Context) { h.size(ctx) }

// createAdminAssetSkillRoutes registers shared skill repository operations.
func createAdminAssetSkillRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, resource *handler.SkillHandler, repo *handler.RepoHandler) {
	h := &adminAssetResourceAliasHandler{index: resource.Index, show: resource.Show, update: resource.Update, size: repo.GetRepoSizeByBranch}
	group := apiGroup.Group("/admin/asset/skill/skills", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSkill), middleware.RepoType(types.SkillRepo), middlewareCollection.Repo.RepoExists)
	group.GET("", h.ListSkill)
	group.GET("/:namespace/:name", h.ShowSkill)
	group.PUT("/:namespace/:name", h.UpdateSkill)
	group.GET("/:namespace/:name/size/:branch", h.SizeSkill)
}

// ListSkill serves the shared skill list operation for asset administration.
// @Security ApiKey
// @Summary List skill resources for asset administration
// @Description Requires asset/skill administrator permission.
// @Tags Admin Asset Skill
// @Produce json
// @Param        search query string false "search text"
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/skill/skills [get]
func (h *adminAssetResourceAliasHandler) ListSkill(ctx *gin.Context) { h.index(ctx) }

// ShowSkill serves the shared skill show operation for asset administration.
// @Security ApiKey
// @Summary Show skill resources for asset administration
// @Description Requires asset/skill administrator permission.
// @Tags Admin Asset Skill
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/skill/skills/{namespace}/{name} [get]
func (h *adminAssetResourceAliasHandler) ShowSkill(ctx *gin.Context) { h.show(ctx) }

// UpdateSkill serves the shared skill update operation for asset administration.
// @Security ApiKey
// @Summary Update skill resources for asset administration
// @Description Requires asset/skill administrator permission.
// @Tags Admin Asset Skill
// @Accept json
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateSkillReq true "body"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/skill/skills/{namespace}/{name} [put]
func (h *adminAssetResourceAliasHandler) UpdateSkill(ctx *gin.Context) { h.update(ctx) }

// SizeSkill serves the shared skill size operation for asset administration.
// @Security ApiKey
// @Summary Size skill resources for asset administration
// @Description Requires asset/skill administrator permission.
// @Tags Admin Asset Skill
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        branch path string true "branch name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/skill/skills/{namespace}/{name}/size/{branch} [get]
func (h *adminAssetResourceAliasHandler) SizeSkill(ctx *gin.Context) { h.size(ctx) }

// createAdminAssetTagReadRoutes registers shared operations for their owning administrator submodules.
func createAdminAssetTagReadRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, tags *handler.TagsHandler) {
	apiGroup.GET("/admin/asset/tag/tags", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetTag), (&adminSharedRouteHandler{target: tags.AllTags}).AssetTags)
	apiGroup.GET("/admin/asset/tag/tags/categories", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetTag), (&adminSharedRouteHandler{target: tags.AllCategories}).AssetTagCategories)
}

// createAdminAssetSyncExclusiveRoutes authorizes synchronization settings and queue reads independently of mirrors.
func createAdminAssetSyncExclusiveRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, settings *handler.SyncClientSettingHandler, mirror *handler.MirrorHandler) {
	apiGroup.GET("/sync/client_setting", middlewareCollection.License.Check, middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSync), settings.Show)
	apiGroup.POST("/sync/client_setting", middlewareCollection.License.Check, middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSync), settings.Create)
	// Register the queue outside the mirror group so its callers only need asset/sync.
	apiGroup.GET("/mirror/repos", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSync), mirror.Repos)
}

// adminMirrorResource binds an administrator repository path to its server-side resource type.
type adminMirrorResource struct {
	path     string
	repoType types.RepositoryType
}

// adminMirrorResources lists the repository types supported by mirror and synchronization administration.
func adminMirrorResources() []adminMirrorResource {
	return []adminMirrorResource{
		{"models", types.ModelRepo},
		{"datasets", types.DatasetRepo},
		{"codes", types.CodeRepo},
		{"spaces", types.SpaceRepo},
		{"mcps", types.MCPServerRepo},
		{"skills", types.SkillRepo},
	}
}

// createAdminAssetMirrorRoutes separates mirror administration from the ordinary repository permission gates.
func createAdminAssetMirrorRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, repo *handler.RepoHandler) {
	group := apiGroup.Group("/admin/asset/mirror", mc.Auth.PlatformAdmin(types.AdminAssetMirror))
	for _, resource := range adminMirrorResources() {
		mirror := group.Group("/"+resource.path+"/:namespace/:name/mirror", middleware.RepoType(resource.repoType), mc.Repo.RepoExists)
		mirror.GET("", (&adminSharedRouteHandler{target: repo.GetMirror}).AssetMirrorGet)
		mirror.POST("", (&adminSharedRouteHandler{target: repo.CreateMirror}).AssetMirrorCreate)
		mirror.PUT("", (&adminSharedRouteHandler{target: repo.UpdateMirror}).AssetMirrorUpdate)
		mirror.DELETE("", (&adminSharedRouteHandler{target: repo.DeleteMirror}).AssetMirrorDelete)
		mirror.POST("/sync", (&adminSharedRouteHandler{target: repo.SyncMirror}).AssetMirrorSync)
	}
}

// createAdminAssetSyncRoutes exposes shared synchronization actions under their own administrator permission.
func createAdminAssetSyncRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, syncMirror, cancelSync gin.HandlerFunc) {
	group := apiGroup.Group("/admin/asset/sync", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSync))
	retry := &adminSharedRouteHandler{target: syncMirror}
	for _, resource := range adminMirrorResources() {
		group.POST("/"+resource.path+"/:namespace/:name/mirror/sync", middleware.RepoType(resource.repoType), middlewareCollection.Repo.RepoExists, retry.AssetSyncMirror)
	}
	group.POST("/lfs_sync/cancel", (&adminSharedRouteHandler{target: cancelSync}).AssetSyncCancel)
}

// AssetSyncMirror delegates synchronization retries to the existing repository handler.
// @Security ApiKey
// @Summary Retry repository synchronization from multi-source synchronization administration
// @Description Requires asset/sync administrator permission. Original repository checks and scheduling behavior are preserved.
// @Tags Admin Asset Sync
// @Accept json
// @Produce json
// @Param namespace path string true "Repository owner name"
// @Param name path string true "Repository name"
// @Param body body types.SyncMirrorParams false "Scheduling options"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/sync/models/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/sync/datasets/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/sync/codes/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/sync/spaces/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/sync/mcps/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/sync/skills/{namespace}/{name}/mirror/sync [post]
func (h *adminSharedRouteHandler) AssetSyncMirror(ctx *gin.Context) { h.target(ctx) }

// AssetSyncCancel delegates cancellation to the original LFS synchronization service endpoint.
// @Security ApiKey
// @Summary Cancel a task from multi-source synchronization administration
// @Description Requires asset/sync administrator permission. The request is forwarded to /api/v1/lfs_sync/cancel.
// @Tags Admin Asset Sync
// @Accept json
// @Produce json
// @Param body body map[string]int64 true "Task to cancel, identified by task_id"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} map[string]string "Invalid cancellation request"
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} map[string]string "Synchronization task not found"
// @Failure 500 {object} map[string]string "Cancellation failed"
// @Router /admin/asset/sync/lfs_sync/cancel [post]
func (h *adminSharedRouteHandler) AssetSyncCancel(ctx *gin.Context) { h.target(ctx) }

// createAdminModelCrossRoutes registers shared operations for their owning administrator submodules.
func createAdminModelCrossRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, model *handler.ModelHandler, repo *handler.RepoHandler) {
	apiGroup.GET("/admin/asset/model/models/:namespace/:name/serverless", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetModel), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: model.GetDeployServerless}).AssetModelServerless)
	apiGroup.GET("/admin/asset/model/models/:namespace/:name/mirror", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetModel), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: repo.GetMirror}).AssetModelMirror)
	apiGroup.POST("/admin/asset/model/models/:namespace/:name/mirror/sync", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetModel), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: repo.SyncMirror}).AssetModelMirrorSync)
	apiGroup.GET("/admin/apigateway/public_inference/models/:namespace/:name", middlewareCollection.Auth.PlatformAdmin(types.AdminAPIGatewayPublicInference), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: model.Show}).PublicInferenceModel)
	apiGroup.GET("/admin/apigateway/public_inference/models/:namespace/:name/serverless", middlewareCollection.Auth.PlatformAdmin(types.AdminAPIGatewayPublicInference), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: model.GetDeployServerless}).PublicInferenceServerless)
	apiGroup.GET("/admin/apigateway/public_inference/models/:namespace/:name/runtime_framework", middlewareCollection.Auth.PlatformAdmin(types.AdminAPIGatewayPublicInference), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: repo.RuntimeFrameworkList}).PublicInferenceRuntime)
	apiGroup.GET("/admin/apigateway/public_inference/models/:namespace/:name/quantizations", middlewareCollection.Auth.PlatformAdmin(types.AdminAPIGatewayPublicInference), middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: model.ListQuantizations}).PublicInferenceQuantizations)
}

// createAdminMCPToolsRoute registers shared operations for their owning administrator submodules.
func createAdminMCPToolsRoute(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, mcp *handler.MCPServerHandler) {
	apiGroup.GET("/admin/asset/mcp/mcps/tools", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetMCP), middleware.RepoType(types.MCPServerRepo), middlewareCollection.Repo.RepoExists, (&adminSharedRouteHandler{target: mcp.Properties}).AssetMCPTools)
}

// AssetTags serves the shared /tags operation for asset/tag.
// @Security ApiKey
// @Summary Get /tags for asset/tag administration
// @Description Requires asset/tag administrator permission.
// @Tags Admin Asset Tag
// @Produce json
// @Param		 category query string false "category name"
// @Param		 scope query string false "scope name" Enums(model, dataset, code, space, prompt, skill)
// @Param		 built_in query bool false "built_in"
// @Param		 search query string false "search on name and show_name fields"
// @Param		 per query int false "Page size, default 50, max 100. When omitted and scope is set, all tags are returned without pagination" default(50)
// @Param		 page query int false "Page number, default 1" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/tag/tags [get]
func (h *adminSharedRouteHandler) AssetTags(ctx *gin.Context) { h.target(ctx) }

// AssetTagCategories serves the shared /tags/categories operation for asset/tag.
// @Security ApiKey
// @Summary Get /tags/categories for asset/tag administration
// @Description Requires asset/tag administrator permission.
// @Tags Admin Asset Tag
// @Produce json
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/tag/tags/categories [get]
func (h *adminSharedRouteHandler) AssetTagCategories(ctx *gin.Context) { h.target(ctx) }

// AssetModelServerless serves the shared /models/:namespace/:name/serverless operation for asset/model.
// @Security ApiKey
// @Summary Get /models/:namespace/:name/serverless for asset/model administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models/{namespace}/{name}/serverless [get]
func (h *adminSharedRouteHandler) AssetModelServerless(ctx *gin.Context) { h.target(ctx) }

// AssetModelMirror serves the shared /models/:namespace/:name/mirror operation for asset/model.
// @Security ApiKey
// @Summary Get /models/:namespace/:name/mirror for asset/model administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models/{namespace}/{name}/mirror [get]
func (h *adminSharedRouteHandler) AssetModelMirror(ctx *gin.Context) { h.target(ctx) }

// AssetModelMirrorSync serves the shared /models/:namespace/:name/mirror/sync operation for asset/model.
// @Security ApiKey
// @Summary Post /models/:namespace/:name/mirror/sync for asset/model administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Accept json
// @Produce json
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        body body types.SyncMirrorParams false "scheduling options"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models/{namespace}/{name}/mirror/sync [post]
func (h *adminSharedRouteHandler) AssetModelMirrorSync(ctx *gin.Context) { h.target(ctx) }

// AssetMirrorGet delegates the independently authorized mirror operation to the shared repository handler.
// @Security ApiKey
// @Summary Get mirror configuration for administration
// @Description Requires asset/mirror permission. Source validation and shared business constraints are preserved.
// @Tags Admin Asset Mirror
// @Produce json
// @Param namespace path string true "Repository owner name"
// @Param name path string true "Repository name"
// @Success 200 {object} types.Response{data=types.Mirror}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/mirror/models/{namespace}/{name}/mirror [get]
// @Router /admin/asset/mirror/datasets/{namespace}/{name}/mirror [get]
// @Router /admin/asset/mirror/codes/{namespace}/{name}/mirror [get]
// @Router /admin/asset/mirror/spaces/{namespace}/{name}/mirror [get]
// @Router /admin/asset/mirror/mcps/{namespace}/{name}/mirror [get]
// @Router /admin/asset/mirror/skills/{namespace}/{name}/mirror [get]
func (h *adminSharedRouteHandler) AssetMirrorGet(ctx *gin.Context) { h.target(ctx) }

// AssetMirrorCreate delegates the independently authorized mirror operation to the shared repository handler.
// @Security ApiKey
// @Summary Create mirror configuration for administration
// @Description Requires asset/mirror permission. Source validation and shared business constraints are preserved.
// @Tags Admin Asset Mirror
// @Accept json
// @Produce json
// @Param namespace path string true "Repository owner name"
// @Param name path string true "Repository name"
// @Param body body types.CreateMirrorParams true "Mirror operation parameters"
// @Success 200 {object} types.Response{data=database.Mirror}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 409 {object} httpbase.R "Target repository already has a different mirror source"
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/mirror/models/{namespace}/{name}/mirror [post]
// @Router /admin/asset/mirror/datasets/{namespace}/{name}/mirror [post]
// @Router /admin/asset/mirror/codes/{namespace}/{name}/mirror [post]
// @Router /admin/asset/mirror/spaces/{namespace}/{name}/mirror [post]
// @Router /admin/asset/mirror/mcps/{namespace}/{name}/mirror [post]
// @Router /admin/asset/mirror/skills/{namespace}/{name}/mirror [post]
func (h *adminSharedRouteHandler) AssetMirrorCreate(ctx *gin.Context) { h.target(ctx) }

// AssetMirrorUpdate delegates the independently authorized mirror operation to the shared repository handler.
// @Security ApiKey
// @Summary Update mirror configuration for administration
// @Description Requires asset/mirror permission. Source validation and shared business constraints are preserved.
// @Tags Admin Asset Mirror
// @Accept json
// @Produce json
// @Param namespace path string true "Repository owner name"
// @Param name path string true "Repository name"
// @Param body body types.UpdateMirrorParams true "Mirror operation parameters"
// @Success 200 {object} types.Response{data=database.Mirror}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/mirror/models/{namespace}/{name}/mirror [put]
// @Router /admin/asset/mirror/datasets/{namespace}/{name}/mirror [put]
// @Router /admin/asset/mirror/codes/{namespace}/{name}/mirror [put]
// @Router /admin/asset/mirror/spaces/{namespace}/{name}/mirror [put]
// @Router /admin/asset/mirror/mcps/{namespace}/{name}/mirror [put]
// @Router /admin/asset/mirror/skills/{namespace}/{name}/mirror [put]
func (h *adminSharedRouteHandler) AssetMirrorUpdate(ctx *gin.Context) { h.target(ctx) }

// AssetMirrorDelete delegates the independently authorized mirror operation to the shared repository handler.
// @Security ApiKey
// @Summary Delete mirror configuration for administration
// @Description Requires asset/mirror permission. Source validation and shared business constraints are preserved.
// @Tags Admin Asset Mirror
// @Produce json
// @Param namespace path string true "Repository owner name"
// @Param name path string true "Repository name"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/mirror/models/{namespace}/{name}/mirror [delete]
// @Router /admin/asset/mirror/datasets/{namespace}/{name}/mirror [delete]
// @Router /admin/asset/mirror/codes/{namespace}/{name}/mirror [delete]
// @Router /admin/asset/mirror/spaces/{namespace}/{name}/mirror [delete]
// @Router /admin/asset/mirror/mcps/{namespace}/{name}/mirror [delete]
// @Router /admin/asset/mirror/skills/{namespace}/{name}/mirror [delete]
func (h *adminSharedRouteHandler) AssetMirrorDelete(ctx *gin.Context) { h.target(ctx) }

// AssetMirrorSync delegates the independently authorized mirror operation to the shared repository handler.
// @Security ApiKey
// @Summary Synchronize a repository mirror for administration
// @Description Requires asset/mirror permission. Source validation and shared business constraints are preserved.
// @Tags Admin Asset Mirror
// @Accept json
// @Produce json
// @Param namespace path string true "Repository owner name"
// @Param name path string true "Repository name"
// @Param body body types.SyncMirrorParams false "Mirror operation parameters"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/mirror/models/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/mirror/datasets/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/mirror/codes/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/mirror/spaces/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/mirror/mcps/{namespace}/{name}/mirror/sync [post]
// @Router /admin/asset/mirror/skills/{namespace}/{name}/mirror/sync [post]
func (h *adminSharedRouteHandler) AssetMirrorSync(ctx *gin.Context) { h.target(ctx) }

// PublicInferenceModel serves the shared /models/:namespace/:name operation for apigateway/public_inference.
// @Security ApiKey
// @Summary Get /models/:namespace/:name for apigateway/public_inference administration
// @Description Requires apigateway/public_inference administrator permission.
// @Tags Admin Apigateway Public Inference
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/apigateway/public_inference/models/{namespace}/{name} [get]
func (h *adminSharedRouteHandler) PublicInferenceModel(ctx *gin.Context) { h.target(ctx) }

// PublicInferenceServerless serves the shared /models/:namespace/:name/serverless operation for apigateway/public_inference.
// @Security ApiKey
// @Summary Get /models/:namespace/:name/serverless for apigateway/public_inference administration
// @Description Requires apigateway/public_inference administrator permission.
// @Tags Admin Apigateway Public Inference
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/apigateway/public_inference/models/{namespace}/{name}/serverless [get]
func (h *adminSharedRouteHandler) PublicInferenceServerless(ctx *gin.Context) { h.target(ctx) }

// PublicInferenceRuntime serves the shared /models/:namespace/:name/runtime_framework operation for apigateway/public_inference.
// @Security ApiKey
// @Summary Get /models/:namespace/:name/runtime_framework for apigateway/public_inference administration
// @Description Requires apigateway/public_inference administrator permission.
// @Tags Admin Apigateway Public Inference
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2, 4) default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/apigateway/public_inference/models/{namespace}/{name}/runtime_framework [get]
func (h *adminSharedRouteHandler) PublicInferenceRuntime(ctx *gin.Context) { h.target(ctx) }

// PublicInferenceQuantizations serves the shared /models/:namespace/:name/quantizations operation for apigateway/public_inference.
// @Security ApiKey
// @Summary Get /models/:namespace/:name/quantizations for apigateway/public_inference administration
// @Description Requires apigateway/public_inference administrator permission.
// @Tags Admin Apigateway Public Inference
// @Produce json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/apigateway/public_inference/models/{namespace}/{name}/quantizations [get]
func (h *adminSharedRouteHandler) PublicInferenceQuantizations(ctx *gin.Context) { h.target(ctx) }

// AssetMCPTools serves the shared /mcps/tools operation for asset/mcp.
// @Security ApiKey
// @Summary Get /mcps/tools for asset/mcp administration
// @Description Requires asset/mcp administrator permission.
// @Tags Admin Asset Mcp
// @Produce json
// @Param        search query string false "search text"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/mcp/mcps/tools [get]
func (h *adminSharedRouteHandler) AssetMCPTools(ctx *gin.Context) { h.target(ctx) }

// adminAssetChangePathHandler binds a resource type before reusing path changes.
type adminAssetChangePathHandler struct {
	change gin.HandlerFunc
}

// createAdminAssetChangePathRoutes grants model and dataset path changes independently.
func createAdminAssetChangePathRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, repo *handler.RepoHandler) {
	h := &adminAssetChangePathHandler{change: repo.ChangePath}
	apiGroup.POST("/admin/asset/model/models/:namespace/:name/change_path", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetModel), h.Model)
	apiGroup.POST("/admin/asset/dataset/datasets/:namespace/:name/change_path", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetDataset), h.Dataset)
}

// Model changes a model repository path through the shared handler.
// @Security ApiKey
// @Summary Change a model path for asset administration
// @Description Requires asset/model administrator permission.
// @Tags Admin Asset Model
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace"
// @Param name path string true "Repository name"
// @Param body body types.ChangePathReq true "Path change"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/model/models/{namespace}/{name}/change_path [post]
func (h *adminAssetChangePathHandler) Model(ctx *gin.Context) {
	ctx.Params = append(ctx.Params, gin.Param{Key: "repo_type", Value: "models"})
	h.change(ctx)
}

// Dataset changes a dataset repository path through the shared handler.
// @Security ApiKey
// @Summary Change a dataset path for asset administration
// @Description Requires asset/dataset administrator permission.
// @Tags Admin Asset Dataset
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace"
// @Param name path string true "Repository name"
// @Param body body types.ChangePathReq true "Path change"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/dataset/datasets/{namespace}/{name}/change_path [post]
func (h *adminAssetChangePathHandler) Dataset(ctx *gin.Context) {
	ctx.Params = append(ctx.Params, gin.Param{Key: "repo_type", Value: "datasets"})
	h.change(ctx)
}

// ComputeNodeHardware serves /space_resources/hardware_types for compute/node administration.
// @Security ApiKey
// @Summary List space_resources/hardware_types for compute/node administration
// @Description Requires compute/node administrator permission.
// @Tags Admin compute/node
// @Produce json
// @Param cluster_id query string false "cluster_id"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/node/space_resources/hardware_types [get]
func (h *adminSharedRouteHandler) ComputeNodeHardware(ctx *gin.Context) { h.target(ctx) }

// ComputeNodeCluster serves /cluster for compute/node administration.
// @Security ApiKey
// @Summary List cluster for compute/node administration
// @Description Requires compute/node administrator permission.
// @Tags Admin compute/node
// @Produce json
// @Param        scope  query  types.ClusterScope  false  "Scope of cluster"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/compute/node/cluster [get]
func (h *adminSharedRouteHandler) ComputeNodeCluster(ctx *gin.Context) { h.target(ctx) }

// createAdminRuntimeFrameworkRoute exposes shared operations with the calling administrator permission.
func createAdminRuntimeFrameworkRoute(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, model *handler.ModelHandler) {
	apiGroup.GET("/admin/asset/runtime_framework/runtime_framework", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetRuntimeFramework), (&adminSharedRouteHandler{target: model.ListAllRuntimeFramework}).AssetRuntimeFrameworks)
}

// AssetRuntimeFrameworks serves /runtime_framework for asset/runtime_framework administration.
// @Security ApiKey
// @Summary List runtime_framework for asset/runtime_framework administration
// @Description Requires asset/runtime_framework administrator permission.
// @Tags Admin asset/runtime_framework
// @Produce json
// @Param 		 deploy_type query int false "deploy_type" Enums(1, 2, 3, 4, 5) default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/asset/runtime_framework/runtime_framework [get]
func (h *adminSharedRouteHandler) AssetRuntimeFrameworks(ctx *gin.Context) { h.target(ctx) }

// createAdminMarketingCodeRoute exposes shared operations with the calling administrator permission.
func createAdminMarketingCodeRoute(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, code *handler.CodeHandler) {
	apiGroup.GET("/admin/marketing/eai_article/codes", middlewareCollection.Auth.PlatformAdmin(types.AdminMarketingEAIArticle), middleware.RepoType(types.CodeRepo), (&adminSharedRouteHandler{target: code.Index}).MarketingArticleCodes)
}

// MarketingArticleCodes serves /codes for marketing/eai_article administration.
// @Security ApiKey
// @Summary List codes for marketing/eai_article administration
// @Description Requires marketing/eai_article administrator permission.
// @Tags Admin marketing/eai_article
// @Produce json
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Router /admin/marketing/eai_article/codes [get]
func (h *adminSharedRouteHandler) MarketingArticleCodes(ctx *gin.Context) { h.target(ctx) }

// createAdminAssetRecommendationRoutes fixes both permission and repository type for each asset submodule.
func createAdminAssetRecommendationRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, setOpWeight func(types.RepositoryType) gin.HandlerFunc) {
	model := &adminSharedRouteHandler{target: setOpWeight(types.ModelRepo)}
	dataset := &adminSharedRouteHandler{target: setOpWeight(types.DatasetRepo)}
	code := &adminSharedRouteHandler{target: setOpWeight(types.CodeRepo)}
	space := &adminSharedRouteHandler{target: setOpWeight(types.SpaceRepo)}
	mcp := &adminSharedRouteHandler{target: setOpWeight(types.MCPServerRepo)}
	skill := &adminSharedRouteHandler{target: setOpWeight(types.SkillRepo)}
	apiGroup.POST("/admin/asset/model/recom/opweight", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetModel), middlewareCollection.License.Check, model.AssetModelRecommendationWeight)
	apiGroup.POST("/admin/asset/dataset/recom/opweight", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetDataset), middlewareCollection.License.Check, dataset.AssetDatasetRecommendationWeight)
	apiGroup.POST("/admin/asset/code/recom/opweight", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetCode), middlewareCollection.License.Check, code.AssetCodeRecommendationWeight)
	apiGroup.POST("/admin/asset/space/recom/opweight", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSpace), middlewareCollection.License.Check, space.AssetSpaceRecommendationWeight)
	apiGroup.POST("/admin/asset/mcp/recom/opweight", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetMCP), middlewareCollection.License.Check, mcp.AssetMCPRecommendationWeight)
	apiGroup.POST("/admin/asset/skill/recom/opweight", middlewareCollection.Auth.PlatformAdmin(types.AdminAssetSkill), middlewareCollection.License.Check, skill.AssetSkillRecommendationWeight)
}

// AssetModelRecommendationWeight documents recommendation updates for model administration.
// @Security ApiKey
// @Summary Update recommendation weight for model administration
// @Description Requires asset/model permission and a target repository whose stored type is model; a type mismatch returns 403 without updating weights.
// @Tags Admin Asset Model
// @Accept json
// @Produce json
// @Param body body handler.SetOpWeight.SetOpWeightReq true "Repository ID and recommendation weight"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/model/recom/opweight [post]
func (h *adminSharedRouteHandler) AssetModelRecommendationWeight(ctx *gin.Context) { h.target(ctx) }

// AssetDatasetRecommendationWeight documents recommendation updates for dataset administration.
// @Security ApiKey
// @Summary Update recommendation weight for dataset administration
// @Description Requires asset/dataset permission and a target repository whose stored type is dataset; a type mismatch returns 403 without updating weights.
// @Tags Admin Asset Dataset
// @Accept json
// @Produce json
// @Param body body handler.SetOpWeight.SetOpWeightReq true "Repository ID and recommendation weight"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/dataset/recom/opweight [post]
func (h *adminSharedRouteHandler) AssetDatasetRecommendationWeight(ctx *gin.Context) { h.target(ctx) }

// AssetCodeRecommendationWeight documents recommendation updates for code administration.
// @Security ApiKey
// @Summary Update recommendation weight for code administration
// @Description Requires asset/code permission and a target repository whose stored type is code; a type mismatch returns 403 without updating weights.
// @Tags Admin Asset Code
// @Accept json
// @Produce json
// @Param body body handler.SetOpWeight.SetOpWeightReq true "Repository ID and recommendation weight"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/code/recom/opweight [post]
func (h *adminSharedRouteHandler) AssetCodeRecommendationWeight(ctx *gin.Context) { h.target(ctx) }

// AssetSpaceRecommendationWeight documents recommendation updates for space administration.
// @Security ApiKey
// @Summary Update recommendation weight for space administration
// @Description Requires asset/space permission and a target repository whose stored type is space; a type mismatch returns 403 without updating weights.
// @Tags Admin Asset Space
// @Accept json
// @Produce json
// @Param body body handler.SetOpWeight.SetOpWeightReq true "Repository ID and recommendation weight"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/space/recom/opweight [post]
func (h *adminSharedRouteHandler) AssetSpaceRecommendationWeight(ctx *gin.Context) { h.target(ctx) }

// AssetMCPRecommendationWeight documents recommendation updates for MCP administration.
// @Security ApiKey
// @Summary Update recommendation weight for MCP administration
// @Description Requires asset/mcp permission and a target repository whose stored type is mcpserver; a type mismatch returns 403 without updating weights.
// @Tags Admin Asset MCP
// @Accept json
// @Produce json
// @Param body body handler.SetOpWeight.SetOpWeightReq true "Repository ID and recommendation weight"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/mcp/recom/opweight [post]
func (h *adminSharedRouteHandler) AssetMCPRecommendationWeight(ctx *gin.Context) { h.target(ctx) }

// AssetSkillRecommendationWeight documents recommendation updates for skill administration.
// @Security ApiKey
// @Summary Update recommendation weight for skill administration
// @Description Requires asset/skill permission and a target repository whose stored type is skill; a type mismatch returns 403 without updating weights.
// @Tags Admin Asset Skill
// @Accept json
// @Produce json
// @Param body body handler.SetOpWeight.SetOpWeightReq true "Repository ID and recommendation weight"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/skill/recom/opweight [post]
func (h *adminSharedRouteHandler) AssetSkillRecommendationWeight(ctx *gin.Context) { h.target(ctx) }

// createAdminCommonJWTRoutes forwards session refresh to the independently authorized user service.
func createAdminCommonJWTRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, user *handler.InternalServiceProxyHandler) {
	apiGroup.POST("/admin/common/jwt/token", mc.Auth.PlatformAdmin(types.AdminPermissionAny), (&adminSharedRouteHandler{target: user.ProxyToApi("/api/v1/admin/common/jwt/token")}).CommonRefreshJWT)
}

// createAdminCommonOrganizationRoutes preserves personal organization scope for administrator navigation.
// Callers register this entry only when the edition's hierarchy feature is enabled.
func createAdminCommonOrganizationRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, user *handler.InternalServiceProxyHandler) {
	apiGroup.GET("/admin/common/organizations/mine", mc.Auth.PlatformAdmin(types.AdminPermissionAny), (&adminSharedRouteHandler{target: user.ProxyToApi("/api/v1/admin/common/organizations/mine")}).CommonOrganizationsMine)
}

// CommonRefreshJWT refreshes the current administrator's existing session through the user service.
// @Security ApiKey
// @Summary Refresh the current administrator JWT
// @Description Requires active administrator membership in both services. The authenticated JWT must belong to the requested user; existing edition-specific expiry rules are preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.RefreshJWTReq true "Current user UUID and existing JWT"
// @Success 200 {object} types.Response{data=types.CreateJWTResp}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/jwt/token [post]
func (h *adminSharedRouteHandler) CommonRefreshJWT(ctx *gin.Context) { h.target(ctx) }

// CommonOrganizationsMine lists the caller's organizations without bypassing membership checks.
// @Security ApiKey
// @Summary List my organizations for administrator navigation
// @Description Requires active administrator membership in both services and the enabled hierarchy feature. Existing organization membership filters remain in effect.
// @Tags Admin Common
// @Produce json
// @Success 200 {object} types.Response{data=object}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/organizations/mine [get]
func (h *adminSharedRouteHandler) CommonOrganizationsMine(ctx *gin.Context) { h.target(ctx) }

// TokenPlanNamespaces resolves namespace display names for subscription administration.
// @Security ApiKey
// @Summary Resolve namespace names for subscription plans
// @Description Requires apigateway/token_plan administrator permission; the User service independently verifies the operator.
// @Tags Admin APIGateway Token Plan
// @Accept json
// @Produce json
// @Param uuids body []string true "Namespace UUIDs"
// @Success 200 {object} types.Response{data=[]types.Namespace}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/namespaces/lookup [post]
func (h *adminSharedRouteHandler) TokenPlanNamespaces(ctx *gin.Context) { h.target(ctx) }
