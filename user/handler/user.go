package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	commonlog "opencsg.com/csghub-server/common/log"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	apicomponent "opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/user/workflow"
	workflowCommon "opencsg.com/csghub-server/user/workflow/common"
)

type UserHandler struct {
	c                              component.UserComponent
	sc                             apicomponent.SensitiveComponent
	publicDomain                   string
	signinSuccessRedirectURL       string
	signinFailureRedirectURL       string
	atc                            component.AccessTokenComponent
	codeSoulerVScodeRedirectURL    string
	codeSoulerJetbrainsRedirectURL string
	liteRedirectURL                string
	flowRedirects                  map[string]*url.URL
	config                         *config.Config
	uv                             component.UserVerifyComponent
}

// CanAdmin reports whether the specified user has platform administrator privileges.
func (h *UserHandler) CanAdmin(ctx context.Context, username string) (bool, error) {
	return h.c.CanAdmin(ctx, username)
}

const (
	VSCODE    = "vscode"
	JETBRAINS = "jetbrains"
	CASDOOR   = "casdoor"
	LITE      = "lite"

	// porticoSigninCookieName binds a Portico sign-in transaction to the
	// browser that initiated it, so the callback can reject a code redeemed
	// from a different browser (login CSRF / account swapping).
	//
	// The __Host- prefix makes the browser reject any cookie that is not
	// Secure, is not scoped to Path=/, or carries a Domain attribute. A
	// same-site subdomain can therefore no longer forge this cookie with a
	// Domain=<apex> attribute and replay a login into a victim's browser.
	porticoSigninCookieName = "__Host-portico_signin_state"
)

func NewUserHandler(config *config.Config) (*UserHandler, error) {
	h := &UserHandler{}
	var err error
	h.c, err = component.NewUserComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create user component: %w", err)
	}
	sc, err := apicomponent.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	h.sc = sc
	domainParsedUrl, err := url.Parse(config.APIServer.PublicDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public domain '%s': %w", config.APIServer.PublicDomain, err)
	}
	h.atc, err = component.NewAccessTokenComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create access token component: %w", err)
	}
	h.publicDomain = domainParsedUrl.Hostname()
	h.signinSuccessRedirectURL = config.User.SigninSuccessRedirectURL
	h.signinFailureRedirectURL = config.ServerFailureRedirectURL
	h.codeSoulerVScodeRedirectURL = config.User.CodeSoulerVScodeRedirectURL
	h.codeSoulerJetbrainsRedirectURL = config.User.CodeSoulerJetBrainsRedirectURL
	h.liteRedirectURL = config.User.LiteRedirectURL
	h.flowRedirects, err = parseFlowRedirects(config.User.FlowRedirects)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user flow redirects: %w", err)
	}
	h.config = config
	h.uv, err = component.NewUserVerifyComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create user verify component: %w", err)
	}
	return h, err
}

// CreateUser godoc
// @Security     ApiKey
// @Summary      Create a new user
// @Description  create a new user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        body   body  types.CreateUserRequest true "body"
// @Success      200  {object}  types.Response{data=database.User} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /users [post]
// func (h *UserHandler) Create(ctx *gin.Context) {
// 	var req *types.CreateUserRequest
// 	if err := ctx.ShouldBindJSON(&req); err != nil {
// 		slog.Error("Bad request format", "error", err)
// 		httpbase.BadRequest(ctx, err.Error())
// 		return
// 	}

// 	slog.Debug("Creating user", slog.Any("req", req))
// 	user, err := h.c.Create(ctx, req)
// 	if err != nil {
// 		slog.Error("Failed to create user", slog.Any("error", err))
// 		httpbase.ServerError(ctx, err)
// 		return
// 	}

// 	slog.Info("Create user succeed", slog.String("user", user.Username))
// 	httpbase.OK(ctx, user)
// }

// UpdateUser godoc
// @Security     ApiKey
// @Summary      Update user. If change user name, should only send 'new_username' in the request body.
// @Description  Update user profile fields. Role assignments are managed through administrator role APIs.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        id path string true "user identifier, could be username(depricated) or uuid"
// @Param        current_user  query  string true "current user"
// @Param        type query string false "type of identifier, uuid or username, default is username" Enums(uuid, username)
// @Param        body   body  types.UpdateUserRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{id} [put]
func (h *UserHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var err error
	_, err = h.sc.CheckRequestV2(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}

	id := ctx.Param("id")
	req.UUID = &id
	req.OpUser = currentUser
	err = h.c.UpdateByUUID(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update user by uuid", slog.Any("error", err), slog.String("uuid", *req.UUID), slog.String("current_user", currentUser), slog.Any("req", *req))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrUserNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrReqParamInvalid) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Update user by uuid succeed", slog.String("uuid", *req.UUID), slog.String("current_user", currentUser))
	httpbase.OK(ctx, nil)
}

