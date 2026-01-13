package errorx

const errAgentPrefix = "AGENT-ERR"

const (
	instanceQuotaExceeded = iota
	instanceNameAlreadyExists
	knowledgeBaseNameAlreadyExists
	mcpServerNameAlreadyExists
)

var (
	// instance quota exceeded
	//
	// Description: The instance quota exceeded. Includes agent type, instance count, and quota in the error message.
	//
	// Description_ZH: 实例配额超出。错误消息中包含智能体类型、实例数量和配额。
	//
	// en-US: Instance quota exceeded, agent type: {{.agent_type}}, instance count: {{.instance_count}}, quota: {{.quota}}
	//
	// zh-CN: 实例配额超出，智能体类型: {{.agent_type}}, 实例数量: {{.instance_count}}，配额: {{.quota}}
	//
	// zh-HK: 實例配額超出，智能體類型: {{.agent_type}}, 實例數量: {{.instance_count}}，配額: {{.quota}}
	ErrInstanceQuotaExceeded error = CustomError{prefix: errAgentPrefix, code: instanceQuotaExceeded}

	// you have a instance with the same name
	//
	// Description: You have an instance with the same name.
	//
	// Description_ZH: 您已存在相同名称的实例。
	//
	// en-US: You have a instance with the same name: {{.instance_name}}
	//
	// zh-CN: 您已存在相同名称的实例: {{.instance_name}}
	//
	// zh-HK: 您已存在相同名稱的實例: {{.instance_name}}
	ErrInstanceNameAlreadyExists error = CustomError{prefix: errAgentPrefix, code: instanceNameAlreadyExists}

	// you have a knowledge base with the same name
	//
	// Description: You have a knowledge base with the same name.
	//
	// Description_ZH: 您已存在相同名称的知识库。
	//
	// en-US: You have a knowledge base with the same name: {{.knowledge_base_name}}
	//
	// zh-CN: 您已存在相同名称的知识库: {{.knowledge_base_name}}
	//
	// zh-HK: 您已存在相同名稱的知識庫: {{.knowledge_base_name}}
	ErrKnowledgeBaseNameAlreadyExists error = CustomError{prefix: errAgentPrefix, code: knowledgeBaseNameAlreadyExists}

	// you have a mcp server with the same name
	//
	// Description: You have an MCP server with the same name.
	//
	// Description_ZH: 您已存在相同名称的MCP服务器。
	//
	// en-US: You have an MCP server with the same name: {{.server_name}}
	//
	// zh-CN: 您已存在相同名称的MCP服务器: {{.server_name}}
	//
	// zh-HK: 您已存在相同名稱的MCP服務器: {{.server_name}}
	ErrMCPServerNameAlreadyExists error = CustomError{prefix: errAgentPrefix, code: mcpServerNameAlreadyExists}
)

func InstanceQuotaExceeded(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(instanceQuotaExceeded),
	}
	return customErr
}

func InstanceNameAlreadyExists(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(instanceNameAlreadyExists),
	}
	return customErr
}

func KnowledgeBaseNameAlreadyExists(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(knowledgeBaseNameAlreadyExists),
	}
	return customErr
}

func MCPServerNameAlreadyExists(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(mcpServerNameAlreadyExists),
	}
	return customErr
}
