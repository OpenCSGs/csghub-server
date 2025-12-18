package errorx

import "fmt"

const errRunnerPrefix = "RUNNER-ERR"
const (
	codeRunnerMaxRevisionErr = iota
	codeRunnerGetMaxScaleFailedErr
	codeRunnerDuplicateRevisionErr
	codeRevisionNotReadyErr
	codeTrafficPercentNotZeroErr
)

var RunnerErrors = map[string]error{
	fmt.Sprintf("%s-%d", errRunnerPrefix, codeRunnerMaxRevisionErr):       ErrRunnerMaxRevision,
	fmt.Sprintf("%s-%d", errRunnerPrefix, codeRunnerGetMaxScaleFailedErr): ErrRunnerGetMaxScaleFailed,
	fmt.Sprintf("%s-%d", errRunnerPrefix, codeRunnerDuplicateRevisionErr): ErrRunnerDuplicateRevision,
	fmt.Sprintf("%s-%d", errServerlessPrefix, codeInvalidPercentErr):      ErrInvalidPercent,
	fmt.Sprintf("%s-%d", errServerlessPrefix, codeRevisionNotFoundErr):    ErrRevisionNotFound,
	fmt.Sprintf("%s-%d", errRunnerPrefix, codeRevisionNotReadyErr):        ErrRevisionNotReady,
	fmt.Sprintf("%s-%d", errRunnerPrefix, codeTrafficPercentNotZeroErr):   ErrTrafficPercentNotZero,
}

var (
	// Description: The max revision number exceeds the max replica number.
	//
	// Description_ZH: 最大版本数量超过最大弹性副本数
	//
	// en-US: The max revision number exceeds the max replica number.
	//
	// zh-CN: 最大版本数量超过最大弹性副本数
	//
	// zh-HK: 最大版本數量超過最大弹性副本數
	ErrRunnerMaxRevision error = CustomError{prefix: errRunnerPrefix, code: codeRunnerMaxRevisionErr}

	// Description: Failed to get max scale.
	//
	// Description_ZH: 获取最大弹性副本数失败
	//
	// en-US: Failed to get max scale.
	//
	// zh-CN: 获取最大弹性副本数失败
	//
	// zh-HK: 獲取最大弹性副本數失敗
	ErrRunnerGetMaxScaleFailed error = CustomError{prefix: errRunnerPrefix, code: codeRunnerGetMaxScaleFailedErr}

	// Description: The revision with commit already exists.
	//
	// Description_ZH: 版本实例已存在
	//
	// en-US: The revision with commit already exists.
	//
	// zh-CN: 版本实例已存在
	//
	// zh-HK: 版本實例已存在
	ErrRunnerDuplicateRevision error = CustomError{prefix: errRunnerPrefix, code: codeRunnerDuplicateRevisionErr}

	// Description: The revision is not ready.
	//
	// Description_ZH: 版本实例未就绪
	//
	// en-US: The revision is not ready.
	//
	// zh-CN: 版本实例未就绪
	//
	// zh-HK: 版本實例未就绪
	ErrRevisionNotReady error = CustomError{prefix: errRunnerPrefix, code: codeRevisionNotReadyErr}

	// Description: The traffic percent is not zero.
	//
	// Description_ZH: 当前版本仍有流量分配（流量占比≠0）
	//
	// en-US: The traffic percent is not zero.
	//
	// zh-CN: 当前版本仍有流量分配（流量占比≠0）
	//
	// zh-HK: 当前版本仍有流量分配（流量占比≠0）
	ErrTrafficPercentNotZero error = CustomError{prefix: errRunnerPrefix, code: codeTrafficPercentNotZeroErr}
)