// DeleteUser godoc
// @Security     ApiKey
// @Summary      Delete user
// @Description  Active administrator roles must be revoked before deletion. This endpoint schedules an asynchronous workflow; administrator membership is checked again in the deletion transaction before related data is removed.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user  query  string true "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      409  {object}  types.APIBadRequest "The user is the last administrator of an organization"
// @Failure      404  {object}  types.APINotFound "Not found"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username} [delete]
func (h *UserHandler) Delete(ctx *gin.Context) {
	operator := httpbase.GetCurrentUser(ctx)
	userName := ctx.Param("username")

	// Check if operator can delete user
	isServerErr, err := h.c.CheckOperatorAndUser(ctx.Request.Context(), operator, userName)

	if err != nil {
		if isServerErr {
			if errors.Is(err, errorx.ErrNotFound) {
				slog.ErrorContext(ctx.Request.Context(), "Operator or user not found", slog.String("operator", operator), slog.String("user", userName), slog.Any("err", err))
				httpbase.NotFoundError(ctx, fmt.Errorf("user cannot be deleted: %w", err))
				return
			}
			slog.ErrorContext(ctx.Request.Context(), "Check operator and user failed", slog.String("operator", operator), slog.String("user", userName), slog.Any("err", err))
			httpbase.ServerError(ctx, fmt.Errorf("user cannot be deleted: %w", err))
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Bad Request", slog.String("operator", operator), slog.String("user", userName), slog.Any("err", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrAdminUserCannotBeDeleted)
		return
	}

	// Prevent deleting the only administrator of any organization.
	isLastOrganizationAdmin, err := h.c.CheckIfUserIsLastOrgAdmin(ctx.Request.Context(), userName)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Check last organization administrator failed", slog.String("user", userName), slog.Any("err", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to check last organization administrator: %w", err))
		return
	}
	if isLastOrganizationAdmin {
		err := errorx.LastOrgAdmin(
			errors.New("cannot delete the last administrator of an organization"),
			errorx.Ctx().Set("username", userName),
		)
		slog.ErrorContext(ctx.Request.Context(), "User is the last organization administrator", slog.String("user", userName))
		httpbase.ConflictError(ctx, err)
		return
	}
	// Check if user has running or building deployments
	hasDeployments, err := h.c.CheckIfUserHasRunningOrBuildingDeployments(ctx.Request.Context(), userName)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Check if user has deployments failed", slog.String("user", userName), slog.Any("err", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to check if user has deployments, error: %w", err))
		return
	}
	if hasDeployments {
		slog.ErrorContext(ctx.Request.Context(), "User has deployments", slog.String("user", userName))
		httpbase.BadRequestWithExt(ctx, errorx.ErrUserHasDeployments)
		return
	}

	// Check if user has bills, Saas only
	hasBills, err := h.c.CheckIfUserHasBills(ctx.Request.Context(), userName)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Check if user has bills failed", slog.String("user", userName), slog.Any("err", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to check if user has bills, error: %w", err))
		return
	}
	if hasBills {
		slog.ErrorContext(ctx.Request.Context(), "User has bills", slog.String("user", userName))
		httpbase.BadRequestWithExt(ctx, errorx.ErrUserHasBills)
		return
	}

	//start workflow to delete user
	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.WorkflowUserDeletionQueueName,
	}

	we, err := workflowClient.ExecuteWorkflow(context.Background(), workflowOptions, workflow.UserDeletionWorkflow,
		workflowCommon.User{
			Username: userName,
			Operator: operator,
		},
		h.config,
	)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to start user deletion workflow", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to start user deletion workflow, error: %w", err))
		return
	}

	slog.InfoContext(ctx.Request.Context(), "start user deletion workflow", slog.String("workflow_id", we.GetID()), slog.String("userName", userName), slog.String("operator", operator))
	httpbase.OK(ctx, nil)
}

