package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
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
	EnableHTTPS                    bool
	signinSuccessRedirectURL       string
	signinFailureRedirectURL       string
	atc                            component.AccessTokenComponent
	codeSoulerVScodeRedirectURL    string
	codeSoulerJetbrainsRedirectURL string
	config                         *config.Config
	uv                             component.UserVerifyComponent
}

const (
	VSCODE    = "vscode"
	JETBRAINS = "jetbrains"
	CASDOOR   = "casdoor"
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
	h.EnableHTTPS = config.EnableHTTPS
	h.signinSuccessRedirectURL = config.User.SigninSuccessRedirectURL
	h.signinFailureRedirectURL = config.ServerFailureRedirectURL
	h.codeSoulerVScodeRedirectURL = config.User.CodeSoulerVScodeRedirectURL
	h.codeSoulerJetbrainsRedirectURL = config.User.CodeSoulerJetBrainsRedirectURL
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
// @Description  update user
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
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var err error
	_, err = h.sc.CheckRequestV2(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
		return
	}

	id := ctx.Param("id")
	req.UUID = &id
	req.OpUser = currentUser
	err = h.c.UpdateByUUID(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to update user by uuid", slog.Any("error", err), slog.String("uuid", *req.UUID), slog.String("current_user", currentUser), slog.Any("req", *req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update user by uuid succeed", slog.String("uuid", *req.UUID), slog.String("current_user", currentUser))
	httpbase.OK(ctx, nil)
}

