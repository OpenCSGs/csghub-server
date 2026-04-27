package component

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/crc32"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

var ErrUserNotFound = errors.New("user not found, please login first")

type AccessTokenComponent interface {
	Create(ctx context.Context, req *types.CreateUserTokenRequest) (*database.AccessToken, error)
	Delete(ctx context.Context, req *types.DeleteUserTokenRequest) error
	Check(ctx context.Context, req *types.CheckAccessTokenReq) (types.CheckAccessTokenResp, error)
	GetTokens(ctx context.Context, req *types.GetAccessTokenRequest) ([]types.CheckAccessTokenResp, error)
	RefreshToken(ctx context.Context, req *types.RefreshTokenReq) (types.CheckAccessTokenResp, error)
	GetOrCreateFirstAvaiToken(ctx context.Context, userName, app, tokenName string) (string, error)
	Update(ctx context.Context, req *types.UpdateAPIKeyRequest) (*types.CheckAccessTokenResp, error)
	GetAPIKeyQuotas(ctx context.Context, apiKey string) ([]database.AccountAccessTokenQuota, error)
	GetOrCreateBuiltinAPIKey(ctx context.Context, req *types.GetAccessTokenRequest) (*types.CheckAccessTokenResp, error)
}

func NewAccessTokenComponent(config *config.Config) (AccessTokenComponent, error) {
	var err error
	ac, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create accounting clent,error:%w", err)
	}
	c := &accessTokenComponentImpl{}
	c.ts = database.NewAccessTokenStore()
	c.us = database.NewUserStore()
	c.nsStore = database.NewNamespaceStore()
	c.orgStore = database.NewOrgStore()
	c.gs, err = git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create git server,error:%w", err)
	}
	c.acctClient = ac
	c.config = config
	c.mc, err = NewMemberComponent(config)
	c.tokenQuotaStore = database.NewAccountAccessTokenQuotaStore()
	c.accountBillStore = database.NewAccountBillStore()
	if err != nil {
		return nil, fmt.Errorf("fail to create member component,error:%w", err)
	}
	return c, nil
}

type accessTokenComponentImpl struct {
	ts               database.AccessTokenStore
	us               database.UserStore
	nsStore          database.NamespaceStore
	orgStore         database.OrgStore
	gs               gitserver.GitServer
	acctClient       accounting.AccountingClient
	config           *config.Config
	mc               MemberComponent
	tokenQuotaStore  database.AccountAccessTokenQuotaStore
	accountBillStore database.AccountBillStore
}

