package errorx

const errTaskPrefix = "TASK-ERR"

const (
	noEntryFile = iota
	multiHostInferenceNotSupported
	multiHostInferenceReplicaCount
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
)

func NoEntryFile(err error, ctx context) error {
	return CustomError{
		prefix:  errTaskPrefix,
		context: ctx,
		err:     err,
		code:    noEntryFile,
	}
}
