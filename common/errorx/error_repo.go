package errorx

const errRepoPrefix = "REPO-ERR"

const (
	codeRepoAlreadyExistErr = iota
	codeRepoNameInvalidErr
	codeNamespaceNotFoundErr
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
)