func (c *accessTokenComponentImpl) Create(ctx context.Context, req *types.CreateUserTokenRequest) (*database.AccessToken, error) {
	var (
		exist bool
		err   error
		user  database.User
	)

	if len(req.NSUUID) > 0 {
		// api keys as namespace scoped
		user, err = c.validateNamespacePermission(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to check namespace %s permission, error:%w", req.NSUUID, err)
		}
		// for check api key by uuid
		exist, err = c.ts.IsExistByUUID(ctx, req.NSUUID, req.TokenName, string(req.Application))
	} else {
		// support origin token create
		user, err = c.us.FindByUsername(ctx, req.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to find user, error:%w", err)
		}
		exist, err = c.ts.IsExist(ctx, req.Username, req.TokenName, string(req.Application))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to check if token exists,error:%w", err)
	}

	if exist {
		return nil, fmt.Errorf("token name duplicated, token_name:%s, app:%s", req.TokenName, req.Application)
	}

	var token *database.AccessToken
	var quota *database.AccountAccessTokenQuota
	// csghub token is shared with git server
	switch req.Application {
	case types.AccessTokenAppGit:
		if c.gs != nil {
			token, err = c.gs.CreateUserToken(req)
			if err != nil {
				return nil, fmt.Errorf("fail to create git user access token,error:%w", err)
			}
		} else {
			tokenContent := c.genUnique()
			token = &database.AccessToken{
				Name:        req.TokenName,
				Token:       tokenContent,
				UserID:      user.ID,
				Application: req.Application,
				Permission:  req.Permission,
				IsActive:    true,
			}
		}
		token.UserID = user.ID
		token.Application = req.Application
	case types.AccessTokenAppAIGateway:
		// Generate token value
		keyValue, err := generateOrgAPIKey("gk", 32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate api key, error:%w", err)
		}
		// Create the API key
		token = &database.AccessToken{
			Name:        req.TokenName,
			Token:       keyValue,
			Application: req.Application,
			Permission:  req.Permission,
			NsUUID:      req.NSUUID,
			IsActive:    true,
			UserID:      user.ID,
			TokenType:   types.AccessTokenTypeOwner,
		}
		quota, err = c.buildNewAccessTokenQuota(ctx, token, req)
		if err != nil {
			return nil, fmt.Errorf("failed to build API key quota, error: %w", err)
		}
	default:
		tokenValue := c.genUnique()
		token = &database.AccessToken{
			Name:        req.TokenName,
			Token:       tokenValue,
			UserID:      user.ID,
			Application: req.Application,
			Permission:  req.Permission,
			IsActive:    true,
			NsUUID:      req.NSUUID,
		}
	}

	if req.ExpiredAt.After(time.Now()) {
		token.ExpiredAt = req.ExpiredAt
	}

	err = c.createUserToken(ctx, token, user, quota)
	if err != nil {
		return nil, fmt.Errorf("fail to create database user access token,error:%w", err)
	}

	if req.Application == types.AccessTokenAppMirror {
		quota, err := c.acctClient.GetQuotaByID(req.Username)
		if err != nil {
			return nil, fmt.Errorf("fail to get quota by username,error:%w", err)
		}
		if quota == nil {
			_, err := c.acctClient.CreateOrUpdateQuota(req.Username, types.AcctQuotaReq{
				RepoCountLimit: c.config.MultiSync.DefaultRepoCountLimit,
				SpeedLimit:     c.config.MultiSync.DefaultSpeedLimit,
				TrafficLimit:   c.config.MultiSync.DefaultTrafficLimit,
			})
			if err != nil {
				return nil, fmt.Errorf("fail to create quota for new mirror token,error:%w", err)
			}
		}
	}

	return token, nil
}

func (c *accessTokenComponentImpl) genUnique() string {
	// TODO:change
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (c *accessTokenComponentImpl) Delete(ctx context.Context, req *types.DeleteUserTokenRequest) error {
	var (
		exist bool
		err   error
	)

	if len(req.NSUUID) > 0 {
		// for check api key by uuid
		token, err := c.ts.FindByID(ctx, req.ID)
		if err != nil {
			return fmt.Errorf("failed to check if token exists,error:%w", err)
		}
		if req.NSUUID != token.NsUUID {
			return errorx.ErrNotFound
		}
		if token.TokenType == types.AccessTokenTypeBuiltIn {
			return errorx.ErrUnauthorized
		}
		// check api keys as namespace scoped
		_, err = c.validateNamespacePermission(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to check namespace %s permission, error:%w", req.NSUUID, err)
		}
		exist = token != nil
	} else {
		// support origin token delete
		exist, err = c.us.IsExist(ctx, req.Username)
		if !exist {
			return fmt.Errorf("user does not exists,error:%w", err)
		}
		exist, err = c.ts.IsExist(ctx, req.Username, req.TokenName, string(req.Application))
		if err != nil {
			return fmt.Errorf("failed to check if token exists,error:%w", err)
		}
	}

	if !exist {
		return errorx.ErrNotFound
	}

	if req.Application == types.AccessTokenAppGit {
		err = c.gs.DeleteUserToken(req)
		if err != nil {
			return fmt.Errorf("failed to delete git user access token,error:%w", err)
		}
	}

	if len(req.NSUUID) > 0 {
		err = c.ts.DeleteByID(ctx, req.ID)
	} else {
		err = c.ts.Delete(ctx, req.Username, req.TokenName, string(req.Application))
	}
	if err != nil {
		return fmt.Errorf("failed to delete database user access token,error,error:%w", err)
	}
	return nil
}

func (c *accessTokenComponentImpl) Check(ctx context.Context, req *types.CheckAccessTokenReq) (types.CheckAccessTokenResp, error) {
	var resp types.CheckAccessTokenResp
	t, err := c.ts.FindByToken(ctx, req.Token, req.Application)
	if err != nil {
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			return resp, errorx.ErrNotFound
		}
		return resp, err
	}

	resp.Token = t.Token
	resp.TokenName = t.Name
	resp.Application = t.Application
	resp.Permission = t.Permission
	if t.User != nil {
		resp.Username = t.User.Username
		resp.UserUUID = t.User.UUID
	}
	resp.ExpireAt = t.ExpiredAt
	resp.NSUUID = t.NsUUID
	resp.CreatedAt = t.CreatedAt
	resp.UpdatedAt = t.UpdatedAt
	resp.TokenType = string(t.TokenType)
	return resp, nil
}

