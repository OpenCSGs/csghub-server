package errorx

import (
	"database/sql"
	"errors"
	"strings"
)

// used to check error type

const errSysPrefix = "SYS-ERR"

const (
	// --- SYS-ERR-xxx: System / Service exceptions ---
	internalServerError = iota
	remoteServiceFail
	// When select in DB, encounter connection failure or other error
	databaseFailure
	// Replace sql.ErrNoRows
	databaseNoRows
	databaseDuplicateKey

	lfsNotFound

	lastOrgAdmin

	cannotPromoteSelfToAdmin

	cannotSetRepoPrivacy
)

var (
	// --- SYS-ERR-xxx: System / Service exceptions ---
	// a generic, unexpected server-side error occurred
	//
	// Description: An unexpected condition was encountered on the server that prevented it from fulfilling the request. This is a catch-all for unhandled exceptions, such as marshalling errors or type conversion failures.
	//
	// Description_ZH: 服务器上遇到了意外情况，导致无法完成请求。这是一个用于捕获未处理异常的通用错误，例如序列化错误或类型转换失败。
	//
	// en-US: Internal Server Error
	//
	// zh-CN: 服务器内部错误
	//
	// zh-HK: 伺服器內部錯誤
	ErrInternalServerError error = CustomError{prefix: errSysPrefix, code: internalServerError}
	// a call to a remote or downstream service failed
	//
	// Description: A request to a dependent downstream or external service failed. This is a generic error that should be converted to a more specific error in the calling component.
	//
	// Description_ZH: 对下游依赖或外部服务的请求失败。这是一个通用错误，应在调用组件中转换为更具体的错误。
	//
	// en-US: Remote service call failed
	//
	// zh-CN: 远程服务调用失败
	//
	// zh-HK: 遠程服務調用失敗
	ErrRemoteServiceFail = CustomError{prefix: errSysPrefix, code: remoteServiceFail}
	// a generic database operation failed
	//
	// Description: An unhandled or unexpected error occurred during a database operation, such as a lost connection (`sql.ErrConnDone`).
	//
	// Description_ZH: 在数据库操作期间发生了未处理或意外的错误，例如连接丢失（`sql.ErrConnDone`）。
	//
	// en-US: Database operation failed
	//
	// zh-CN: 数据库操作失败
	//
	// zh-HK: 資料庫操作失敗
	ErrDatabaseFailure = CustomError{prefix: errSysPrefix, code: databaseFailure}
	// a database query returned no results when one was expected
	//
	// Description: A database query that was expected to return at least one row found no matching records. This is a system-level wrapper for `sql.ErrNoRows`.
	//
	// Description_ZH: 期望至少返回一行的数据库查询没有找到匹配的记录。这是 `sql.ErrNoRows` 的系统级封装。
	//
	// en-US: Record not found in database
	//
	// zh-CN: 数据库中未找到记录
	//
	// zh-HK: 資料庫中未找到記錄
	ErrDatabaseNoRows = CustomError{prefix: errSysPrefix, code: databaseNoRows}
	// a database write operation violated a unique key constraint
	//
	// Description: An `INSERT` or `UPDATE` operation failed because it would have created a duplicate value in a column with a unique constraint.
	//
	// Description_ZH: `INSERT` 或 `UPDATE` 操作失败，因为它会在具有唯一约束的列中创建重复值。
	//
	// en-US: Duplicate entry for key
	//
	// zh-CN: 键值重复
	//
	// zh-HK: 鍵值重複
	ErrDatabaseDuplicateKey = CustomError{prefix: errSysPrefix, code: databaseDuplicateKey}
	// the LFS (Large File Storage) service is not configured or found
	//
	// Description: The system could not find or connect to the configured LFS service. This indicates a system configuration issue.
	//
	// Description_ZH: 系统无法找到或连接到配置的LFS（大文件存储）服务。这表明存在系统配置问题。
	//
	// en-US: LFS service not found
	//
	// zh-CN: 未找到LFS服务
	//
	// zh-HK: 未找到LFS服務
	ErrLFSNotFound = CustomError{prefix: errSysPrefix, code: lfsNotFound}
	// cannot remove the last administrator of an organization
	//
	// Description: The requested action to remove a user's admin role is prohibited because they are the sole administrator of an organization. This prevents the organization from being locked.
	//
	// Description_ZH: 禁止移除用户管理员角色的请求，因为他们是组织的唯一管理员。此举可防止组织被锁定而无法管理。
	//
	// en-US: Cannot remove the last administrator of the organization
	//
	// zh-CN: 不能移除组织的最后一个管理员
	//
	// zh-HK: 不能移除組織的最後一個管理員
	ErrLastOrgAdmin = CustomError{prefix: errSysPrefix, code: lastOrgAdmin}
	// cannot promote yourself to admin
	//
	// Description: The requested action to promote yourself to an administrator is prohibited.
	//
	// Description_ZH: 禁止将自身提升为管理员。
	//
	// en-US: Cannot promote yourself to admin
	//
	// zh-CN: 不能将自身提升为管理员
	//
	// zh-HK: 不能將自身提升為管理員
	ErrCannotPromoteSelfToAdmin = CustomError{prefix: errSysPrefix, code: cannotPromoteSelfToAdmin}
	// cannot change repository privacy
	//
	// Description: The requested action to change the privacy setting of a repository is prohibited. Because sensitive check not passed.
	//
	// Description_ZH: 用户禁止更改存储库的隐私设置，由于敏感词检测没有通过。
	//
	// en-US: Cannot change repository privacy
	//
	// zh-CN: 不能更改存储库的隐私
	//
	// zh-HK: 不能更改存儲庫的隱私
	ErrCannotSetRepoPrivacy = CustomError{prefix: errSysPrefix, code: cannotSetRepoPrivacy}
)

// Used in DB to convert db error to custom error
//
// Add new error in future
func HandleDBError(err error, ctx map[string]interface{}) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		context: ctx,
		err:     err,
	}
	if errors.Is(err, sql.ErrNoRows) {
		customErr.code = int(databaseNoRows)
		return customErr
	} else if strings.Contains(err.Error(), "duplicate key value") {
		customErr.code = int(databaseDuplicateKey)
		return customErr
	} else {
		customErr.code = int(databaseFailure)
		return customErr
	}
}

func InternalServerError(err error, ctx context) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		code:    internalServerError,
		err:     err,
		context: ctx,
	}
	return customErr
}

// Used to convert service error to custom error
func RemoteSvcFail(err error, ctx context) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		context: ctx,
		code:    int(remoteServiceFail),
		err:     err,
	}
	return customErr
}

func LFSNotFound(err error, ctx context) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		context: ctx,
		code:    int(lfsNotFound),
		err:     err,
	}
	return customErr
}

func LastOrgAdmin(err error, ctx context) error {
	if err == nil {
		return nil
	}
	return CustomError{
		prefix:  errSysPrefix,
		err:     err,
		code:    lastOrgAdmin,
		context: ctx,
	}
}

func CannotPromoteSelfToAdmin(err error, ctx context) error {
	if err == nil {
		return nil
	}
	return CustomError{
		prefix:  errSysPrefix,
		err:     err,
		code:    cannotPromoteSelfToAdmin,
		context: ctx,
	}
}

func CannotSetRepoPrivacy(err error, ctx context) error {
	if err == nil {
		return nil
	}
	return CustomError{
		prefix:  errSysPrefix,
		err:     err,
		code:    cannotSetRepoPrivacy,
		context: ctx,
	}
}