// GetUser godoc
// @Security     ApiKey
// @Summary      Get user info. Admin and the user self can see full info, other users can only see basic info.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username or uuid, defined by the query string 'type'"
// @Param        current_user  query  string false "current user"
// @Param 		 type query string false "path param is usernam or uuid, default to username" Enums(username, uuid)
// @Success      200  {object}  types.Response{data=types.User} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username} [get]
func (h *UserHandler) Get(ctx *gin.Context) {
	visitorName := httpbase.GetCurrentUser(ctx)
	authType := httpbase.GetAuthType(ctx)
	userNameOrUUID := ctx.Param("username")
	useUUID := ctx.Query("type") == "uuid"
	var user *types.User
	var err error
	if authType == httpbase.AuthTypeSystemApiKey {
		user, err = h.c.GetInternal(ctx.Request.Context(), userNameOrUUID, useUUID)
	} else {
		user, err = h.c.Get(ctx.Request.Context(), userNameOrUUID, visitorName, useUUID)
	}
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get user", slog.Any("error", err))
		// TODO: in user server component need to use errorx
		if errors.Is(err, sql.ErrNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Get user succeed", slog.String("userName", userNameOrUUID))
	httpbase.OK(ctx, user)
}

// GetUsers godoc
// @Security     ApiKey
// @Summary      Get users info. Only Admin
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        verify_status  query  string true "verify_status"
// @Param        search  query  string true "search"
// @Param        labels  query  []string false "labels, such as ['vip', 'basic']"
// @Param        exact_match  query  bool false "exact_match, default is false"
// @Success      200  {object}  types.Response{data=[]types.User,total=int} "OK"
// @Param        exact_match  query  bool false "exact_match, default is false"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /users [get]
func (h *UserHandler) Index(ctx *gin.Context) {
	req, err := parseUserListRequest(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	users, count, err := h.c.Index(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get user", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  users,
		"total": count,
	}

	slog.InfoContext(ctx.Request.Context(), "Get users succeed")
	httpbase.OK(ctx, respData)
}

// UserChoices serves minimal candidates through independently documented submodule route handlers.
// It shares search parsing with Index but never returns the full account-list projection.
func (h *UserHandler) UserChoices(ctx *gin.Context) {
	req, err := parseUserListRequest(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	users, total, err := h.c.UserChoices(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get user choices", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, types.UserChoiceList{Data: users, Total: total})
}

// UserChoice serves a voucher recipient projection through an independently documented submodule handler.
func (h *UserHandler) UserChoice(ctx *gin.Context) {
	username := ctx.Param("username")
	if username == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("username is empty"), nil))
		return
	}
	user, err := h.c.UserChoice(ctx.Request.Context(), username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}
	httpbase.OK(ctx, user)
}

// parseUserListRequest preserves pagination, filtering, and validation for both list projections.
func parseUserListRequest(ctx *gin.Context) (types.UserListReq, error) {
	visitorName := httpbase.GetCurrentUser(ctx)
	search := ctx.Query("search")
	sortBy := ctx.Query("sort_by")
	sortOrder := ctx.Query("sort_order")

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return types.UserListReq{}, err
	}
	_labels := ctx.QueryArray("labels")
	labels := types.ParseLabels(_labels)
	verifyStatus := ctx.Query("verify_status")
	exactMatchQuery := ctx.Query("exact_match")
	exactMatch := false
	if exactMatchQuery != "" {
		var err error
		exactMatch, err = strconv.ParseBool(exactMatchQuery)
		if err != nil {
			ext := errorx.Ctx().Set("query", "exact_match")
			return types.UserListReq{}, errorx.ReqParamInvalid(err, ext)
		}

	}
	return types.UserListReq{
		VisitorName:  visitorName,
		Search:       search,
		VerifyStatus: verifyStatus,
		Labels:       labels,
		Per:          per,
		Page:         page,
		SortBy:       sortBy,
		SortOrder:    sortOrder,
		ExactMatch:   exactMatch,
	}, nil
}

func (h *UserHandler) Casdoor(ctx *gin.Context) {
	code := ctx.Query("code")
	state := ctx.Query("state")
	slog.DebugContext(ctx.Request.Context(), "get casdoor callback", slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)), slog.String("state_fingerprint", commonlog.CredentialFingerprint("oauth_state", state)))

	h.handleSigninCallback(ctx, code, state, state)
}

// PorticoSignin starts the backend-initiated Portico sign-in flow with PKCE
// and redirects the browser to the Portico authorization page. The optional
// state query param carries the same routing state as the casdoor callback.
func (h *UserHandler) PorticoSignin(ctx *gin.Context) {
	state := ctx.Query("state")
	if state == "" {
		state = CASDOOR
	}
	slog.DebugContext(ctx.Request.Context(), "start portico signin", slog.String("state_fingerprint", commonlog.CredentialFingerprint("oauth_state", state)))

	redirectURL, oauthState, err := h.c.BuildSigninRedirect(ctx.Request.Context(), state)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to start portico signin", slog.Any("error", commonlog.SafeError(err)), slog.String("state_fingerprint", commonlog.CredentialFingerprint("oauth_state", state)))
		h.redirectToSigninFailure(ctx, fmt.Sprintf("failed to signin: %v", err))
		return
	}

	// Bind the sign-in transaction to the initiating browser. The callback
	// refuses to redeem the authorization code unless this cookie matches the
	// state in the callback URL, preventing cross-browser code redemption.
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     porticoSigninCookieName,
		Value:    oauthState,
		Path:     "/",
		HttpOnly: true,
		// Secure must stay true regardless of EnableHTTPS: the __Host- prefix
		// is only valid on Secure cookies, and a non-Secure downgrade would let
		// the browser reject the binding silently instead of failing closed.
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	slog.InfoContext(ctx.Request.Context(), "redirecting to portico authorization page", slog.String("url", redirectURL))
	redirectWithoutBody(ctx, http.StatusFound, redirectURL)
}

// Portico handles the OAuth callback from Portico. The OAuth state is the
// nonce prefix, which the sso client consumes during the token exchange,
// followed by the original routing state.
func (h *UserHandler) Portico(ctx *gin.Context) {
	code := ctx.Query("code")
	state := ctx.Query("state")
	slog.DebugContext(ctx.Request.Context(), "get portico callback", slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)), slog.String("state_fingerprint", commonlog.CredentialFingerprint("oauth_state", state)))

	_, routingState, ok := rpc.SplitPorticoSigninState(state)
	if !ok {
		errMsg := fmt.Sprintf("invalid portico signin state: %s", state)
		slog.ErrorContext(ctx.Request.Context(), errMsg, slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)))
		h.redirectToSigninFailure(ctx, "failed to signin: "+errMsg)
		return
	}

	// The callback must be completed by the same browser that initiated the
	// sign-in. Without this binding, an attacker who retains their own
	// un-consumed callback URL could hand it to a victim and log the victim
	// into the attacker's account.
	boundState, err := ctx.Cookie(porticoSigninCookieName)
	if err != nil || boundState != state {
		slog.ErrorContext(ctx.Request.Context(), "portico callback rejected: sign-in state is not bound to this browser", slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)), slog.String("state_fingerprint", commonlog.CredentialFingerprint("oauth_state", state)))
		h.redirectToSigninFailure(ctx, "failed to signin: portico sign-in state is not bound to this browser")
		return
	}

	h.handleSigninCallback(ctx, code, state, routingState)
}

