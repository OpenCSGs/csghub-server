package errorx

const errDeployPrefix = "DEPLOY-ERR"

const (
	codeDeployNameAlreadyExistsErr = iota
	codeDeployStopFirstErr
)

var (
	// Description: A deploy with the same name already exists for this deploy type.
	//
	// Description_ZH: 同类型下已存在同名部署
	//
	// en-US: Deploy name already exists for this deploy type.
	//
	// zh-CN: 同类型下已存在同名部署
	//
	// zh-HK: 同類型下已存在同名部署
	ErrDeployNameAlreadyExists error = CustomError{prefix: errDeployPrefix, code: codeDeployNameAlreadyExistsErr}

	// Description: The deploy is still running, please stop it first before updating.
	//
	// Description_ZH: 部署实例仍在运行中，请先停止后再更新
	//
	// en-US: The deploy is still running, please stop it first before updating.
	//
	// zh-CN: 部署实例仍在运行中，请先停止后再更新
	//
	// zh-HK: 部署實例仍在運行中，請先停止後再更新
	ErrDeployStopFirst error = CustomError{prefix: errDeployPrefix, code: codeDeployStopFirstErr}
)
