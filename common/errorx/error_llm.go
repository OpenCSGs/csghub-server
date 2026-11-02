package errorx

const errLLMPrefix = "LLM-ERR"

const (
	llmConfigNotFound = iota
	llmConfigModelGrantSubjectNotFound
	llmConfigModelGrantAlreadyExists
)

var (
	// llm config not found
	//
	// Description: The LLM configuration was not found.
	//
	// Description_ZH: 未找到该 LLM 配置。
	//
	// en-US: LLM configuration not found
	//
	// zh-CN: 未找到该 LLM 配置
	//
	// zh-HK: 未找到該 LLM 配置
	ErrLLMConfigNotFound error = CustomError{prefix: errLLMPrefix, code: llmConfigNotFound}
	// model grant subject not found
	//
	// Description: The user or organization to grant was not found.
	//
	// Description_ZH: 未找到要授权的用户或组织。
	//
	// en-US: Grant user or organization not found
	//
	// zh-CN: 未找到要授权的用户或组织
	//
	// zh-HK: 未找到要授權的用戶或組織
	ErrLLMConfigModelGrantSubjectNotFound error = CustomError{prefix: errLLMPrefix, code: llmConfigModelGrantSubjectNotFound}
	// model grant already exists
	//
	// Description: The user or organization is already granted access to this model.
	//
	// Description_ZH: 该用户或组织已被授权访问此模型。
	//
	// en-US: The user or organization is already granted access to this model
	//
	// zh-CN: 该用户或组织已被授权访问此模型
	//
	// zh-HK: 該用戶或組織已被授權訪問此模型
	ErrLLMConfigModelGrantAlreadyExists error = CustomError{prefix: errLLMPrefix, code: llmConfigModelGrantAlreadyExists}
)
