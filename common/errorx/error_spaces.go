package errorx

const errSpacePrefix = "SPACE-ERR"

const (
	codeGetSpaceDockerTemplatePathFailedErr = iota
	codeSpaceNameAlreadyExistErr
	codeSpaceInitFailedErr
)

var (
	// Description: Failed to get the space Docker template path.
	//
	// Description_ZH: 获取空间 Docker 模板路径失败
	//
	// en-US: Failed to get the space Docker template path.
	//
	// zh-CN: 获取空间 Docker 模板路径失败
	//
	// zh-HK: 獲取空間 Docker 模板路徑失敗
	ErrGetSpaceDockerTemplatePathFailed error = CustomError{prefix: errSpacePrefix, code: codeGetSpaceDockerTemplatePathFailedErr}

	// Description: The space name already exists.
	//
	// Description_ZH: 空间名称已经存在
	//
	// en-US: The space name already exists.
	//
	// zh-CN: 空间名称已经存在
	//
	// zh-HK: 空間名稱已存在
	ErrSpaceNameAlreadyExist error = CustomError{prefix: errSpacePrefix, code: codeSpaceNameAlreadyExistErr}

	// Description: Failed to initialize the space.
	//
	// Description_ZH: 初始化空间失败
	//
	// en-US: Failed to initialize the space.
	//
	// zh-CN: 初始化空间失败
	//
	// zh-HK: 初始化空間失敗
	ErrSpaceInitFailed error = CustomError{prefix: errSpacePrefix, code: codeSpaceInitFailedErr}
)