// handleSigninCallback exchanges the OAuth code for a local JWT and redirects
// the browser according to the routing state, which selects the vscode,
// jetbrains, lite or portal redirect targets, or treats the routing state as
// a custom flow URL.
func (h *UserHandler) handleSigninCallback(ctx *gin.Context, code, oauthState, routingState string) {
	jwtToken, signed, err := h.c.Signin(ctx.Request.Context(), code, oauthState)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to signin", slog.Any("error", commonlog.SafeError(err)), slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)), slog.String("state_fingerprint", commonlog.CredentialFingerprint("oauth_state", oauthState)))
		var customErr errorx.CustomError
		if errors.As(err, &customErr) {
			if handleConflictCustomError(ctx, customErr, h.signinFailureRedirectURL) {
				return
			}
		}

		h.redirectToSigninFailure(ctx, fmt.Sprintf("failed to signin: %v", err))
		return
	}

	var (
		targetUrl      string
		starshipApiKey string
	)

	if routingState == VSCODE || routingState == JETBRAINS {
		starshipApiKey, err = h.getStarshipApiKey(ctx, jwtToken.CurrentUser, "codesouler-"+routingState)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get user %s starship apikey for login from %s", jwtToken.CurrentUser, routingState)
			slog.ErrorContext(ctx.Request.Context(), errMsg, slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)), slog.Any("error", commonlog.SafeError(err)))
			h.redirectToSigninFailure(ctx, errMsg)
			return
		}
		codeSoulerEndpoint := h.codeSoulerVScodeRedirectURL
		if routingState == JETBRAINS {
			codeSoulerEndpoint = h.codeSoulerJetbrainsRedirectURL
		}
		targetUrl = fmt.Sprintf("%s?apikey=%s&portal_url=%s&jwt=%s", codeSoulerEndpoint, starshipApiKey, h.signinSuccessRedirectURL, signed)
	} else if routingState == LITE {
		liteToken, err := h.atc.GetOrCreateFirstAvaiToken(ctx, jwtToken.CurrentUser, string(types.AccessTokenAppCSGHub), "csglite")
		if err != nil {
			errMsg := fmt.Sprintf("failed to get user %s csghub access token for login from %s", jwtToken.CurrentUser, routingState)
			slog.ErrorContext(ctx.Request.Context(), errMsg, slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)), slog.Any("error", commonlog.SafeError(err)))
			h.redirectToSigninFailure(ctx, errMsg)
			return
		}
		targetUrl = fmt.Sprintf("%s?token=%s&portal_url=%s&jwt=%s", h.liteRedirectURL, url.QueryEscape(liteToken), h.signinSuccessRedirectURL, signed)
	} else if routingState == CASDOOR {
		targetUrl = fmt.Sprintf("%s?jwt=%s", h.signinSuccessRedirectURL, signed)
	} else {
		flowTarget, ok := h.resolveFlowRedirectURL(routingState, signed)
		if !ok {
			errMsg := "untrusted signin callback target for 'flows'"
			slog.ErrorContext(ctx.Request.Context(), errMsg, slog.String("code_fingerprint", commonlog.CredentialFingerprint("oauth_code", code)), slog.String("state_fingerprint", commonlog.CredentialFingerprint("oauth_state", routingState)))
			h.redirectToSigninFailure(ctx, errMsg)
			return
		}
		targetUrl = flowTarget
	}

	slog.InfoContext(ctx.Request.Context(), "generate login redirect url", slog.String("target_url", commonlog.SafeURLString(targetUrl)))
	redirectWithoutBody(ctx, http.StatusFound, targetUrl)
}

// redirectToSigninFailure sends the browser back to the configured signin
// failure page instead of any client-provided URL.
func (h *UserHandler) redirectToSigninFailure(ctx *gin.Context, errMsg string) {
	errorRedirectURL := fmt.Sprintf("%s?error_code=500&error_message=%s", h.signinFailureRedirectURL, url.QueryEscape(errMsg))
	slog.InfoContext(ctx.Request.Context(), "redirecting to error page", slog.String("url", errorRedirectURL))
	redirectWithoutBody(ctx, http.StatusFound, errorRedirectURL)
}

// resolveFlowRedirectURL resolves the legacy "flows" signin state to a
// redirect target built exclusively from configured flow callbacks. The state
// is accepted only as a registered flow name, or as a URL whose parsed
// scheme, host, port, and path exactly match a registered callback URL.
// Client-provided userinfo, host, port, path, and fragment never reach the
// redirect target, so host-confusion forms like "https://trusted@evil.com"
// are rejected or discarded by construction.
func (h *UserHandler) resolveFlowRedirectURL(state, signed string) (string, bool) {
	if registered, ok := h.flowRedirects[state]; ok {
		return flowCallbackWithJWT(registered, nil, signed), true
	}
	stateURL, err := url.Parse(state)
	if err != nil || stateURL.User != nil || stateURL.Host == "" {
		return "", false
	}
	for _, registered := range h.flowRedirects {
		if sameCallbackTarget(stateURL, registered) {
			return flowCallbackWithJWT(registered, stateURL.Query(), signed), true
		}
	}
	return "", false
}

