package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/user/handler"
)

// adminUserRouteHandler documents administrator operations that reuse existing user handlers.
type adminUserRouteHandler struct{ target gin.HandlerFunc }

// createAdminUserRoutes binds shared user operations to their calling submodule permissions.
func createAdminUserRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, users *handler.UserHandler) {
	apiGroup.DELETE("/user/:username", mc.Auth.PlatformAdmin(types.AdminAccountUser), users.Delete)
	apiGroup.PUT("/user/labels", mc.Auth.PlatformAdmin(types.AdminAccountUser), users.UpdateUserLabels)
	apiGroup.GET("/users/stream-export", mc.Auth.PlatformAdmin(types.AdminAccountUser), users.ExportUserInfo)
	apiGroup.PUT("/user/verify/:id", mc.Auth.PlatformAdmin(types.AdminAccountUser), users.UpdateVerify)
	account := apiGroup.Group("/admin/account/user", mc.Auth.PlatformAdmin(types.AdminAccountUser))
	account.GET("/users", (&adminUserRouteHandler{target: users.Index}).AccountUsers)
	apiGroup.GET("/admin/account/admin/users/search", mc.Auth.PlatformAdmin(types.AdminAccountAdmin), (&adminUserRouteHandler{target: users.Index}).AdminCandidateUsers)
	account.GET("/user/:identifier", (&adminUserRouteHandler{target: users.Get}).AccountUser)
	account.PUT("/user/:identifier", (&adminUserRouteHandler{target: users.Update}).UpdateAccountUser)
	account.GET("/user/verify/:id", (&adminUserRouteHandler{target: users.GetVerify}).UserVerification)
	apiGroup.GET("/admin/account/organization/users", mc.Auth.PlatformAdmin(types.AdminAccountOrganization), (&adminUserRouteHandler{target: users.UserChoices}).OrganizationUsers)
	apiGroup.GET("/admin/compute/node/users", mc.Auth.PlatformAdmin(types.AdminComputeNode), (&adminUserRouteHandler{target: users.UserChoices}).NodeUsers)
	apiGroup.GET("/admin/asset/model/users", mc.Auth.PlatformAdmin(types.AdminAssetModel), (&adminUserRouteHandler{target: users.UserChoices}).ModelUsers)
	apiGroup.GET("/admin/asset/dataset/users", mc.Auth.PlatformAdmin(types.AdminAssetDataset), (&adminUserRouteHandler{target: users.UserChoices}).DatasetUsers)
	apiGroup.GET("/admin/academy/user_permission/users", mc.Auth.PlatformAdmin(types.AdminAcademyUserPermission), (&adminUserRouteHandler{target: users.UserChoices}).AcademyUsers)
}

// createAdminCommonUserRoutes independently checks membership while preserving profile business authorization.
func createAdminCommonUserRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, users *handler.UserHandler) {
	common := apiGroup.Group("/admin/common/user", mc.Auth.PlatformAdmin(types.AdminPermissionAny))
	common.GET("/:identifier", (&adminUserRouteHandler{target: users.Get}).CommonUser)
	common.PUT("/:identifier", (&adminUserRouteHandler{target: users.Update}).CommonUpdateUser)
	common.POST("/tags", (&adminUserRouteHandler{target: users.ResetUserTags}).CommonUserTags)
	common.POST("/email-verification-code/:email", (&adminUserRouteHandler{target: users.GenerateVerificationCodeAndSendEmail}).CommonEmailVerificationCode)
}

// createAdminOrganizationVerificationRoutes separates administration from personal verification reads.
func createAdminOrganizationVerificationRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, organizations *handler.OrganizationHandler) {
	apiGroup.PUT("/organization/verify/:id", mc.Auth.PlatformAdmin(types.AdminAccountOrganization), organizations.UpdateVerify)
	apiGroup.GET("/admin/account/organization/organization/verify/:namespace", mc.Auth.PlatformAdmin(types.AdminAccountOrganization), (&adminUserRouteHandler{target: organizations.GetVerify}).OrganizationVerification)
}