// DeleteUser godoc
// @Security     ApiKey
// @Summary      Delete user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user  query  string true "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username} [delete]
func (h *UserHandler) Delete(ctx *gin.Context) {
	operator := httpbase.GetCurrentUser(ctx)
	userName := ctx.Param("username")

	// Check if operator can delete user
	isServerErr, err := h.c.CheckOperatorAndUser(ctx, operator, userName)
	if err != nil && isServerErr {
		httpbase.ServerError(ctx, fmt.Errorf("user cannot be deleted: %w", err))
		return
	}
	if err != nil && !isServerErr {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	// Check if user has organizations
	hasOrgs, err := h.c.CheckIfUserHasOrgs(ctx, userName)
	if err != nil {
		httpbase.ServerError(ctx, fmt.Errorf("failed to check if user has organzitions, error: %w", err))
		return
	}
	if hasOrgs {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("users who own organizations cannot be deleted"), nil))
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

	//start workflow to delete user
	workflowClient := workflow.GetWorkflowClient()
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
		httpbase.ServerError(ctx, fmt.Errorf("failed to start user deletion workflow, error: %w", err))
		return
	}

	slog.Info("start user deletion workflow", slog.String("workflow_id", we.GetID()), slog.String("userName", userName), slog.String("operator", operator))
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
	if authType == httpbase.AuthTypeApiKey {
		user, err = h.c.GetInternal(ctx, userNameOrUUID, useUUID)
	} else {
		user, err = h.c.Get(ctx, userNameOrUUID, visitorName, useUUID)
	}
	if err != nil {
		slog.Error("Failed to get user", slog.Any("error", err))
		// TODO: in user server component need to use errorx
		if errors.Is(err, sql.ErrNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	slog.Info("Get user succeed", slog.String("userName", userNameOrUUID))
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
// @Success      200  {object}  types.Response{data=[]types.User,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /users [get]
func (h *UserHandler) Index(ctx *gin.Context) {
	visitorName := httpbase.GetCurrentUser(ctx)
	search := ctx.Query("search")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get per and page", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	_labels := ctx.QueryArray("labels")
	labels := types.ParseLabels(_labels)
	verifyStatus := ctx.Query("verify_status")
	users, count, err := h.c.Index(ctx, visitorName, search, verifyStatus, labels, per, page)
	if err != nil {
		slog.Error("Failed to get user", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  users,
		"total": count,
	}

	slog.Info("Get users succeed")
	httpbase.OK(ctx, respData)
}

func (h *UserHandler) Casdoor(ctx *gin.Context) {
	code := ctx.Query("code")
	state := ctx.Query("state")
	slog.Debug("get casdoor callback", slog.String("code", code), slog.String("state", state))

	jwtToken, signed, err := h.c.Signin(ctx.Request.Context(), code, state)
	if err != nil {
		slog.Error("Failed to signin", slog.Any("error", err), slog.String("code", code), slog.String("state", state))
		errorMsg := url.QueryEscape(fmt.Sprintf("failed to signin: %v", err))
		errorRedirectURL := fmt.Sprintf("%s?error_code=500&error_message=%s", h.signinFailureRedirectURL, errorMsg)
		slog.Info("redirecting to error page", slog.String("url", errorRedirectURL))
		ctx.Redirect(http.StatusMovedPermanently, errorRedirectURL)
		return
	}

	var (
		targetUrl      string
		starshipApiKey string
	)

	if state == VSCODE || state == JETBRAINS {
		starshipApiKey, err = h.getStarshipApiKey(ctx, jwtToken.CurrentUser, "codesouler-"+state)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get user %s starship apikey for login from %s", jwtToken.CurrentUser, state)
			slog.Error(errMsg, slog.String("code", code), slog.Any("error", err))
			errorMsg := url.QueryEscape(errMsg)
			errorRedirectURL := fmt.Sprintf("%s?error_code=500&error_message=%s", h.signinFailureRedirectURL, errorMsg)
			slog.Info("redirecting to error page", slog.String("url", errorRedirectURL))
			ctx.Redirect(http.StatusMovedPermanently, errorRedirectURL)
			return
		}
		codeSoulerEndpoint := h.codeSoulerVScodeRedirectURL
		if state == JETBRAINS {
			codeSoulerEndpoint = h.codeSoulerJetbrainsRedirectURL
		}
		targetUrl = fmt.Sprintf("%s?apikey=%s&portal_url=%s&jwt=%s", codeSoulerEndpoint, starshipApiKey, h.signinSuccessRedirectURL, signed)
	} else if state == CASDOOR {
		targetUrl = fmt.Sprintf("%s?jwt=%s", h.signinSuccessRedirectURL, signed)
	} else {
		// parse state as url and get host
		var flowURL *url.URL
		var err error
		flowURL, err = url.Parse(state)
		if err != nil || flowURL.Host == "" {
			errMsg := fmt.Sprintf("invalid state format for 'flows', not a valid URL: %s", state)
			slog.Error(errMsg, slog.String("code", code), slog.Any("error", err))
			errorMsg := url.QueryEscape(errMsg)
			errorRedirectURL := fmt.Sprintf("%s?error_code=500&error_message=%s",
				h.signinFailureRedirectURL, errorMsg)
			slog.Info("redirecting to error page", slog.String("url", errorRedirectURL))
			ctx.Redirect(http.StatusMovedPermanently, errorRedirectURL)
			return
		}
		// set jwt token in jwt query
		query := flowURL.Query()
		query.Set("jwt_token", signed)
		flowURL.RawQuery = query.Encode()
		targetUrl = flowURL.String()
	}

	slog.Info("generate login redirect url", slog.Any("targetUrl", targetUrl))
	ctx.Redirect(http.StatusMovedPermanently, targetUrl)
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
// @Description  create a new user identity verification request
// @Tags         UserVerify
// @Accept       json
// @Produce      json
// @Param        body body types.UserVerifyReq true "User verification request body"
// @Success      200  {object}  types.Response{data=database.UserVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/verify [post]
func (h *UserHandler) CreateVerify(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.UserVerifyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.Username = currentUser
	orgVerify, err := h.uv.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Create organization Verify succeed", slog.String("real name", orgVerify.RealName))
	httpbase.OK(ctx, orgVerify)
}

// UpdateVerify godoc
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
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/verify/{id} [put]
func (h *UserHandler) UpdateVerify(ctx *gin.Context) {
	vID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var req types.UserVerifyStatusReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if req.Status != types.VerifyStatusRejected && req.Status != types.VerifyStatusApproved {
		slog.Error("Bad request format", slog.String("err", "Not allowed status"))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("not allowed status"), nil))
	}

	if req.Status == types.VerifyStatusRejected && req.Reason == "" {
		slog.Error("Bad request format", slog.String("err", "rejected need reason"))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("rejected need reason"), nil))

	}

	orgVerify, err := h.uv.Update(ctx, vID, req.Status, req.Reason)
	if err != nil {
		slog.Error("Failed to update organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("update organization Verify succeed", slog.String("real name", orgVerify.RealName))
	httpbase.OK(ctx, orgVerify)
}

// GetVerify godoc
// @Security     ApiKey
// @Summary      Get user verification
// @Description  get user verification information by user ID
// @Tags         UserVerify
// @Accept       json
// @Produce      json
// @Param        id             path  string true  "user UUID"
// @Success      200  {object}  types.Response{data=database.UserVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/verify/{id} [get]
func (h *UserHandler) GetVerify(ctx *gin.Context) {
	id := ctx.Param("id")
	orgVerify, err := h.uv.Get(ctx, id)
	if err != nil {
		slog.Error("Failed to get organization Verify", slog.Any("error", err))
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
		slog.Error("Invalid user labels update request", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("invalid request body"), nil))
		return
	}
	for _, label := range req.Labels {
		if !types.ValidLabels[label] {
			slog.Error("Invalid user labels update request", slog.String("label", label))
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("invalid request label"), nil))
			return
		}
	}
	req.OpUser = currentUser

	err := h.c.UpdateUserLabels(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to update user labels by uuid", slog.Any("error", err), slog.String("uid", req.UUID), slog.String("current_user", currentUser), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update labels by uuid succeed", slog.String("uid", req.UUID), slog.String("current_user", currentUser))
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
		slog.Error("Failed to get per and page", slog.Any("error", err))
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
		slog.Error("Failed to get all user emails", slog.Any("error", err))
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
		slog.Error("Failed to get all user emails", slog.Any("error", err))
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
		slog.Error("Failed to find user by uuids", slog.Any("error", err), slog.Any("uuids", uuids))
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

	// Check if user has organizations
	hasOrgs, err := h.c.CheckIfUserHasOrgs(ctx, userName)
	if err != nil {
		httpbase.ServerError(ctx, fmt.Errorf("failed to check if user has organzitions, error: %w", err))
		return
	}
	if hasOrgs {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("users who own organizations cannot be deleted"), nil))
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
	workflowClient := workflow.GetWorkflowClient()
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

	slog.Info("start user soft deletion workflow", slog.String("workflow_id", we.GetID()), slog.String("userName", userName), slog.String("operator", operator))

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
func (e *UserHandler) GenerateVerificationCodeAndSendEmail(ctx *gin.Context) {
	uid := httpbase.GetCurrentUserUUID(ctx)
	email := ctx.Param("email")
	err := e.c.GenerateVerificationCodeAndSendEmail(ctx, uid, email)
	if err != nil {
		if errors.Is(err, errorx.ErrUserNotFound) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("GenerateVerificationCodeAndSendEmail failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
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
func (e *UserHandler) ResetUserTags(ctx *gin.Context) {
	uid := httpbase.GetCurrentUserUUID(ctx)
	var req []int64
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("ResetUserTags failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if err := e.c.ResetUserTags(ctx, uid, req); err != nil {
		slog.Error("ResetUserTags failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// SendSmsCode godoc
// @Security     ApiKey
// @Summary      generate sms verification code and send it by sms
// @Description  generate sms verification code and send it by sms
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        body body types.SendSMSCodeRequest true "SendSMSCodeRequest"
// @Success      200  {object}  types.Response{data=types.SendSMSCodeResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/sms-code [post]
func (e *UserHandler) SendSMSCode(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	var req types.SendSMSCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("SendSMSCodeRequest failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}
	resp, err := e.c.SendSMSCode(ctx, currentUserUUID, req)
	if err != nil {
		slog.Error("SendSMSCode failed", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}

// UpdatePhone godoc
// @Security     ApiKey
// @Summary      Update current user phone
// @Description  Update current user phone
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        body body types.UpdateUserPhoneRequest true "UpdateUserPhoneRequest"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/phone [put]
func (e *UserHandler) UpdatePhone(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	var req types.UpdateUserPhoneRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to update user's phone", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}

	err := e.c.UpdatePhone(ctx, currentUserUUID, req)
	if err != nil {
		slog.Error("failed to update user's phone", slog.Any("err", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