// sameCallbackTarget reports whether candidate points at the registered
// callback origin and path. Host comparison is case-insensitive; scheme and
// port must match exactly, so a registered "https://a.example.com/cb" does
// not accept "https://a.example.com:443/cb" or an http downgrade.
func sameCallbackTarget(candidate, registered *url.URL) bool {
	if candidate.Scheme != registered.Scheme {
		return false
	}
	if !strings.EqualFold(candidate.Hostname(), registered.Hostname()) {
		return false
	}
	if candidate.Port() != registered.Port() {
		return false
	}
	return candidate.EscapedPath() == registered.EscapedPath()
}

// flowCallbackWithJWT builds the redirect target from a registered callback
// URL: the registered query is authoritative and wins over colliding state
// query keys, state query params fill in the rest, and the freshly signed JWT
// is appended last.
func flowCallbackWithJWT(registered *url.URL, extra url.Values, signed string) string {
	target := *registered
	query := registered.Query()
	for key, values := range extra {
		if _, ok := query[key]; !ok {
			query[key] = values
		}
	}
	query.Set("jwt_token", signed)
	target.RawQuery = query.Encode()
	return target.String()
}

// parseFlowRedirects validates the configured flow callback targets. Entries
// must be absolute URLs without userinfo; https is required except for
// loopback http targets reserved for local development. A misconfigured entry
// fails startup instead of silently weakening the signin redirect allowlist.
func parseFlowRedirects(redirects config.FlowRedirects) (map[string]*url.URL, error) {
	parsed := make(map[string]*url.URL, len(redirects))
	for _, redirect := range redirects {
		if redirect.Name == "" {
			return nil, fmt.Errorf("flow redirect entry %q has an empty name", redirect.CallbackURL)
		}
		target, err := url.Parse(redirect.CallbackURL)
		if err != nil {
			return nil, fmt.Errorf("invalid callback URL for flow redirect %q: %w", redirect.Name, err)
		}
		if target.Host == "" || target.User != nil {
			return nil, fmt.Errorf("callback URL for flow redirect %q must be an absolute URL without userinfo", redirect.Name)
		}
		if target.Scheme != "https" && !(target.Scheme == "http" && isLoopbackHost(target.Hostname())) {
			return nil, fmt.Errorf("callback URL for flow redirect %q must use https, or http on a loopback address for local development", redirect.Name)
		}
		parsed[redirect.Name] = target
	}
	return parsed, nil
}

// isLoopbackHost reports whether the host refers to the local machine: the
// "localhost" name or an IP in the loopback ranges (127.0.0.0/8, ::1).
func isLoopbackHost(hostname string) bool {
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func (h *UserHandler) getStarshipApiKey(ctx *gin.Context, userName, tokenName string) (string, error) {
	token, err := h.atc.GetOrCreateFirstAvaiToken(ctx, userName, string(types.AccessTokenAppStarship), tokenName)
	if err != nil {
		return "", fmt.Errorf("fail to get starship token for user %s, tokenName %s: %w", userName, tokenName, err)
	}
	return token, nil
}

// CreateVerify godoc
// @Security     ApiKey
// @Summary      Create user verification
// @Description  Create a user verification request; SaaS binds ownership to the authenticated user UUID.
// @Tags         UserVerify
// @Accept       json
// @Produce      json
// @Param        body body types.UserVerifyReq true "User verification request body"
// @Success      200  {object}  types.Response{data=database.UserVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Authenticated user UUID required"
// @Failure      409  {object}  types.APIBadRequest "Verification record conflicts with the manual submission"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/verify [post]
func (h *UserHandler) CreateVerify(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.UserVerifyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.Username = currentUser
	orgVerify, err := h.uv.Create(ctx.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, errorx.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		if errors.Is(err, database.ErrIdentityConflict) {
			httpbase.ConflictError(ctx, errorx.ErrIdentityConflict)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to create organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Create organization Verify succeed", slog.String("real name", orgVerify.RealName))
	httpbase.OK(ctx, orgVerify)
}

// UpdateVerify updates verification while preserving request authorization and cancellation in the component.
// @Security     ApiKey
// @Summary      Update user verification
// @Description  update user verification status (approved or rejected)
// @Tags         UserVerify
// @Accept       json
// @Produce      json
// @Param        id     path  int64  true  "verification ID"
// @Param        body  body  types.UserVerifyStatusReq true "Update verification request body"
// @Success      200  {object}  types.Response{data=database.UserVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      409  {object}  types.APIBadRequest "Verification record conflicts with the manual review"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/verify/{id} [put]
func (h *UserHandler) UpdateVerify(ctx *gin.Context) {
	vID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var req types.UserVerifyStatusReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if req.Status != types.VerifyStatusRejected && req.Status != types.VerifyStatusApproved {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.String("err", "Not allowed status"))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("not allowed status"), nil))
		return
	}

	if req.Status == types.VerifyStatusRejected && req.Reason == "" {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.String("err", "rejected need reason"))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("rejected need reason"), nil))
		return
	}

	orgVerify, err := h.uv.Update(ctx.Request.Context(), vID, req.Status, req.Reason)
	if err != nil {
		if errors.Is(err, database.ErrIdentityConflict) {
			httpbase.ConflictError(ctx, errorx.ErrIdentityConflict)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "update organization Verify succeed", slog.String("real name", orgVerify.RealName))
	httpbase.OK(ctx, orgVerify)
}

