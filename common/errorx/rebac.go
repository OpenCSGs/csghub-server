package errorx

const errReBACPrefix = "REBAC-ERR"

const (
	codeReBACNamespacePermissionCreateFailed = iota
	codeReBACNamespacePermissionDeleteFailed
)

var (
	// ErrReBACNamespacePermissionCreateFailed indicates that a namespace permission relationship could not be created.
	//
	// Description: Failed to create the namespace permission relationship.
	//
	// Description_ZH: 创建命名空间权限关系失败。
	//
	// en-US: Failed to create the namespace permission relationship.
	//
	// zh-CN: 创建命名空间权限关系失败。
	//
	// zh-HK: 建立命名空間權限關係失敗。
	ErrReBACNamespacePermissionCreateFailed error = CustomError{
		prefix: errReBACPrefix,
		code:   codeReBACNamespacePermissionCreateFailed,
	}

	// ErrReBACNamespacePermissionDeleteFailed indicates that a namespace permission relationship could not be deleted.
	//
	// Description: Failed to delete the namespace permission relationship.
	//
	// Description_ZH: 删除命名空间权限关系失败。
	//
	// en-US: Failed to delete the namespace permission relationship.
	//
	// zh-CN: 删除命名空间权限关系失败。
	//
	// zh-HK: 刪除命名空間權限關係失敗。
	ErrReBACNamespacePermissionDeleteFailed error = CustomError{
		prefix: errReBACPrefix,
		code:   codeReBACNamespacePermissionDeleteFailed,
	}
)

// ReBACNamespacePermissionCreateFailed wraps a namespace permission relationship creation failure with optional context.
func ReBACNamespacePermissionCreateFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errReBACPrefix,
		code:    codeReBACNamespacePermissionCreateFailed,
		err:     err,
		context: ctx,
	}
}

// ReBACNamespacePermissionDeleteFailed wraps a namespace permission relationship deletion failure with optional context.
func ReBACNamespacePermissionDeleteFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errReBACPrefix,
		code:    codeReBACNamespacePermissionDeleteFailed,
		err:     err,
		context: ctx,
	}
}
