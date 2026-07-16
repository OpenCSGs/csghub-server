// Package errorx provides structured error types and error codes for the Federation Adapter module.
// This file defines the FEDAP-ERR error code family used across the Federation Adapter service.
//
// Error codes follow the project convention: each error has a unique iota-based code under
// the "FEDAP-ERR" prefix, an exported sentinel variable for errors.Is() matching, and a
// factory function that wraps a cause error with contextual metadata.
package errorx

// errFederationAdapterPrefix is the error code prefix for all Federation Adapter errors.
// The literal FEDAP-ERR value is retained for compatibility with existing external error codes.
const errFederationAdapterPrefix = "FEDAP-ERR"

const (
	tokenExpired = iota
	tokenExchangeFailed
	siteFetchFailed
	siteUnavailable
	oauthAuthenticationFailed
	invalidToken
	userInfoFetchFailed
	proxyRequestProcessFailed
	oauthAccessDenied
	oauthCredentialProcessingFailed
	federationAdapterUnauthorized
	federationAdapterSyncRepoFailed
	applicationScopesFetchFailed
	federationAdapterRepositoryAlreadyExists
)

// Sentinel error variables for use with errors.Is().
// These carry no wrapped error or context; use the factory functions below
// when you need to attach a cause or contextual metadata.
var (
	// Description: Both access token and refresh token have expired. Please re-authorize to continue accessing the remote site.
	//
	// Description_ZH: 访问令牌和刷新令牌均已过期，请重新授权以继续访问远端资源。
	//
	// en-US: Token expired, please re-authorize
	//
	// zh-CN: 令牌已过期，请重新授权
	//
	// zh-HK: 令牌已過期，請重新授權
	ErrTokenExpired error = CustomError{prefix: errFederationAdapterPrefix, code: tokenExpired}

	// Description: Failed to exchange the access token for a scoped token via RFC 8693 Token Exchange.
	//
	// Description_ZH: 通过 RFC 8693 Token Exchange 兑换受限令牌失败。
	//
	// en-US: Token exchange failed
	//
	// zh-CN: 令牌兑换失败
	//
	// zh-HK: 令牌兌換失敗
	ErrTokenExchangeFailed error = CustomError{prefix: errFederationAdapterPrefix, code: tokenExchangeFailed}

	// Description: Failed to load the specified federation site configuration. The site may not exist, or the upstream site registry lookup may have failed.
	//
	// Description_ZH: 获取指定的联邦对端配置失败。可能是该对端不存在，或上游站点注册表查询失败。
	//
	// en-US: Failed to get federation site
	//
	// zh-CN: 获取联邦对端失败
	//
	// zh-HK: 獲取聯邦對端失敗
	ErrSiteFetchFailed error = CustomError{prefix: errFederationAdapterPrefix, code: siteFetchFailed}

	// Description: The remote federation site service is currently unavailable.
	//
	// Description_ZH: 远端联邦服务当前不可用。
	//
	// en-US: Federation site unavailable
	//
	// zh-CN: 联邦对端服务不可用
	//
	// zh-HK: 聯邦對端服務不可用
	ErrSiteUnavailable error = CustomError{prefix: errFederationAdapterPrefix, code: siteUnavailable}

	// Description: A general error occurred during the OAuth authentication flow that does not fall into a more specific federation adapter error category.
	//
	// Description_ZH: OAuth 认证流程中发生了通用错误，且不属于更具体的联邦适配器错误分类。
	//
	// en-US: OAuth authentication failed
	//
	// zh-CN: OAuth 认证失败
	//
	// zh-HK: OAuth 認證失敗
	ErrOAuthAuthenticationFailed error = CustomError{prefix: errFederationAdapterPrefix, code: oauthAuthenticationFailed}

	// Description: The provided token is invalid or unauthorized and cannot be used to access the requested resource. It may be malformed, rejected by the remote service, or otherwise not accepted.
	//
	// Description_ZH: 提供的令牌无效、未授权或格式错误，无法用于访问请求的资源。它可能格式不正确、被远端服务拒绝，或因其他原因不被接受。
	//
	// en-US: Invalid token
	//
	// zh-CN: 无效令牌
	//
	// zh-HK: 無效令牌
	ErrInvalidToken error = CustomError{prefix: errFederationAdapterPrefix, code: invalidToken}

	// Description: Failed to fetch or parse user information from the remote OAuth provider after token exchange.
	//
	// Description_ZH: 在令牌兑换成功后，从远端 OAuth 提供方获取或解析用户信息失败。
	//
	// en-US: Failed to fetch user information
	//
	// zh-CN: 获取用户信息失败
	//
	// zh-HK: 獲取用戶信息失敗
	ErrUserInfoFetchFailed error = CustomError{prefix: errFederationAdapterPrefix, code: userInfoFetchFailed}

	// Description: Failed to process the fedap proxy request. This includes proxy URL build failures,
	// outbound request construction or execution failures, upstream response handling failures,
	// and response header processing failures.
	//
	// Description_ZH: 处理 fedap 代理请求失败。包括代理 URL 构建失败、外发请求构建或执行失败、
	// 上游响应处理失败，以及响应头处理失败等通用代理处理错误。
	//
	// en-US: Failed to process fedap proxy request
	//
	// zh-CN: 处理 fedap 代理请求失败
	//
	// zh-HK: 處理 fedap 代理請求失敗
	ErrProxyRequestProcess error = CustomError{prefix: errFederationAdapterPrefix, code: proxyRequestProcessFailed}

	// Description: The upstream OAuth provider returned access_denied because the user declined the authorization request.
	//
	// Description_ZH: 上游 OAuth 提供方返回 access_denied，表示用户拒绝了授权请求。
	//
	// en-US: OAuth access denied
	//
	// zh-CN: OAuth 授权被拒绝
	//
	// zh-HK: OAuth 授權被拒絕
	ErrOAuthAccessDenied error = CustomError{prefix: errFederationAdapterPrefix, code: oauthAccessDenied}

	// Description: Failed to process locally issued OAuth credentials, such as encrypting tokens or persisting authorization records.
	//
	// Description_ZH: 处理本地 OAuth 凭证失败，例如加密令牌或持久化授权记录失败。
	//
	// en-US: OAuth credential processing failed
	//
	// zh-CN: OAuth 凭证处理失败
	//
	// zh-HK: OAuth 憑證處理失敗
	ErrOAuthCredentialProcessingFailed error = CustomError{prefix: errFederationAdapterPrefix, code: oauthCredentialProcessingFailed}

	// Description: The current user has not authorized the requested federation site, or the local authorization is not usable.
	//
	// Description_ZH: 当前用户未授权请求的联邦对端，或本地授权不可用。
	//
	// en-US: Federation adapter authorization required
	//
	// zh-CN: 需要完成联邦适配器授权
	//
	// zh-HK: 需要完成聯邦適配器授權
	ErrFederationAdapterUnauthorized error = CustomError{prefix: errFederationAdapterPrefix, code: federationAdapterUnauthorized}

	// Description: Failed to sync the remote repository into a local repository, or failed to query the repository sync status.
	//
	// Description_ZH: 同步远端仓库到本地仓库失败，或查询仓库同步状态失败。
	//
	// en-US: Failed to sync repository
	//
	// zh-CN: 同步仓库失败
	//
	// zh-HK: 同步倉庫失敗
	ErrFederationAdapterSyncRepoFailed error = CustomError{prefix: errFederationAdapterPrefix, code: federationAdapterSyncRepoFailed}

	// Description: Failed to fetch custom scopes from the remote Casdoor application. This may be caused by an unreachable server, invalid application ID, or an unexpected server response.
	//
	// Description_ZH: 从远端 Casdoor 应用获取自定义权限范围失败，可能是因为服务器不可达、应用 ID 无效或服务器返回异常。
	//
	// en-US: Failed to fetch application scopes
	//
	// zh-CN: 获取应用权限范围失败
	//
	// zh-HK: 獲取應用權限範圍失敗
	ErrApplicationScopesFetchFailed error = CustomError{prefix: errFederationAdapterPrefix, code: applicationScopesFetchFailed}

	// Description: The requested federation repository already exists locally, or the existing federation sync mapping conflicts with the requested repository.
	//
	// Description_ZH: 请求的联邦仓库已在本地存在，或已有联邦同步映射与请求的仓库冲突。
	//
	// en-US: Repository already exists
	//
	// zh-CN: 仓库已存在
	//
	// zh-HK: 倉庫已存在
	ErrFederationAdapterRepositoryAlreadyExists error = CustomError{prefix: errFederationAdapterPrefix, code: federationAdapterRepositoryAlreadyExists}
)

