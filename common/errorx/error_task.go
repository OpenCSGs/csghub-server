package errorx

const errTaskPrefix = "TASK-ERR"

const (
	noEntryFile = iota
	multiHostInferenceNotSupported
	multiHostInferenceReplicaCount
	multiHostNotebookNotSupported
	notEnoughResource
	clusterUnavailable
)

var (
	// no entry file found for the task
	//
	// Description: The task requires a specific entry file to start execution (e.g., 'main.py' or 'app.js'), but no such file could be found in the specified source directory.
	//
	// Description_ZH: 该任务需要一个特定的入口文件来开始执行（例如 'main.py' 或 'app.js'），但在指定的源目录中找不到这样的文件。
	//
	// en-US: Task entry file not found
	//
	// zh-CN: 未找到任务入口文件
	//
	// zh-HK: 未找到任務入口檔案
	ErrNoEntryFile error = CustomError{prefix: errTaskPrefix, code: noEntryFile}
	// only vllm and sglang support multi-host inference
	//
	// Description: The multi-host inference feature is currently only available for VLLM and SGLang frameworks. Other frameworks do not support this functionality.
	//
	// Description_ZH: 多主机推理功能目前仅支持 VLLM 和 SGLang 框架，其他框架暂不支持此功能。
	//
	// en-US: Multi-host inference is only supported by VLLM and SGLang
	//
	// zh-CN: 只支持 vllm 和 sglang 的多主机推理
	//
	// zh-HK: 只支持 vllm 和 sglang 的多主機推理
	ErrMultiHostInferenceNotSupported = CustomError{prefix: errTaskPrefix, code: multiHostInferenceNotSupported}
	// multi-host notebooks are not supported
	//
	// Description: The multi-host notebook feature (running notebook tasks across multiple hosts) is not supported.
	// Use single-host notebook execution instead. This limitation applies to distributed notebook sessions which require
	// synchronized kernel/state across hosts and is currently not implemented.
	//
	// Description_ZH: 多主机 Notebook 功能（在多个主机上运行 Notebook 任务）不被支持。请改用单主机 Notebook 执行。
	// 该限制适用于需要在主机间同步内核/状态的分布式 Notebook 会话，目前尚未实现。
	//
	// en-US: Multi-host notebook is not supported
	//
	// zh-CN: 不支持多主机 Notebook
	//
	// zh-HK: 不支援多主機 Notebook
	ErrMultiHostNotebookNotSupported = CustomError{prefix: errTaskPrefix, code: multiHostNotebookNotSupported}
	// multi-host inference only supports a minimum replica count greater than 0
	//
	// Description: For multi-host inference configuration, the minimum number of replicas must be greater than zero to ensure proper service operation.
	//
	// Description_ZH: 在配置多主机推理时，最小副本数必须大于零以确保服务正常运行。
	//
	// en-US: Multi-host inference requires minimum replica count to be greater than zero
	//
	// zh-CN: 多主机推理仅支持大于 0 的最低副本数
	//
	// zh-HK: 多主機推理僅支持大於 0 的最低副本數
	ErrMultiHostInferenceReplicaCount = CustomError{prefix: errTaskPrefix, code: multiHostInferenceReplicaCount}
	// not enough resource to run the task
	//
	// Description: The task requires more resources than are available in the cluster. This error occurs when the cluster does not have sufficient capacity to run the task.
	//
	// Description_ZH: 任务需要的资源超过了集群可用的资源。当集群资源不足时，会出现此错误。
	//
	// en-US: Not enough resource to run the task
	//
	// zh-CN: 集群资源不足
	//
	// zh-HK: 集群資源不足
	ErrNotEnoughResource = CustomError{prefix: errTaskPrefix, code: notEnoughResource}
	// cluster is unavailable to run the task
	//
	// Description: The cluster is currently unavailable, either due to maintenance or other reasons. This error occurs when the cluster is not ready to accept new tasks.
	//
	// Description_ZH: 集群当前不可用，可能是由于维护或其他原因。当集群未准备好接受新任务时，会出现此错误。
	//
	// en-US: Cluster is unavailable to run the task
	//
	// zh-CN: 集群当前不可用
	//
	// zh-HK: 集群當前不可用
	ErrClusterUnavailable = CustomError{prefix: errTaskPrefix, code: clusterUnavailable}
)

func NoEntryFile(err error, ctx context) error {
	return CustomError{
		prefix:  errTaskPrefix,
		context: ctx,
		err:     err,
		code:    noEntryFile,
	}
}

func NotEnoughResource(err error, ctx context) error {
	return CustomError{
		prefix:  errTaskPrefix,
		context: ctx,
		err:     err,
		code:    notEnoughResource,
	}
}

func ClusterUnavailable(err error, ctx context) error {
	return CustomError{
		prefix:  errTaskPrefix,
		context: ctx,
		err:     err,
		code:    clusterUnavailable,
	}
}