func (c *accessTokenComponentImpl) GetTokens(ctx context.Context, req *types.GetAccessTokenRequest) ([]types.CheckAccessTokenResp, error) {
	var resps []types.CheckAccessTokenResp
	var tokens []database.AccessToken
	var err error
	var user database.User

	if len(req.NSUUID) > 0 {
		// support api key tokens
		checkReq := &types.CreateUserTokenRequest{
			Username:    req.Username,
			OpUUID:      req.OpUUID,
			NSUUID:      req.NSUUID,
			Application: req.Application,
		}

		// api keys as namespace scoped
		user, err = c.validateNamespacePermission(ctx, checkReq)
		if err != nil {
			return nil, fmt.Errorf("failed to check namespace %s permission, error:%w", req.NSUUID, err)
		}
		_, err = c.checkOrCreateBuiltinAPIKey(ctx, req, user)
		if err != nil {
			return nil, err
		}
		tokens, err = c.ts.FindByNsUUID(ctx, req.NSUUID, string(req.Application))
		if err != nil {
			return nil, err
		}
	} else {
		// origin user tokens
		tokens, err = c.ts.FindByUser(ctx, req.Username, string(req.Application))
		if err != nil {
			return nil, err
		}
	}

	for _, t := range tokens {
		var resp types.CheckAccessTokenResp
		resp.ID = t.ID
		if len(req.NSUUID) > 0 {
			resp.Token = maskToken(t.Token)
		} else {
			resp.Token = t.Token
		}
		resp.TokenName = t.Name
		resp.Application = t.Application
		resp.Permission = t.Permission
		if t.User != nil {
			resp.Username = t.User.Username
			resp.UserUUID = t.User.UUID
		}
		resp.ExpireAt = t.ExpiredAt
		resp.NSUUID = t.NsUUID
		resp.CreatedAt = t.CreatedAt
		resp.UpdatedAt = t.UpdatedAt
		resp.TokenType = string(t.TokenType)

		quotas, err := c.tokenQuotaStore.FindByAPIKey(ctx, t.Token)
		if err != nil {
			slog.ErrorContext(ctx, "failed to find access token quota for API key %s, error: %w", t.Token, err)
		}
		if len(quotas) > 0 {
			resp.QuotaType = quotas[0].QuotaType
			resp.QuotaValueType = quotas[0].ValueType
			resp.Usage = quotas[0].Usage
			resp.Quota = quotas[0].Quota
			resp.LastUsedAt = quotas[0].LastUsedAt
		}
		resps = append(resps, resp)
	}
	return resps, nil
}

