package errorx

const errDeployPrefix = "DEPLOY-ERR"

const (
	codeDeployNameAlreadyExistsErr = iota
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
)