// GetVerify reads verification while preserving request authorization and cancellation in the component.
// @Security     ApiKey
// @Summary      Get user verification
// @Description  Get local verification information by user UUID; SaaS permits only the authenticated account's UUID and includes a masked identity number when available.
// @Tags         UserVerify
// @Accept       json
// @Produce      json
// @Param        id             path  string true  "user UUID"
// @Success      200  {object}  types.Response{data=database.UserVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Authentication required"
// @Failure      403  {object}  types.APIForbidden "Another account's verification is forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/verify/{id} [get]
func (h *UserHandler) GetVerify(ctx *gin.Context) {
	id := ctx.Param("id")
	orgVerify, err := h.uv.Get(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errorx.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, orgVerify)
}

// UpdateUserLabels godoc
// @Security     ApiKey
// @Summary      Update user labels
// @Description  Update the labels of a specified user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        body  body  types.UserLabelsRequest true "Update user labels body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400   {object} types.APIBadRequest "Bad request"
// @Failure      500   {object} types.APIInternalServerError "Internal server error"
// @Router       /user/labels [put]
func (h *UserHandler) UpdateUserLabels(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.UserLabelsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Invalid user labels update request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("invalid request body"), nil))
		return
	}
	for _, label := range req.Labels {
		if !types.ValidLabels[label] {
			slog.ErrorContext(ctx.Request.Context(), "Invalid user labels update request", slog.String("label", label))
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("invalid request label"), nil))
			return
		}
	}
	req.OpUser = currentUser

	err := h.c.UpdateUserLabels(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update user labels by uuid", slog.Any("error", err), slog.String("uid", req.UUID), slog.String("current_user", currentUser), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Update labels by uuid succeed", slog.String("uid", req.UUID), slog.String("current_user", currentUser))
	httpbase.OK(ctx, nil)
}

// GetEmails godoc
// @Security     ApiKey
// @Summary      Get all user emails
// @Description  Retrieve all user email addresses
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(50)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{data=[]string,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/emails [get]
func (h *UserHandler) GetEmails(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get per and page", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	visitorName := httpbase.GetCurrentUser(ctx)
	emails, count, err := h.c.GetEmails(ctx, visitorName, per, page)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get all user emails", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, emails, count)
}

// GetEmailsInternal godoc
// @Security     ApiKey
// @Summary      Get all user emails for internal services
// @Description  Retrieve all user email addresses for internal services
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(50)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{data=[]string,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /internal/user/emails [get]
func (h *UserHandler) GetEmailsInternal(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	emails, count, err := h.c.GetEmailsInternal(ctx, per, page)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get all user emails", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, emails, count)
}

// FindByUUIDs godoc
// @Security     ApiKey
// @Summary      Find users by UUIDs
// @Description  Retrieve a list of users by their UUIDs
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        uuids   query  []string  true  "User UUIDs"
// @Success      200     {object} types.Response{data=[]types.User} "OK"
// @Failure      400     {object} types.APIBadRequest "Bad request"
// @Failure      500     {object} types.APIInternalServerError "Internal server error"
// @Router       /users/by-uuids [get]
func (h *UserHandler) FindByUUIDs(ctx *gin.Context) {
	uuids := ctx.QueryArray("uuids")
	users, err := h.c.FindByUUIDs(ctx, uuids)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to find user by uuids", slog.Any("error", err), slog.Any("uuids", uuids))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, users)
}

// CloseAccount godoc
// @Security     ApiKey
// @Summary      Delete user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user  query  string true "current user"
// @Param        repository  query  bool false "repository"
// @Param        discussion  query  bool false "discussion"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      409  {object}  types.APIBadRequest "The user is the last administrator of an organization"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/close_account [delete]
func (h *UserHandler) CloseAccount(ctx *gin.Context) {
	operator := httpbase.GetCurrentUser(ctx)
	userName := ctx.Param("username")

	if operator != userName {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("invalid request"), nil))
		return
	}

	repository, _ := strconv.ParseBool(ctx.Query("repository"))
	discussion, _ := strconv.ParseBool(ctx.Query("discussion"))
	req := types.CloseAccountReq{
		Repository: repository,
		Discussion: discussion,
	}

	// Prevent closing the account of the only administrator of any organization.
	isLastOrganizationAdmin, err := h.c.CheckIfUserIsLastOrgAdmin(ctx, userName)
	if err != nil {
		httpbase.ServerError(ctx, fmt.Errorf("failed to check last organization administrator: %w", err))
		return
	}
	if isLastOrganizationAdmin {
		err := errorx.LastOrgAdmin(
			errors.New("cannot delete the last administrator of an organization"),
			errorx.Ctx().Set("username", userName),
		)
		httpbase.ConflictError(ctx, err)
		return
	}
	// Check if user has running or building deployments
	hasDeployments, err := h.c.CheckIfUserHasRunningOrBuildingDeployments(ctx, userName)
	if err != nil {
		httpbase.ServerError(ctx, fmt.Errorf("failed to check if user has deployments, error: %w", err))
		return
	}
	if hasDeployments {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("users who own deployments cannot be deleted"), nil))
		return
	}

	// Check if user has bills, Saas only
	hasBills, err := h.c.CheckIfUserHasBills(ctx, userName)
	if err != nil {
		httpbase.ServerError(ctx, fmt.Errorf("failed to check if user has bills, error: %w", err))
		return
	}
	if hasBills {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("users who own bills cannot be deleted"), nil))
		return
	}

	//start workflow to soft delete user
	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.WorkflowUserDeletionQueueName,
	}

	we, err := workflowClient.ExecuteWorkflow(context.Background(), workflowOptions, workflow.UserSoftDeletionWorkflow,
		workflowCommon.User{
			Username: userName,
			Operator: operator,
		},
		req,
		h.config,
	)
	if err != nil {
		httpbase.ServerError(ctx, fmt.Errorf("failed to start user soft deletion workflow, error: %w", err))
		return
	}

	slog.InfoContext(ctx.Request.Context(), "start user soft deletion workflow", slog.String("workflow_id", we.GetID()), slog.String("userName", userName), slog.String("operator", operator))

	httpbase.OK(ctx, nil)
}