// createAdminOrganizationQueryRoutes independently authorizes organization reads for each calling submodule.
func createAdminOrganizationQueryRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, organizations *handler.OrganizationHandler) {
	account := apiGroup.Group("/admin/account/organization", mc.Auth.PlatformAdmin(types.AdminAccountOrganization))
	account.GET("/organizations", (&adminUserRouteHandler{target: organizations.Index}).AccountOrganizations)
	account.GET("/organization/:name", (&adminUserRouteHandler{target: organizations.Get}).AccountOrganization)
	apiGroup.GET("/admin/compute/node/organizations", mc.Auth.PlatformAdmin(types.AdminComputeNode), (&adminUserRouteHandler{target: organizations.OrganizationChoices}).NodeOrganizations)
}

// AccountOrganizations lists organizations after independent account/organization authorization.
// @Security ApiKey
// @Summary List organizations for account administration
// @Description Requires account/organization administrator permission independently checked by the user service. The response data contains the organization list and total count.
// @Tags Admin account organization
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Organization name or path search"
// @Param org_type query string false "Organization type filter"
// @Param verify_status query string false "Verification status filter"
// @Param tag query string false "Tag name filter"
// @Success 200 {object} types.Response{data=object}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/organization/organizations [get]
func (h *adminUserRouteHandler) AccountOrganizations(ctx *gin.Context) { h.target(ctx) }

// AccountOrganization reads an organization after independent account/organization authorization.
// @Security ApiKey
// @Summary Get an organization for account administration
// @Description Requires account/organization administrator permission independently checked by the user service.
// @Tags Admin account organization
// @Produce json
// @Param name path string true "Organization namespace"
// @Success 200 {object} types.Response{data=types.Organization}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/organization/organization/{name} [get]
func (h *adminUserRouteHandler) AccountOrganization(ctx *gin.Context) {
	ctx.Params = append(ctx.Params, gin.Param{Key: "namespace", Value: ctx.Param("name")})
	h.target(ctx)
}

// NodeOrganizations lists organization choices after independent compute/node authorization.
// @Security ApiKey
// @Summary List organizations for compute node administration
// @Description Requires compute/node administrator permission independently checked by the user service. The response data contains the organization list and total count. Only selector identity, display fields, and namespace identifiers are returned; contact details, roles, verification metadata, and tags are excluded.
// @Tags Admin compute node
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Organization name or path search"
// @Param org_type query string false "Organization type filter"
// @Param verify_status query string false "Verification status filter"
// @Param tag query string false "Tag name filter"
// @Success 200 {object} types.Response{data=types.OrganizationChoiceList}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/compute/node/organizations [get]
func (h *adminUserRouteHandler) NodeOrganizations(ctx *gin.Context) { h.target(ctx) }

// UserVerification reads verification details after user administration authorization.
// @Security ApiKey
// @Summary Get user verification for administration
// @Description Requires account/user permission, independently checked by the user service.
// @Tags Admin account user
// @Produce json
// @Param id path string true "User UUID"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/user/user/verify/{id} [get]
func (h *adminUserRouteHandler) UserVerification(ctx *gin.Context) { h.target(ctx) }

// OrganizationVerification reads verification details after organization administration authorization.
// @Security ApiKey
// @Summary Get organization verification for administration
// @Description Requires account/organization permission, independently checked by the user service.
// @Tags Admin account organization
// @Produce json
// @Param namespace path string true "Organization namespace"
// @Success 200 {object} types.Response{}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/organization/organization/verify/{namespace} [get]
func (h *adminUserRouteHandler) OrganizationVerification(ctx *gin.Context) { h.target(ctx) }

// createAdminOrganizationCreateRoute uses the configured organization model after module authorization.
func createAdminOrganizationCreateRoute(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, create gin.HandlerFunc) {
	apiGroup.POST("/admin/account/organization/organizations", mc.Auth.PlatformAdmin(types.AdminAccountOrganization), (&adminUserRouteHandler{target: create}).CreateOrganization)
}

// AccountUsers lists users after independent account/user authorization.
// @Security ApiKey
// @Summary Lists users for account/user administration
// @Description Requires account/user administrator permission; the user service independently verifies the original user.
// @Tags Admin account user
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
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/user/users [get]
func (h *adminUserRouteHandler) AccountUsers(ctx *gin.Context) { h.target(ctx) }

