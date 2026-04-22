package errorx

const errAgentPrefix = "AGENT-ERR"

const (
	instanceQuotaExceeded = iota
	instanceNameAlreadyExists
	knowledgeBaseNameAlreadyExists
	mcpServerNameAlreadyExists
	pinLimitExceeded
	invalidShareSessionUUID
	shareSessionUUIDExpired
	schedulerQuotaExceeded
	schedulerInstanceNoCapability
	schedulerStartTimeInPast
	credentialNameAlreadyExists
	runtimeCredentialTokenInvalid
	runtimeCredentialGrantUnavailable
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

	// pin limit exceeded
	//
	// Description: The pin limit exceeded. Maximum 5 items can be pinned per entity type.
	//
	// Description_ZH: 置顶数量超出限制。每种实体类型最多可置顶 5 个项目。
	//
	// en-US: Pin limit exceeded. Maximum 5 items can be pinned per entity type.
	//
	// zh-CN: 置顶数量超出限制。每种实体类型最多可置顶 5 个项目。
	//
	// zh-HK: 置頂數量超出限制。每種實體類型最多可置頂 5 個項目。
	ErrPinLimitExceeded error = CustomError{prefix: errAgentPrefix, code: pinLimitExceeded}

	// invalid share session uuid
	//
	// Description: The share session uuid is invalid.
	//
	// Description_ZH: 分享会话UUID无效。
	//
	// en-US: Invalid share session uuid
	//
	// zh-CN: 分享会话UUID无效
	//
	// zh-HK: 分享會話UUID無效
	ErrInvalidShareSessionUUID error = CustomError{prefix: errAgentPrefix, code: invalidShareSessionUUID}

	// share session uuid expired
	//
	// Description: The share session uuid expired.
	//
	// Description_ZH: 分享会话UUID已过期。
	//
	// en-US: Share session UUID expired
	//
	// zh-CN: 分享会话UUID已过期
	//
	// zh-HK: 分享會話UUID已過期
	ErrShareSessionUUIDExpired error = CustomError{prefix: errAgentPrefix, code: shareSessionUUIDExpired}

	// scheduler quota exceeded
	//
	// Description: The scheduled task creation quota exceeded. User has reached the limit of schedulers they can create.
	//
	// Description_ZH: 定时任务创建数量已达上限。
	//
	// en-US: You have created {{.scheduler_count}} scheduled tasks, which has reached the limit. Please delete unused scheduled tasks to free up slots before creating new ones.
	//
	// zh-CN: 你当前已创建 {{.scheduler_count}} 个定时任务，已达到创建上限，暂时无法创建新的定时任务。 请先删除不再使用的定时任务，释放名额后即可继续创建。
	//
	// zh-HK: 你當前已創建 {{.scheduler_count}} 個定時任務，已達到創建上限，暫時無法創建新的定時任務。 請先刪除不再使用的定時任務，釋放名額後即可繼續創建。
	ErrSchedulerQuotaExceeded error = CustomError{prefix: errAgentPrefix, code: schedulerQuotaExceeded}

	// agent instance does not have the scheduler capability
	//
	// Description: The agent instance does not support scheduling. The "scheduler" capability must be added to the instance metadata.
	//
	// Description_ZH: 该智能体实例不支持定时任务功能，需在实例元数据中添加 "scheduler" 能力。
	//
	// en-US: Agent instance does not support scheduling
	//
	// zh-CN: 该智能体实例不支持定时任务
	//
	// zh-HK: 該智能體實例不支持定時任務
	ErrSchedulerInstanceNoCapability error = CustomError{prefix: errAgentPrefix, code: schedulerInstanceNoCapability}

	// scheduler start time is in the past
	//
	// Description: The specified start time is in the past. One-time schedules must use a future date/time.
	//
	// Description_ZH: 指定的开始时间已过去，一次性定时任务必须使用未来的日期/时间。
	//
	// en-US: Scheduler start time is in the past; use a future date/time for one-time schedules
	//
	// zh-CN: 定时任务开始时间已过去，一次性任务请使用未来的日期/时间
	//
	// zh-HK: 定時任務開始時間已過去，一次性任務請使用未來的日期/時間
	ErrSchedulerStartTimeInPast error = CustomError{prefix: errAgentPrefix, code: schedulerStartTimeInPast}

	// you have a credential with the same name
	//
	// Description: You have a credential with the same name.
	//
	// Description_ZH: 您已存在相同名称的凭证。
	//
	// en-US: You have a credential with the same name: {{.credential_name}}
	//
	// zh-CN: 您已存在相同名称的凭证: {{.credential_name}}
	//
	// zh-HK: 您已存在相同名稱的憑證: {{.credential_name}}
	ErrCredentialNameAlreadyExists error = CustomError{prefix: errAgentPrefix, code: credentialNameAlreadyExists}

	// runtime credential token is invalid
	//
	// Description: The runtime credential token is missing, invalid, or expired.
	//
	// Description_ZH: 运行时凭证令牌缺失、无效或已过期。
	//
	// en-US: Runtime credential token is invalid or expired
	//
	// zh-CN: 运行时凭证令牌无效或已过期
	//
	// zh-HK: 運行時憑證令牌無效或已過期
	ErrRuntimeCredentialTokenInvalid error = CustomError{prefix: errAgentPrefix, code: runtimeCredentialTokenInvalid}

	// runtime credential grant is unavailable
	//
	// Description: The runtime credential token is valid, but the requested credential is not granted, revoked, expired, or unavailable.
	//
	// Description_ZH: 运行时凭证令牌有效，但请求的凭证未授权、已撤销、已过期或不可用。
	//
	// en-US: Runtime credential grant is unavailable
	//
	// zh-CN: 运行时凭证授权不可用
	//
	// zh-HK: 運行時憑證授權不可用
	ErrRuntimeCredentialGrantUnavailable error = CustomError{prefix: errAgentPrefix, code: runtimeCredentialGrantUnavailable}
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

func SchedulerQuotaExceeded(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(schedulerQuotaExceeded),
	}
	return customErr
}

func SchedulerInstanceNoCapability(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(schedulerInstanceNoCapability),
	}
}

func SchedulerStartTimeInPast(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(schedulerStartTimeInPast),
	}
}

func CredentialNameAlreadyExists(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(credentialNameAlreadyExists),
	}
}

func RuntimeCredentialTokenInvalid(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(runtimeCredentialTokenInvalid),
	}
}

func RuntimeCredentialGrantUnavailable(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(runtimeCredentialGrantUnavailable),
	}
}
