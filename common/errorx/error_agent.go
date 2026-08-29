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
	credentialVerifyURLInvalid
	credentialVerifyTokenInvalid
	credentialVerifyFailed
	instanceProvisioningMetadataImmutable
	csgclawTemplateCreationForbidden
	agentProvisionRequestFieldNull
	agentProvisionRequestFieldType
	agentProvisionRequestFieldEmpty
	agentProvisionRequestModelUnavailable
	csgclawTemplateNotFound
	agentTemplateSensitiveCheckCreateAgentBlocked
	agentTemplateSensitiveCheckMakePublicBlocked
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

	// credential verification URL is invalid
	//
	// Description: The credential verification URL or API endpoint is invalid.
	//
	// Description_ZH: 凭证验证 URL 或 API 端点无效。
	//
	// en-US: Credential verification URL is invalid
	//
	// zh-CN: 凭证验证 URL 无效
	//
	// zh-HK: 憑證驗證 URL 無效
	ErrCredentialVerifyURLInvalid error = CustomError{prefix: errAgentPrefix, code: credentialVerifyURLInvalid}

	// credential token is invalid
	//
	// Description: The credential token is invalid, expired, or missing required permissions.
	//
	// Description_ZH: 凭证令牌无效、已过期或缺少所需权限。
	//
	// en-US: Credential token is invalid
	//
	// zh-CN: 凭证令牌无效
	//
	// zh-HK: 憑證令牌無效
	ErrCredentialVerifyTokenInvalid error = CustomError{prefix: errAgentPrefix, code: credentialVerifyTokenInvalid}

	// credential verification failed
	//
	// Description: Credential verification failed.
	//
	// Description_ZH: 凭证验证失败。
	//
	// en-US: Credential verification failed
	//
	// zh-CN: 凭证验证失败
	//
	// zh-HK: 憑證驗證失敗
	ErrCredentialVerifyFailed error = CustomError{prefix: errAgentPrefix, code: credentialVerifyFailed}

	// instance provisioning metadata is immutable after creation
	//
	// Description: Agent instance provisioning metadata cannot be updated after creation. Sandbox provisioning is fixed at instance creation and there is no live sandbox update path. The error message includes the agent instance type.
	//
	// Description_ZH: 智能体实例的部署元数据在创建后无法更新。沙箱部署在实例创建时确定，且没有实时沙箱更新路径。错误消息中包含智能体实例类型。
	//
	// en-US: {{.instance_type}} instance provisioning metadata cannot be updated after creation
	//
	// zh-CN: {{.instance_type}} 实例的部署元数据在创建后无法更新
	//
	// zh-HK: {{.instance_type}} 實例的部署元數據在創建後無法更新
	ErrInstanceProvisioningMetadataImmutable error = CustomError{prefix: errAgentPrefix, code: instanceProvisioningMetadataImmutable}

	// csgclaw agent template creation is forbidden via the API
	//
	// Description: csgclaw agent templates are managed by the code repository and cannot be created via the API. The error message includes the template type.
	//
	// Description_ZH: csgclaw 智能体模板由代码仓库托管，不能通过 API 创建。错误消息中包含模板类型。
	//
	// en-US: {{.template_type}} agent templates are managed by the code repository and cannot be created via the API
	//
	// zh-CN: {{.template_type}} 智能体模板由代码仓库托管，不能通过 API 创建
	//
	// zh-HK: {{.template_type}} 智能體模板由程式碼倉庫托管，不能通過 API 建立
	ErrCSGClawTemplateCreationForbidden error = CustomError{prefix: errAgentPrefix, code: csgclawTemplateCreationForbidden}

	// an agent provision request field is null but must not be
	//
	// Description: A field in the agent instance provision request metadata is null. The error message includes the agent instance type and the offending field.
	//
	// Description_ZH: 智能体实例部署请求元数据中的字段为 null。错误消息中包含智能体实例类型和出错的字段。
	//
	// en-US: {{.instance_type}} provision request field {{.field}} must not be null
	//
	// zh-CN: {{.instance_type}} 部署请求字段 {{.field}} 不能为 null
	//
	// zh-HK: {{.instance_type}} 部署請求欄位 {{.field}} 不能為 null
	ErrAgentProvisionRequestFieldNull error = CustomError{prefix: errAgentPrefix, code: agentProvisionRequestFieldNull}

	// an agent provision request field has an invalid type
	//
	// Description: A field in the agent instance provision request metadata has an invalid type. The error message includes the agent instance type and the offending field.
	//
	// Description_ZH: 智能体实例部署请求元数据中的字段类型无效。错误消息中包含智能体实例类型和出错的字段。
	//
	// en-US: {{.instance_type}} provision request field {{.field}} has an invalid type
	//
	// zh-CN: {{.instance_type}} 部署请求字段 {{.field}} 类型无效
	//
	// zh-HK: {{.instance_type}} 部署請求欄位 {{.field}} 類型無效
	ErrAgentProvisionRequestFieldType error = CustomError{prefix: errAgentPrefix, code: agentProvisionRequestFieldType}

	// an agent provision request field is empty but must not be
	//
	// Description: A field in the agent instance provision request metadata is empty. The error message includes the agent instance type and the offending field.
	//
	// Description_ZH: 智能体实例部署请求元数据中的字段为空。错误消息中包含智能体实例类型和出错的字段。
	//
	// en-US: {{.instance_type}} provision request field {{.field}} must not be empty
	//
	// zh-CN: {{.instance_type}} 部署请求字段 {{.field}} 不能为空
	//
	// zh-HK: {{.instance_type}} 部署請求欄位 {{.field}} 不能為空
	ErrAgentProvisionRequestFieldEmpty error = CustomError{prefix: errAgentPrefix, code: agentProvisionRequestFieldEmpty}

	// a pinned llm model is not available for the agent provision request
	//
	// Description: The model pinned in the agent instance provision request is not in the available llm model catalog. The error message includes the model name and the agent instance type.
	//
	// Description_ZH: 智能体实例部署请求中指定的模型不在可用 llm 模型目录中。错误消息中包含模型名称和智能体实例类型。
	//
	// en-US: llm model {{.model}} is not available for {{.instance_type}}
	//
	// zh-CN: llm 模型 {{.model}} 不适用于 {{.instance_type}}
	//
	// zh-HK: llm 模型 {{.model}} 不適用於 {{.instance_type}}
	ErrAgentProvisionRequestModelUnavailable error = CustomError{prefix: errAgentPrefix, code: agentProvisionRequestModelUnavailable}

	// csgclaw agent template not found for the repository path
	//
	// Description: No csgclaw agent template matches the repository path in the instance provision request.
	//
	// Description_ZH: 未找到与实例部署请求中的仓库路径匹配的 csgclaw 智能体模板。
	//
	// en-US: csgclaw template not found for repository path {{.repo_path}}
	//
	// zh-CN: 仓库路径 {{.repo_path}} 未找到对应的 csgclaw 模板
	//
	// zh-HK: 倉庫路徑 {{.repo_path}} 未找到對應的 csgclaw 模板
	ErrCSGClawTemplateNotFound error = CustomError{prefix: errAgentPrefix, code: csgclawTemplateNotFound}

	// agent template sensitive check blocks creating an agent
	ErrAgentTemplateSensitiveCheckCreateAgentBlocked error = CustomError{prefix: errAgentPrefix, code: agentTemplateSensitiveCheckCreateAgentBlocked}

	// agent template sensitive check blocks making a template public
	ErrAgentTemplateSensitiveCheckMakePublicBlocked error = CustomError{prefix: errAgentPrefix, code: agentTemplateSensitiveCheckMakePublicBlocked}
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