func (c *accessTokenComponentImpl) RefreshToken(ctx context.Context, refreshReq *types.RefreshTokenReq) (types.CheckAccessTokenResp, error) {
	var resp types.CheckAccessTokenResp
	var newToken *database.AccessToken
	var t *database.AccessToken
	var err error

	if len(refreshReq.NSUUID) > 0 {
		checkReq := &types.CreateUserTokenRequest{
			Username: refreshReq.Username,
			OpUUID:   refreshReq.OpUUID,
			NSUUID:   refreshReq.NSUUID,
		}
		// Validate org namespace and admin permission
		_, err := c.validateNamespacePermission(ctx, checkReq)
		if err != nil {
			return resp, err
		}
		t, err = c.ts.FindBuiltinByNsUUID(ctx, refreshReq.NSUUID, refreshReq.App)
		if err != nil {
			if errors.Is(err, errorx.ErrDatabaseNoRows) {
				return types.CheckAccessTokenResp{}, errorx.ErrNotFound
			}
			return types.CheckAccessTokenResp{}, err
		}
	} else {
		t, err = c.ts.FindByTokenName(ctx, refreshReq.Username, refreshReq.TokenName, refreshReq.App)
		if err != nil {
			if errors.Is(err, errorx.ErrDatabaseNoRows) {
				return types.CheckAccessTokenResp{}, errorx.ErrNotFound
			}
			return types.CheckAccessTokenResp{}, err
		}
	}

	var newTokenValue string
	req := &types.CreateUserTokenRequest{
		Username:    refreshReq.Username,
		TokenName:   t.Name,
		Application: t.Application,
		Permission:  t.Permission,
	}
	// csghub token is shared with git server
	if req.Application == "" || req.Application == types.AccessTokenAppCSGHub {
		// TODO:allow git client to refresh token
		// git server cannot create tokens with the same nanme
		err := c.gs.DeleteUserToken(&types.DeleteUserTokenRequest{
			Username:  refreshReq.Username,
			TokenName: t.Name,
		})
		if err != nil {
			return resp, fmt.Errorf("fail to delete old git user access token,error:%w", err)
		}
		newToken, err := c.gs.CreateUserToken(req)
		if err != nil {
			return resp, fmt.Errorf("fail to create git user access token,error:%w", err)
		}
		newTokenValue = newToken.Token
	} else if req.Application == types.AccessTokenAppAIGateway {
		keyValue, err := generateOrgAPIKey("gk", 32)
		if err != nil {
			return types.CheckAccessTokenResp{}, fmt.Errorf("failed to generate builtin api key, error: %w", err)
		}
		newTokenValue = keyValue
	} else {
		newTokenValue = c.genUnique()
	}

	if len(refreshReq.NSUUID) > 0 {
		newToken = t
		newToken.Token = newTokenValue
		err = c.ts.UpdateToken(ctx, newToken)
	} else {
		newToken, err = c.ts.Refresh(ctx, t, newTokenValue, refreshReq.NewExpiredAt)
	}
	if err != nil {
		return resp, fmt.Errorf("failed to update access token with new token value,error:%w", err)
	}

	resp.ID = newToken.ID
	resp.Token = newToken.Token
	resp.TokenName = newToken.Name
	resp.Application = newToken.Application
	resp.Permission = newToken.Permission
	if newToken.User != nil {
		resp.Username = newToken.User.Username
		resp.UserUUID = newToken.User.UUID
	}
	resp.ExpireAt = newToken.ExpiredAt
	resp.TokenType = string(newToken.TokenType)
	resp.NSUUID = newToken.NsUUID
	resp.CreatedAt = newToken.CreatedAt
	resp.UpdatedAt = newToken.UpdatedAt

	return resp, nil
}

func (c *accessTokenComponentImpl) GetOrCreateFirstAvaiToken(ctx context.Context, userName, app, tokenName string) (string, error) {
	tokenReq := &types.GetAccessTokenRequest{
		Username:    userName,
		Application: types.AccessTokenApp(app),
	}

	tokens, err := c.GetTokens(ctx, tokenReq)
	if err != nil {
		return "", fmt.Errorf("failed to select user %s access %s tokens, error:%w", userName, app, err)
	}
	if len(tokens) > 0 {
		return tokens[0].Token, nil
	}

	req := types.CreateUserTokenRequest{
		Username:    userName,
		TokenName:   tokenName,
		Application: types.AccessTokenApp(app),
		Permission:  "",
	}

	token, err := c.Create(ctx, &req)
	if err != nil {
		return "", fmt.Errorf("failed to create user %s access %s token, error:%w", userName, app, err)
	}

	return token.Token, nil
}