// GetUserUUIDs godoc
// @Security     ApiKey
// @Summary      Get user UUIDs
// @Description  Get user UUIDs
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{data=[]string,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure 	 401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/user_uuids [get]
func (h *UserHandler) GetUserUUIDs(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	userUUIDs, total, err := h.c.GetUserUUIDs(ctx, per, page)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  userUUIDs,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}

// GetAdminUserUUIDs godoc
// @Security     ApiKey
// @Summary      Get administrator user UUIDs
// @Description  Retrieve the UUIDs of users with administrator permissions
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]string,total=int} "OK"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/admin_uuids [get]
func (h *UserHandler) GetAdminUserUUIDs(ctx *gin.Context) {
	userUUIDs, err := h.c.GetAdminUserUUIDs(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  userUUIDs,
		"total": len(userUUIDs),
	}
	httpbase.OK(ctx, respData)
}

// GetAdminEmails godoc
// @Security     ApiKey
// @Summary      Get administrator email addresses
// @Description  Retrieve the email addresses of users with administrator permissions
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]string,total=int} "OK"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/admin_emails [get]
func (h *UserHandler) GetAdminEmails(ctx *gin.Context) {
	emails, err := h.c.GetAdminEmails(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  emails,
		"total": len(emails),
	}
	httpbase.OK(ctx, respData)
}