func CredentialVerifyURLInvalid(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(credentialVerifyURLInvalid),
	}
}

func CredentialVerifyTokenInvalid(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(credentialVerifyTokenInvalid),
	}
}

func CredentialVerifyFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(credentialVerifyFailed),
	}
}

func InstanceProvisioningMetadataImmutable(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(instanceProvisioningMetadataImmutable),
	}
}

func CSGClawTemplateCreationForbidden(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(csgclawTemplateCreationForbidden),
	}
}

func AgentProvisionRequestFieldNull(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(agentProvisionRequestFieldNull),
	}
}

func AgentProvisionRequestFieldType(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(agentProvisionRequestFieldType),
	}
}

func AgentProvisionRequestFieldEmpty(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(agentProvisionRequestFieldEmpty),
	}
}

func AgentProvisionRequestModelUnavailable(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(agentProvisionRequestModelUnavailable),
	}
}

func CSGClawTemplateNotFound(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(csgclawTemplateNotFound),
	}
}

func AgentTemplateSensitiveCheckCreateAgentBlocked(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(agentTemplateSensitiveCheckCreateAgentBlocked),
	}
}

func AgentTemplateSensitiveCheckMakePublicBlocked(err error, ctx context) error {
	return CustomError{
		prefix:  errAgentPrefix,
		context: ctx,
		err:     err,
		code:    int(agentTemplateSensitiveCheckMakePublicBlocked),
	}
}
