package errorx

const errServerlessPrefix = "SERVERLESS-ERR"

const (
	codeStrategyTypeErr = iota
	codeDeployNotFoundErr
	codeDeployStatusNotMatchErr
	codeDeployMaxReplicaErr
	codeRevisionNotFoundErr
	codeInvalidPercentErr
	codeCommitIDEmptyErr
	codeTrafficInvalidErr
	codeInvalidCommitIDErr
)

var (
	// Description: The request parameter does not match the server requirements, and the server cannot process the request.
	//
	// Description_ZH: 请求参数不匹配, 服务器无法处理该请求。
	//
	// en-US: The strategy type is not supported.
	//
	// zh-CN: 部署策略类型不支持
	//
	// zh-HK: 部署策略類型不支持
	ErrStrategyTypeErr error = CustomError{prefix: errServerlessPrefix, code: codeStrategyTypeErr}

	// Description: The deploy not found.
	//
	// Description_ZH: 部署实例不存在
	//
	// en-US: The deploy not found.
	//
	// zh-CN: 部署实例不存在
	//
	// zh-HK: 部署實例不存在
	ErrDeployNotFoundErr error = CustomError{prefix: errServerlessPrefix, code: codeDeployNotFoundErr}

	// Description: The deploy status not match.
	//
	// Description_ZH: 部署实例状态不匹配
	//
	// en-US: The deploy status not match.
	//
	// zh-CN: 部署实例状态不匹配
	//
	// zh-HK: 部署實例狀態不匹配
	ErrDeployStatusNotMatchErr error = CustomError{prefix: errServerlessPrefix, code: codeDeployStatusNotMatchErr}

	// Description: The deploy max replica not match.
	//
	// Description_ZH: 策略部署仅支持最大副本数为1的部署实例
	//
	// en-US: The deploy max replica not match.
	//
	// zh-CN: 策略部署仅支持最大副本数为1的部署实例
	//
	// zh-HK: 策略部署僅支持最大副本數為1的部署實例
	ErrDeployMaxReplicaErr error = CustomError{prefix: errServerlessPrefix, code: codeDeployMaxReplicaErr}

	// Description: The revision not found.
	//
	// Description_ZH: 修订版本不存在
	//
	// en-US: The revision not found.
	//
	// zh-CN: 修订版本不存在
	//
	// zh-HK: 修訂版本不存在
	ErrRevisionNotFound error = CustomError{prefix: errServerlessPrefix, code: codeRevisionNotFoundErr}

	// Description: The percent not match.
	//
	// Description_ZH: 百分比总和不为100
	//
	// en-US: The percent not match.
	//
	// zh-CN: 百分比总和不为100
	//
	// zh-HK: 百分比總和不為100
	ErrInvalidPercent error = CustomError{prefix: errServerlessPrefix, code: codeInvalidPercentErr}

	// Description: The commit id is empty.
	//
	// Description_ZH: commit id 为空
	//
	// en-US: The commit id is empty.
	//
	// zh-CN: commit id 为空
	//
	// zh-HK: commit id 為空
	ErrCommitIDEmpty error = CustomError{prefix: errServerlessPrefix, code: codeCommitIDEmptyErr}

	// Description: The commit id is invalid.
	//
	// Description_ZH: 无效的commitId
	//
	// en-US: The commit id is invalid.
	//
	// zh-CN: 无效的commitId
	//
	// zh-HK: 無效的commitId
	ErrInvalidCommitID error = CustomError{prefix: errServerlessPrefix, code: codeInvalidCommitIDErr}

	// Description: The traffic percent is invalid.
	//
	// Description_ZH: 流量百分比无效
	//
	// en-US: The traffic percent is invalid.
	//
	// zh-CN: 流量百分比无效
	//
	// zh-HK: 流量百分比無效
	ErrTrafficInvalid error = CustomError{prefix: errServerlessPrefix, code: codeTrafficInvalidErr}

	//Description: no other valid revision except
	//
	//Description_ZH: 没有其他有效修订版本
	//
	//en-US: no other valid revision except
	//
	//zh-CN: 没有其他有效修订版本
	//
	//zh-HK: 沒有其他有效修訂版本
	ErrNoOtherValidRevision error = CustomError{prefix: errServerlessPrefix, code: codeTrafficInvalidErr}
)
