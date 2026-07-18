package errorx

const errRepoPrefix = "REPO-ERR"

const (
	codeRepoAlreadyExistErr = iota
	codeRepoNameInvalidErr
	codeNamespaceNotFoundErr
	codeRepoNotFoundErr
	codeRepoNoDefaultBranchErr
	codeCodeZipDownloadFailedErr
	codeBatchGetRepoExtraFailedErr
	codeChangePathBlockedErr
	codeSensitiveCheckNotPassedErr
)

var (
	// Description: A repository with the same name already exists.
	//
	// Description_ZH: 仓库名称已存在
	//
	// en-US: A repository with the same name already exists.
	//
	// zh-CN: 仓库名称已存在
	//
	// zh-HK: 倉庫名稱已存在
	ErrRepoAlreadyExist error = CustomError{prefix: errRepoPrefix, code: codeRepoAlreadyExistErr}

	// Description: The repository name is invalid.
	//
	// Description_ZH: 仓库名称无效
	//
	// en-US: The repository name is invalid.
	//
	// zh-CN: 仓库名称无效
	//
	// zh-HK: 倉庫名稱無效
	ErrRepoNameInvalid error = CustomError{prefix: errRepoPrefix, code: codeRepoNameInvalidErr}

	// Description: The namespace does not exist.
	//
	// Description_ZH: 命名空间不存在
	//
	// en-US: The namespace does not exist.
	//
	// zh-CN: 命名空间不存在
	//
	// zh-HK: 命名空間不存在
	ErrNamespaceNotFound error = CustomError{prefix: errRepoPrefix, code: codeNamespaceNotFoundErr}

	// Description: The repository was not found.
	//
	// Description_ZH: 仓库未找到
	//
	// en-US: Repository not found
	//
	// zh-CN: 仓库未找到
	//
	// zh-HK: 儲存庫未找到
	ErrRepoNotFound error = CustomError{prefix: errRepoPrefix, code: codeRepoNotFoundErr}

	// Description: No revision specified and repository has no default branch. Please specify a revision.
	//
	// Description_ZH: 用户未指定分支，请指定分支后再试
	//
	// en-US: No revision specified. Please specify a revision.
	//
	// zh-CN: 用户未指定分支，请指定分支后再试
	//
	// zh-HK: 用戶未指定分支，請指定分支後再試
	ErrRepoNoDefaultBranch error = CustomError{prefix: errRepoPrefix, code: codeRepoNoDefaultBranchErr}

	// Description: Failed to download code repository as zip archive.
	//
	// Description_ZH: 下载代码仓库 zip 归档失败
	//
	// en-US: Failed to download code zip archive
	//
	// zh-CN: 下载代码 zip 归档失败
	//
	// zh-HK: 下載代碼 zip 歸檔失敗
	ErrCodeZipDownloadFailed error = CustomError{prefix: errRepoPrefix, code: codeCodeZipDownloadFailedErr}

	// Description: Failed to batch get repository extra information.
	//
	// Description_ZH: 批量获取仓库额外信息失败
	//
	// en-US: Failed to batch get repository extra information
	//
	// zh-CN: 批量获取仓库额外信息失败
	//
	// zh-HK: 批量獲取倉庫額外資訊失敗
	ErrBatchGetRepoExtraFailed error = CustomError{prefix: errRepoPrefix, code: codeBatchGetRepoExtraFailedErr}

	// Description: Cannot change repository path because dependent entities exist. Please remove them first.
	//
	// Description_ZH: 无法更改仓库路径，存在依赖实体。请先移除它们。
	//
	// en-US: Cannot change repository path because the following dependent entities exist: {{.entity}}. Please remove them first.
	//
	// zh-CN: 无法更改仓库路径，以下依赖实体存在：{{.entity}}。请先移除它们。
	//
	// zh-HK: 無法更改倉庫路徑，以下依賴實體存在：{{.entity}}。請先移除它們。
	ErrChangePathBlocked error = CustomError{prefix: errRepoPrefix, code: codeChangePathBlockedErr}

	// Description: The repository cannot be made public because the compliance scan has not passed.
	//
	// Description_ZH: 合规扫描未通过，无法将仓库设置为公开。
	//
	// en-US: Compliance scan not passed, cannot make repository public.
	//
	// zh-CN: 合规扫描未通过，无法将仓库设置为公开。
	//
	// zh-HK: 合規掃描未通過，無法將倉庫設置為公開。
	ErrSensitiveCheckNotPassed error = CustomError{prefix: errRepoPrefix, code: codeSensitiveCheckNotPassedErr}
)

// RepoNotFound creates a REPO-ERR-3 error with context.
func RepoNotFound(err error, ctx context) error {
	return CustomError{prefix: errRepoPrefix, code: codeRepoNotFoundErr, err: err, context: ctx}
}

// RepoNoDefaultBranch creates a REPO-ERR-4 error.
func RepoNoDefaultBranch(ctx context) error {
	return CustomError{prefix: errRepoPrefix, code: codeRepoNoDefaultBranchErr, context: ctx}
}

// CodeZipDownloadFailed creates a REPO-ERR-5 error with context.
func CodeZipDownloadFailed(err error, ctx context) error {
	return CustomError{prefix: errRepoPrefix, code: codeCodeZipDownloadFailedErr, err: err, context: ctx}
}

// BatchGetRepoExtraFailed creates a REPO-ERR-6 error.
func BatchGetRepoExtraFailed(err error) error {
	return CustomError{prefix: errRepoPrefix, code: codeBatchGetRepoExtraFailedErr, err: err}
}

// ChangePathBlocked creates a REPO-ERR-7 error with context.
func ChangePathBlocked(err error, ctx context) error {
	return CustomError{prefix: errRepoPrefix, code: codeChangePathBlockedErr, err: err, context: ctx}
}

// SensitiveCheckNotPassed creates a REPO-ERR-7 error with context.
func SensitiveCheckNotPassed(err error, ctx context) error {
	return CustomError{prefix: errRepoPrefix, code: codeSensitiveCheckNotPassedErr, err: err, context: ctx}
}