// AdminCandidateUsers searches users after independent administrator-assignment authorization.
// @Security ApiKey
// @Summary Search users for administrator role assignment
// @Description Requires account/admin permission independently of the API server. Searches all users, including users without administrator roles, and returns the existing nested user-list response with numeric user IDs.
// @Tags Admin account admin
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
func (h *adminUserRouteHandler) AdminCandidateUsers(ctx *gin.Context) { h.target(ctx) }

// OrganizationUsers lists users for account/organization administration after downstream administrator authorization.
// @Security ApiKey
// @Summary Lists users for account/organization administration
// @Description Requires account/organization administrator permission; the downstream service independently verifies the original user. Returns only id, uuid, username, nickname, and avatar; pagination and search are preserved.
// @Tags Admin account organization
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param verify_status query string false "Verification status"
// @Param labels query []string false "User labels"
// @Param exact_match query bool false "Match the search text exactly"
// @Param sort_by query string false "Sort field"
// @Param sort_order query string false "Sort direction"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/organization/users [get]
func (h *adminUserRouteHandler) OrganizationUsers(ctx *gin.Context) { h.target(ctx) }

// NodeUsers lists users for compute/node administration after downstream administrator authorization.
// @Security ApiKey
// @Summary Lists users for compute/node administration
// @Description Requires compute/node administrator permission; the downstream service independently verifies the original user. Returns only id, uuid, username, nickname, and avatar; pagination and search are preserved.
// @Tags Admin compute node
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param verify_status query string false "Verification status"
// @Param labels query []string false "User labels"
// @Param exact_match query bool false "Match the search text exactly"
// @Param sort_by query string false "Sort field"
// @Param sort_order query string false "Sort direction"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/compute/node/users [get]
func (h *adminUserRouteHandler) NodeUsers(ctx *gin.Context) { h.target(ctx) }

// ModelUsers lists users for asset/model administration after downstream administrator authorization.
// @Security ApiKey
// @Summary Lists users for asset/model administration
// @Description Requires asset/model administrator permission; the downstream service independently verifies the original user. Returns only id, uuid, username, nickname, and avatar; pagination and search are preserved.
// @Tags Admin asset model
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param verify_status query string false "Verification status"
// @Param labels query []string false "User labels"
// @Param exact_match query bool false "Match the search text exactly"
// @Param sort_by query string false "Sort field"
// @Param sort_order query string false "Sort direction"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/model/users [get]
func (h *adminUserRouteHandler) ModelUsers(ctx *gin.Context) { h.target(ctx) }

// DatasetUsers lists users for asset/dataset administration after downstream administrator authorization.
// @Security ApiKey
// @Summary Lists users for asset/dataset administration
// @Description Requires asset/dataset administrator permission; the downstream service independently verifies the original user. Returns only id, uuid, username, nickname, and avatar; pagination and search are preserved.
// @Tags Admin asset dataset
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param verify_status query string false "Verification status"
// @Param labels query []string false "User labels"
// @Param exact_match query bool false "Match the search text exactly"
// @Param sort_by query string false "Sort field"
// @Param sort_order query string false "Sort direction"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/asset/dataset/users [get]
func (h *adminUserRouteHandler) DatasetUsers(ctx *gin.Context) { h.target(ctx) }

// AcademyUsers lists users for academy/user_permission administration after downstream administrator authorization.
// @Security ApiKey
// @Summary Lists users for academy/user_permission administration
// @Description Requires academy/user_permission administrator permission; the downstream service independently verifies the original user. Returns only id, uuid, username, nickname, and avatar; pagination and search are preserved.
// @Tags Admin academy user_permission
// @Produce json
// @Param page query int false "Page number"
// @Param per query int false "Page size"
// @Param search query string false "Search text"
// @Param verify_status query string false "Verification status"
// @Param labels query []string false "User labels"
// @Param exact_match query bool false "Match the search text exactly"
// @Param sort_by query string false "Sort field"
// @Param sort_order query string false "Sort direction"
// @Success 200 {object} types.Response{data=types.UserChoiceList}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/academy/user_permission/users [get]
func (h *adminUserRouteHandler) AcademyUsers(ctx *gin.Context) { h.target(ctx) }