// TokenExpiredErr wraps err as a token-expired error with optional context.
func TokenExpiredErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: tokenExpired, err: err, context: ctx}
}

// TokenExchangeFailedErr wraps err as an RFC 8693 Token Exchange failure with optional context.
func TokenExchangeFailedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: tokenExchangeFailed, err: err, context: ctx}
}

// SiteFetchFailedErr wraps err as a site-fetch-failed error with optional context.
func SiteFetchFailedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: siteFetchFailed, err: err, context: ctx}
}

// SiteUnavailableErr wraps err as a site-unavailable error with optional context.
func SiteUnavailableErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: siteUnavailable, err: err, context: ctx}
}

// OAuthAuthenticationFailedErr wraps err as a general OAuth authentication flow failure with optional context.
func OAuthAuthenticationFailedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: oauthAuthenticationFailed, err: err, context: ctx}
}

// InvalidTokenErr wraps err as an invalid-token error with optional context.
func InvalidTokenErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: invalidToken, err: err, context: ctx}
}

// UserInfoFetchFailedErr wraps err as a userinfo-fetch-failed error with optional context.
func UserInfoFetchFailedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: userInfoFetchFailed, err: err, context: ctx}
}

// ProxyRequestProcessErr wraps err as a general fedap proxy request processing failure with optional context.
func ProxyRequestProcessErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: proxyRequestProcessFailed, err: err, context: ctx}
}

// OAuthAccessDeniedErr wraps err as an OAuth-access-denied error with optional context.
func OAuthAccessDeniedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: oauthAccessDenied, err: err, context: ctx}
}

// OAuthCredentialProcessingFailedErr wraps err as an OAuth-credential-processing-failed error with optional context.
func OAuthCredentialProcessingFailedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: oauthCredentialProcessingFailed, err: err, context: ctx}
}

// FederationAdapterUnauthorizedErr wraps err as a federation-adapter unauthorized error with optional context.
func FederationAdapterUnauthorizedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: federationAdapterUnauthorized, err: err, context: ctx}
}

// FederationAdapterSyncRepoFailedErr wraps err as a repository sync failure with optional context.
func FederationAdapterSyncRepoFailedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: federationAdapterSyncRepoFailed, err: err, context: ctx}
}

// ApplicationScopesFetchFailedErr wraps err as an application-scopes-fetch-failed error with optional context.
func ApplicationScopesFetchFailedErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: applicationScopesFetchFailed, err: err, context: ctx}
}

// FederationAdapterRepositoryAlreadyExistsErr wraps err as a federation repository conflict with optional context.
func FederationAdapterRepositoryAlreadyExistsErr(err error, ctx context) error {
	return CustomError{prefix: errFederationAdapterPrefix, code: federationAdapterRepositoryAlreadyExists, err: err, context: ctx}
}
