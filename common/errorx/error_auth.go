package errorx

import (
	"errors"
)

const errAuthPrefix = "AUTH-ERR"

const (
	unauthorized = iota
	userNotFound
	forbidden
	noEmail
	invalidJWT
	invalidAuthHeader
	notAdmin
	userNotMatch
	needUUID
	needAPIKey
)

var (
	// --- Auth-ERR-xxxx: User and Permission related errors ---

	// not allowed for anoymous user (need to login first)
	//
	// Description: The user is not logged in. Please log in to access this resource.
	//
	// Description_ZH: 用户没有登录，请登录后访问资源
	//
	// en-US: Unauthorized
	//
	// zh-CN: 未授权访问
	//
	// zh-HK: 未授權
	ErrUnauthorized error = CustomError{prefix: errAuthPrefix, code: unauthorized}

	// cannot find corresponding user
	//
	// Description: The user account specified could not be found.
	//
	// Description_ZH: 找不到指定的用户帐户。
	//
	// en-US: User not found, please login first
	//
	// zh-CN: 用户未找到，请先登录
	//
	// zh-HK: 用戶未找到，請先登入
	ErrUserNotFound error = CustomError{prefix: errAuthPrefix, code: userNotFound}

	// not enough permission for current user
	//
	// Description: The current user does not have sufficient permissions to perform this action.
	//
	// Description_ZH: 当前用户没有足够的权限来执行此操作。
	//
	// en-US: Access forbidden, insufficient permissions
	//
	// zh-CN: 访问被禁止，权限不足
	//
	// zh-HK: 權限被拒絕
	ErrForbidden error = CustomError{prefix: errAuthPrefix, code: forbidden}

	// user account has no email address
	//
	// Description: The user's account does not have an associated email address, which is required for this operation.
	//
	// Description_ZH: 用户的帐户没有关联的电子邮件地址，而此操作需要该地址。
	//
	// en-US: Email address is required, please set your email first
	//
	// zh-CN: 需要设置邮箱地址
	//
	// zh-HK: 需要設置電郵地址
	ErrNoEmail error = CustomError{prefix: errAuthPrefix, code: noEmail}

	// provided JWT is invalid or expired
	//
	// Description: The authentication token (JWT) is malformed, invalid, or has expired. Please log in again.
	//
	// Description_ZH: 身份验证令牌（JWT）格式错误、无效或已过期。请重新登录。
	//
	// en-US: Invalid JWT token
	//
	// zh-CN: 无效的JWT令牌
	//
	// zh-HK: 無效的JWT令牌
	ErrInvalidJWT error = CustomError{prefix: errAuthPrefix, code: invalidJWT}

	// authorization header is invalid or malformed
	//
	// Description: The Authorization header is missing or incorrectly formatted. It should typically be in the format 'Bearer {token}'.
	//
	// Description_ZH: Authorization请求头缺失或格式不正确。通常应为 'Bearer {token}' 格式。
	//
	// en-US: Invalid authorization header
	//
	// zh-CN: 无效的授权请求头
	//
	// zh-HK: 無效的授權標頭
	ErrInvalidAuthHeader error = CustomError{prefix: errAuthPrefix, code: invalidAuthHeader}

	// user is not an administrator
	//
	// Description: This operation requires administrator privileges, but the current user is not an administrator.
	//
	// Description_ZH: 此操作需要管理员权限，但当前用户不是管理员。
	//
	// en-US: Only admin user can access
	//
	// zh-CN: 需要管理员权限
	//
	// zh-HK: 僅管理員用戶可以訪問
	ErrUserNotAdmin error = CustomError{prefix: errAuthPrefix, code: notAdmin}

	// authenticated user does not match the target user
	//
	// Description: You can only perform this action on your own account.
	//
	// Description_ZH: 您只能在自己的账户上执行此操作。
	//
	// en-US: User not match, try to query user account not owned
	//
	// zh-CN: 用户身份不匹配
	//
	// zh-HK: 用戶不匹配，嘗試查詢不屬於自己的用戶賬戶
	ErrUserNotMatch error = CustomError{prefix: errAuthPrefix, code: userNotMatch}

	// request is missing user UUID
	//
	// Description: The request must include the user's UUID in the header or body to identify the target account.
	//
	// Description_ZH: 请求必须在请求头或正文中包含用户的UUID以识别目标账户。
	//
	// en-US: UUID is required to identify user account
	//
	// zh-CN: 需要提供用户UUID
	//
	// zh-HK: 需要用户uuid
	ErrNeedUUID error = CustomError{prefix: errAuthPrefix, code: needUUID} // need uuid in request header or body to identify user account

	// request is missing API Key
	//
	// Description: The request must include an API Key in the header or body for authentication.
	//
	// Description_ZH: 请求必须在请求头或正文中包含API密钥以进行身份验证。
	//
	// en-US: Need API key for authentication
	//
	// zh-CN: 需要提供API密钥进行身份验证
	//
	// zh-HK: 需要API密鑰
	ErrNeedAPIKey error = CustomError{prefix: errAuthPrefix, code: needAPIKey} // need api key in request header or body to identify user account
)

/*
func ErrUnauthorized() errAuth {
	return errUnauthorized
}

func ErrUserNotFound() errAuth {
	return errUserNotFound
}

func ErrPermissionDenied() errAuth {
	return errPermissionDenied
}

func ErrForbidden() errAuth {
	return errForbidden
}
*/

func InvalidJWT(err error, errCtx context) error {
	customErr := CustomError{
		prefix:  errAuthPrefix,
		code:    invalidJWT,
		err:     err,
		context: errCtx,
	}
	return customErr
}

func InvalidAuthHeader(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    invalidAuthHeader,
		err:     err,
		context: errCtx,
	}
}

func UserNotFound(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    userNotFound,
		err:     err,
		context: errCtx,
	}
}

func UserNotMatch(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    userNotMatch,
		err:     err,
		context: errCtx,
	}
}

func NeedUUID(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    needUUID,
		err:     err,
		context: errCtx,
	}
}

func NeedAPIKey(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    needAPIKey,
		err:     err,
		context: errCtx,
	}
}

func UserNotAdmin(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    notAdmin,
		err:     err,
		context: errCtx,
	}
}

func Forbidden(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    forbidden,
		err:     err,
		context: errCtx,
	}
}

func NoEmail(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    noEmail,
		err:     err,
		context: errCtx,
	}
}

func Unauthorized(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    unauthorized,
		err:     err,
		context: errCtx,
	}
}

// ErrForbiddenMsg returns a new ErrForbidden with extra message
func ErrForbiddenMsg(msg string) error {

	return CustomError{
		prefix:  errAuthPrefix,
		code:    forbidden,
		err:     errors.New(msg),
		context: nil,
	}
}