// AccountUser returns a user profile for account administration after downstream administrator authorization.
// @Security ApiKey
// @Summary Returns a user profile for account administration
// @Description Requires account/user administrator permission; the downstream service independently verifies the original user.
// @Tags Admin account user
// @Produce json
// @Param identifier path string true "Username or user UUID"
// @Param type query string false "Use uuid to look up a user UUID"
// @Success 200 {object} types.Response{data=types.User}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/user/user/{identifier} [get]
func (h *adminUserRouteHandler) AccountUser(ctx *gin.Context) {
	ctx.Params = append(ctx.Params, gin.Param{Key: "username", Value: ctx.Param("identifier")})
	h.target(ctx)
}

// UpdateAccountUser updates a user profile for account administration after downstream administrator authorization.
// @Security ApiKey
// @Summary Updates a user profile for account administration
// @Description Requires account/user administrator permission; the downstream service independently verifies the original user.
// @Tags Admin account user
// @Accept json
// @Produce json
// @Param identifier path string true "User UUID"
// @Param body body types.UpdateUserRequest true "Profile fields; role assignments use separate role APIs"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/user/user/{identifier} [put]
func (h *adminUserRouteHandler) UpdateAccountUser(ctx *gin.Context) {
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: ctx.Param("identifier")})
	h.target(ctx)
}

// CreateOrganization creates an organization using the configured organization model after downstream administrator authorization.
// @Security ApiKey
// @Summary Creates an organization using the configured organization model
// @Description Requires account/organization administrator permission; the downstream service independently verifies the original user.
// @Tags Admin account organization
// @Accept json
// @Produce json
// @Param body body types.CreateOrgReq true "Organization fields"
// @Success 200 {object} types.Response{data=types.Organization}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/organization/organizations [post]
func (h *adminUserRouteHandler) CreateOrganization(ctx *gin.Context) { h.target(ctx) }

// CommonUser independently authorized entry preserves the existing user handler contract.
// @Security ApiKey
// @Summary Get a profile for the administrator navigation bar
// @Description Requires administrator membership independently checked by the user service; original business permissions and personal data scope are preserved.
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
func (h *adminUserRouteHandler) CommonUser(ctx *gin.Context) {
	ctx.Params = append(ctx.Params, gin.Param{Key: "username", Value: ctx.Param("identifier")})
	h.target(ctx)
}

// CommonUpdateUser independently authorized entry preserves the existing user handler contract.
// @Security ApiKey
// @Summary Update a profile from the administrator navigation bar
// @Description Requires administrator membership independently checked by the user service; original business permissions and personal data scope are preserved.
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
func (h *adminUserRouteHandler) CommonUpdateUser(ctx *gin.Context) {
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: ctx.Param("identifier")})
	h.target(ctx)
}

// CommonUserTags independently authorized entry preserves the existing user handler contract.
// @Security ApiKey
// @Summary Set the current administrator user's industry tags
// @Description Requires administrator membership independently checked by the user service; original business permissions and personal data scope are preserved.
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
func (h *adminUserRouteHandler) CommonUserTags(ctx *gin.Context) {
	h.target(ctx)
}

// CommonEmailVerificationCode independently authorized entry preserves the existing user handler contract.
// @Security ApiKey
// @Summary Send a profile email verification code
// @Description Requires administrator membership independently checked by the user service; original business permissions and personal data scope are preserved.
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
func (h *adminUserRouteHandler) CommonEmailVerificationCode(ctx *gin.Context) {
	h.target(ctx)
}

// createAdminCommonJWTRoutes independently authorizes the caller's existing administrator session.
func createAdminCommonJWTRoutes(apiGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, tokens *handler.JWTHandler) {
	apiGroup.POST("/admin/common/jwt/token", mc.Auth.PlatformAdmin(types.AdminPermissionAny), tokens.RefreshAdmin)
}

// createAdminNamespaceLookupRoute independently checks token-plan permission before resolving namespace names.
func createAdminNamespaceLookupRoute(adminGroup *gin.RouterGroup, mc middleware.MiddlewareCollection, lookup gin.HandlerFunc) {
	adminGroup.POST("/namespaces/lookup", mc.Auth.PlatformAdmin(types.AdminAPIGatewayTokenPlan), lookup)
}
