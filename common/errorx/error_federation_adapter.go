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
	// tokenExpired means both access_token and refresh_token have expired; re-authorization is required.
	//
	// Description: Both access token and refresh token have expired. Please re-authorize to continue accessing the remote site.
	//
	// Description_ZH: 访问令牌和刷新令牌均已过期，请重新授权以继续访问远端资源。
	//
	// en-US: Token expired, please re-authorize
	//
	// zh-CN: 令牌已过期，请重新授权
	//
	// zh-HK: 令牌已過期，請重新授權
	tokenExpired = iota

	// tokenExchangeFailed means the RFC 8693 Token Exchange request failed.
	//
	// Description: Failed to exchange the access token for a scoped token via RFC 8693 Token Exchange.
	//
	// Description_ZH: 通过 RFC 8693 Token Exchange 兑换受限令牌失败。
	//
	// en-US: Token exchange failed
	//
	// zh-CN: 令牌兑换失败
	//
	// zh-HK: 令牌兌換失敗
	tokenExchangeFailed

	// siteFetchFailed means loading the federation site configuration failed.
	//
	// Description: Failed to load the specified federation site configuration. The site may not exist, or the upstream site registry lookup may have failed.
	//
	// Description_ZH: 获取指定的联邦对端配置失败。可能是该对端不存在，或上游站点注册表查询失败。
	//
	// en-US: Failed to get federation site
	//
	// zh-CN: 获取联邦对端失败
	//
	// zh-HK: 獲取聯邦對端失敗
	siteFetchFailed

	// siteUnavailable means the remote site service is unreachable.
	//
	// Description: The remote federation site service is currently unavailable.
	//
	// Description_ZH: 远端联邦服务当前不可用。
	//
	// en-US: Federation site unavailable
	//
	// zh-CN: 联邦对端服务不可用
	//
	// zh-HK: 聯邦對端服務不可用
	siteUnavailable

	// oauthAuthenticationFailed means a general OAuth authentication flow error occurred.
	//
	// Description: A general error occurred during the OAuth authentication flow that does not fall into a more specific federation adapter error category.
	//
	// Description_ZH: OAuth 认证流程中发生了通用错误，且不属于更具体的联邦适配器错误分类。
	//
	// en-US: OAuth authentication failed
	//
	// zh-CN: OAuth 认证失败
	//
	// zh-HK: OAuth 認證失敗
	oauthAuthenticationFailed

	// invalidToken means the provided token is unauthorized, malformed, or otherwise unusable.
	// This error is different from tokenExpired and should be used when the token itself is not accepted.
	//
	// Description: The provided token is invalid or unauthorized and cannot be used to access the requested resource. It may be malformed, rejected by the remote service, or otherwise not accepted.
	//
	// Description_ZH: 提供的令牌无效、未授权或格式错误，无法用于访问请求的资源。它可能格式不正确、被远端服务拒绝，或因其他原因不被接受。
	//
	// en-US: Invalid token
	//
	// zh-CN: 无效令牌
	//
	// zh-HK: 無效令牌
	invalidToken

	// userInfoFetchFailed means the OAuth callback succeeded in exchanging a token but failed
	// to retrieve or parse the remote Casdoor userinfo payload.
	//
	// Description: Failed to fetch or parse user information from the remote OAuth provider after token exchange.
	//
	// Description_ZH: 在令牌兑换成功后，从远端 OAuth 提供方获取或解析用户信息失败。
	//
	// en-US: Failed to fetch user information
	//
	// zh-CN: 获取用户信息失败
	//
	// zh-HK: 獲取用戶信息失敗
	userInfoFetchFailed

	// proxyRequestProcessFailed means a general error occurred while processing a fedap proxy request.
	//
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
	proxyRequestProcessFailed
)

// Sentinel error variables for use with errors.Is().
// These carry no wrapped error or context; use the factory functions below
// when you need to attach a cause or contextual metadata.
var (
	// ErrTokenExpired indicates both access and refresh tokens have expired.
	ErrTokenExpired error = CustomError{prefix: errFederationAdapterPrefix, code: tokenExpired}
	// ErrTokenExchangeFailed indicates RFC 8693 Token Exchange failure.
	ErrTokenExchangeFailed error = CustomError{prefix: errFederationAdapterPrefix, code: tokenExchangeFailed}
	// ErrSiteFetchFailed indicates loading the federation site configuration failed.
	ErrSiteFetchFailed error = CustomError{prefix: errFederationAdapterPrefix, code: siteFetchFailed}
	// ErrSiteUnavailable indicates the remote site service is unreachable.
	ErrSiteUnavailable error = CustomError{prefix: errFederationAdapterPrefix, code: siteUnavailable}
	// ErrOAuthAuthenticationFailed indicates a general OAuth authentication flow failure.
	ErrOAuthAuthenticationFailed error = CustomError{prefix: errFederationAdapterPrefix, code: oauthAuthenticationFailed}
	// ErrInvalidToken indicates the provided token is invalid, unauthorized, malformed, or otherwise unusable.
	ErrInvalidToken error = CustomError{prefix: errFederationAdapterPrefix, code: invalidToken}
	// ErrUserInfoFetchFailed indicates userinfo retrieval or parsing failed after token exchange.
	ErrUserInfoFetchFailed error = CustomError{prefix: errFederationAdapterPrefix, code: userInfoFetchFailed}
	// ErrProxyRequestProcess indicates a general fedap proxy request processing failure.
	ErrProxyRequestProcess error = CustomError{prefix: errFederationAdapterPrefix, code: proxyRequestProcessFailed}
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