func (c *accessTokenComponentImpl) createUserToken(ctx context.Context, newToken *database.AccessToken, user database.User, quota *database.AccountAccessTokenQuota) error {
	var quotas []database.AccountAccessTokenQuota
	if quota != nil {
		quotas = []database.AccountAccessTokenQuota{*quota}
	}
	err := c.ts.Create(ctx, newToken, quotas)
	if err != nil {
		return fmt.Errorf("fail to create user %s new %s token, error: %w", user.Username, newToken.Application, err)
	}

	if newToken.Application == types.AccessTokenAppStarship {
		// charge 100 credit for create starship token by call accounting service
		err = c.presentForNewAccessToken(user)
		if err != nil {
			slog.ErrorContext(ctx, "fail to charge for new starship user with retry 3 times", slog.Any("user.uuid", user.UUID), slog.Any("err", err))
		}
	}

	return nil
}

func (c *accessTokenComponentImpl) presentForNewAccessToken(user database.User) error {
	var err error
	req := types.ACTIVITY_REQ{
		ID:     types.StarShipNewUser.ID,
		Value:  types.StarShipNewUser.Value,
		OpUID:  user.Username,
		OpDesc: types.StarShipNewUser.OpDesc,
	}
	// retry 3 time
	for i := 0; i < 3; i++ {
		_, err = c.acctClient.PresentAccountingUser(user.UUID, req)
		if err == nil {
			return nil
		}
	}
	return err
}

func (c *accessTokenComponentImpl) validateNamespacePermission(ctx context.Context, req *types.CreateUserTokenRequest) (database.User, error) {
	// Validate that the UUID is a valid namespace UUID
	ns, err := c.nsStore.FindByUUID(ctx, req.NSUUID)
	if err != nil {
		return database.User{}, fmt.Errorf("failed to find namespace by uuid, uuid: %s, error: %w", req.NSUUID, err)
	}

	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return database.User{}, fmt.Errorf("failed to find user by username: %s error: %w", req.Username, err)
	}

	if user.CanAdmin() {
		return user, nil
	}

	if ns.NamespaceType == database.UserNamespace {
		// user namespace must match user username for user's apikeys
		if ns.Path == user.Username {
			return user, nil
		} else {
			return database.User{}, fmt.Errorf("namespace path %s does not match user %s", ns.Path, user.Username)
		}
	}

	// Check if current user is admin of the org
	role, err := c.mc.GetMemberRole(ctx, ns.Path, req.Username)
	if err != nil {
		return database.User{}, fmt.Errorf("failed to get member role, org: %s, user: %s, error: %w", ns.Path, req.Username, err)
	}
	if !role.CanAdmin() {
		return database.User{}, errorx.ErrForbiddenMsg("current user does not have permission to manage API keys in this organization")
	}

	return user, nil
}

func generateOrgAPIKey(prefix string, length int) (string, error) {
	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate api key random bytes: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)
	hash := crc32.ChecksumIEEE(randomBytes)
	checksum := fmt.Sprintf("%08x", hash)

	var keyParts []string
	keyParts = append(keyParts, prefix)
	keyParts = append(keyParts, encoded)
	keyParts = append(keyParts, checksum)

	rawKey := strings.Join(keyParts, "_")
	return rawKey, nil
}

