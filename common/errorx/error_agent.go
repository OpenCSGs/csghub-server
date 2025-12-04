package errorx

const errAgentPrefix = "AGENT-ERR"

const (
	instanceQuotaExceeded = iota
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