// GenerateVerificationCodeAndSendEmail godoc
// @Security     ApiKey
// @Summary      GenerateVerificationCodeAndSendEmail
// @Description  GenerateVerificationCodeAndSendEmail
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        email path string true "email"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/email-verification-code/{email} [post]
func (h *UserHandler) GenerateVerificationCodeAndSendEmail(ctx *gin.Context) {
	uid := httpbase.GetCurrentUserUUID(ctx)
	email := ctx.Param("email")
	err := h.c.GenerateVerificationCodeAndSendEmail(ctx, uid, email)
	if err != nil {
		if errors.Is(err, errorx.ErrUserNotFound) || errors.Is(err, errorx.ErrForbidden) || errors.Is(err, errorx.ErrContactBlocked) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "GenerateVerificationCodeAndSendEmail failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// AdminGetUserTags godoc
// @Security     ApiKey
// @Summary      List a user's tags as an administrator
// @Description  Returns the user-scope tags of the user identified by the path parameter (a username or a uuid), routing tags (low_priority / priority:<n>) included. The public profile hides routing tags.
// @Tags         User
// @Produce      json
// @Param        identifier path string true "username or uuid"
// @Success      200  {object}  types.Response{data=[]types.RepoTag} "OK"
// @Failure      401  {object}  types.APIUnauthorized
// @Failure      403  {object}  types.APIForbidden
// @Failure      404  {object}  types.APINotFound "User not found"
// @Router       /admin/account/user/user/{identifier}/tags [get]
func (h *UserHandler) AdminGetUserTags(ctx *gin.Context) {
	identifier := ctx.Param("identifier")
	tags, err := h.c.AdminGetUserTags(ctx, identifier)
	if err != nil {
		if errors.Is(err, errorx.ErrUserNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "AdminGetUserTags failed", slog.Any("err", err), slog.String("identifier", identifier))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, tags)
}

// AdminResetUserTags godoc
// @Security     ApiKey
// @Summary      Reset a user's tags as an administrator
// @Description  Replaces the user-scope tags of the user identified by the path parameter (a username or a uuid). Routing tags (low_priority / priority:<n>) are writable here because they are operator-managed.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        identifier path string true "username or uuid"
// @Param        tagIDs body []int64 true "tagIDs"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request (unknown, duplicated or non user-scope tag ids)"
// @Failure      401  {object}  types.APIUnauthorized
// @Failure      403  {object}  types.APIForbidden
// @Failure      404  {object}  types.APINotFound "User not found"
// @Router       /admin/account/user/user/{identifier}/tags [put]
func (h *UserHandler) AdminResetUserTags(ctx *gin.Context) {
	identifier := ctx.Param("identifier")
	var req []int64
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "AdminResetUserTags failed", slog.Any("err", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	// A JSON null binds to a nil slice with no error; it is a client bug, not
	// a request to clear the set. Only an explicit [] means "clear".
	if req == nil {
		httpbase.BadRequest(ctx, "tag_ids is required")
		return
	}
	if err := h.c.AdminResetUserTags(ctx, identifier, req,
		httpbase.GetCurrentUserUUID(ctx), httpbase.GetCurrentUser(ctx)); err != nil {
		switch {
		case errors.Is(err, errorx.ErrUserNotFound):
			httpbase.NotFoundError(ctx, err)
		case errors.Is(err, component.ErrInvalidUserTagIDs):
			httpbase.BadRequest(ctx, err.Error())
		default:
			slog.ErrorContext(ctx.Request.Context(), "AdminResetUserTags failed", slog.Any("err", err), slog.String("identifier", identifier))
			httpbase.ServerError(ctx, err)
		}
		return
	}
	httpbase.OK(ctx, nil)
}

// ResetUserTags godoc
// @Security     ApiKey
// @Summary      ResetUserTags
// @Description  Allows a user to reset their own tags. This endpoint is only for users to manage their personal tags, not for administrators to set tags for other users.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        tagIDs body []int64 true "tagIDs"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/tags [post]
func (h *UserHandler) ResetUserTags(ctx *gin.Context) {
	uid := httpbase.GetCurrentUserUUID(ctx)
	var req []int64
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "ResetUserTags failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if err := h.c.ResetUserTags(ctx, uid, req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "ResetUserTags failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

func handleConflictCustomError(ctx *gin.Context, customErr errorx.CustomError, redirectURL string) bool {
	errCode := customErr.Code()

	cField, cValue, ok := extractConflictInfo(customErr)
	if !ok {
		return false
	}

	u, _ := url.Parse(redirectURL)
	q := u.Query()
	q.Set("error_code", strconv.Itoa(http.StatusConflict))
	q.Set("error_message_code", errCode)
	q.Set("field_name", cField)
	q.Set("field_value", cValue)
	u.RawQuery = q.Encode()
	slog.InfoContext(ctx.Request.Context(), "redirecting to error page with conflict error", slog.String("url", u.String()))
	redirectWithoutBody(ctx, http.StatusFound, u.String())

	return true
}

func extractConflictInfo(customErr errorx.CustomError) (field, value string, ok bool) {
	errCtx := customErr.Context()

	switch {
	case customErr.Is(errorx.ErrUsernameExists):
		if username, exists := errCtx["username"]; exists {
			if usernameStr, ok := username.(string); ok {
				return "username", usernameStr, true
			}
		}
	case customErr.Is(errorx.ErrEmailExists):
		if email, exists := errCtx["email"]; exists {
			if emailStr, ok := email.(string); ok {
				return "email", emailStr, true
			}
		}
	}
	return "", "", false
}

// ExportUserInfo godoc
// @Security     ApiKey
// @Summary      Export users info. Only Admin
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        verify_status  query  string  false  "Verify status (e.g. 'none', 'pending', 'approved', 'rejected')"
// @Param        search         query  string  false  "Search keyword (match username/email/phone)"
// @Param        labels         query  []string  false  "Labels (e.g. vip,basic) - multiple values supported"
// @Param        cursor         query  int64  false  "Cursor for pagination"
// @Success      200  "OK - SSE stream: events are 'users' (single user JSON), 'error' (error message), 'end' (completion message)"
// @Failure      403  {object}  types.APIForbidden   "Forbidden - not admin"
// @Router       /users/stream-export [get]
func (h *UserHandler) ExportUserInfo(ctx *gin.Context) {
	search := ctx.Query("search")
	_labels := ctx.QueryArray("labels")
	labels := types.ParseLabels(_labels)
	verifyStatus := ctx.Query("verify_status")

	req := types.UserIndexReq{
		Search:       search,
		VerifyStatus: types.VerifyStatus(verifyStatus),
		Labels:       labels,
		Per:          300,
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	data, err := h.c.StreamExportUsers(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "stream export failed in component", slog.Any("error", err), slog.Any("req", req))
		select {
		case data <- types.UserIndexResp{Error: err}:
		case <-ctx.Request.Context().Done():
		}
	}

	for resp := range data {
		if resp.Error != nil {
			ctx.SSEvent("error", resp.Error.Error())
			ctx.Writer.Flush()
			return
		}

		jsonData, err := json.Marshal(resp.Users)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Failed to marshal users", slog.Any("error", err))
			ctx.SSEvent("error", errorx.ErrInternalServerError.Error())
			ctx.Writer.Flush()
			return
		}
		ctx.SSEvent("users", string(jsonData))
		ctx.Writer.Flush()
	}

	ctx.SSEvent("end", "export completed")
	ctx.Writer.Flush()
}

func redirectWithoutBody(ctx *gin.Context, code int, url string) {
	// Signin redirects carry freshly issued tokens in the Location URL; keep
	// them out of Referer headers, back/forward caches, and shared caches.
	ctx.Header("Referrer-Policy", "no-referrer")
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Location", url)
	ctx.AbortWithStatus(code)
}