func (c *accessTokenComponentImpl) Update(ctx context.Context, req *types.UpdateAPIKeyRequest) (*types.CheckAccessTokenResp, error) {
	// Get the API key by ID
	token, err := c.ts.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token by id %d, error: %w", req.ID, err)
	}
	if req.NSUUID != token.NsUUID {
		return nil, errorx.ErrNotFound
	}

	checkReq := &types.CreateUserTokenRequest{
		Username: req.CurrentUser,
		OpUUID:   req.OpUUID,
		NSUUID:   req.NSUUID,
	}

	// Validate org namespace and admin permission
	_, err = c.validateNamespacePermission(ctx, checkReq)
	if err != nil {
		return nil, err
	}

	if !token.IsActive {
		return nil, fmt.Errorf("token is inactive")
	}

	if token.TokenType != types.AccessTokenTypeBuiltIn {
		// Update fields except built-in token type
		if req.KeyName != nil {
			token.Name = *req.KeyName
		}
		if req.ExpiredAt != nil {
			token.ExpiredAt = *req.ExpiredAt
		}
	}

	quota, err := c.updateAccessTokenQuota(ctx, token, req)
	if err != nil {
		return nil, fmt.Errorf("failed to build API key quota, error: %w", err)
	}

	result, err := c.ts.UpdateTokenAndQuota(ctx, token, quota)
	if err != nil {
		return nil, fmt.Errorf("failed to update token, error: %w", err)
	}

	resp := types.CheckAccessTokenResp{
		ID:             result.ID,
		Token:          maskToken(result.Token),
		TokenName:      result.Name,
		Application:    result.Application,
		ExpireAt:       result.ExpiredAt,
		NSUUID:         result.NsUUID,
		QuotaType:      quota.QuotaType,
		QuotaValueType: quota.ValueType,
		Usage:          quota.Usage,
		Quota:          quota.Quota,
		LastUsedAt:     quota.LastUsedAt,
		CreatedAt:      result.CreatedAt,
		UpdatedAt:      result.UpdatedAt,
		TokenType:      string(result.TokenType),
	}

	return &resp, nil
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return "*****"
	}
	return token[:2] + "***" + token[len(token)-2:]
}

func (c *accessTokenComponentImpl) GetOrCreateBuiltinAPIKey(ctx context.Context, req *types.GetAccessTokenRequest) (*types.CheckAccessTokenResp, error) {
	checkReq := &types.CreateUserTokenRequest{
		Username: req.Username,
		OpUUID:   req.OpUUID,
		NSUUID:   req.NSUUID,
	}

	// Validate org namespace and admin permission
	user, err := c.validateNamespacePermission(ctx, checkReq)
	if err != nil {
		return nil, err
	}

	defaultToken, err := c.checkOrCreateBuiltinAPIKey(ctx, req, user)
	if err != nil {
		return nil, err
	}

	resp := &types.CheckAccessTokenResp{
		ID:          defaultToken.ID,
		Token:       defaultToken.Token,
		TokenName:   defaultToken.Name,
		Application: defaultToken.Application,
		NSUUID:      defaultToken.NsUUID,
		CreatedAt:   defaultToken.CreatedAt,
		UpdatedAt:   defaultToken.UpdatedAt,
		TokenType:   string(defaultToken.TokenType),
	}
	return resp, nil
}

func (c *accessTokenComponentImpl) checkOrCreateBuiltinAPIKey(ctx context.Context, req *types.GetAccessTokenRequest, user database.User) (*database.AccessToken, error) {
	// Try to find existing builtin API key
	token, err := c.ts.FindBuiltinByNsUUID(ctx, req.NSUUID, string(types.AccessTokenAppAIGateway))
	if err == nil && token != nil {
		return token, nil
	}

	keyValue, err := generateOrgAPIKey("gk", 32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate builtin api key, error: %w", err)
	}

	newToken := &database.AccessToken{
		Name:        string(types.AccessTokenTypeBuiltIn),
		Token:       keyValue,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      req.NSUUID,
		IsActive:    true,
		UserID:      user.ID,
		TokenType:   types.AccessTokenTypeBuiltIn,
	}

	err = c.ts.Create(ctx, newToken, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create builtin api key, error: %w", err)
	}

	return newToken, nil
}
