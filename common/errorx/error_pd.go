package errorx

const errPDConfigPrefix = "PD-ERR"

const (
	codePDConfigInvalid = iota
)

var (
	// The PD (Prefill-Decode) disaggregation configuration is invalid.
	//
	// Description: The PD disaggregation configuration provided in the request is invalid. This includes missing PD config, missing prefill or decode role, invalid TP/DP/EP/PodsSize values, or GPU count mismatch.
	//
	// Description_ZH: 请求中提供的PD分离部署配置无效。包括缺少PD配置、缺少prefill或decode角色、TP/DP/EP/PodsSize值无效，或GPU数量不匹配。
	//
	// en-US: Invalid PD disaggregation configuration
	//
	// zh-CN: PD分离部署配置无效
	//
	// zh-HK: PD分離部署配置無效
	ErrPDConfigInvalid error = CustomError{prefix: errPDConfigPrefix, code: codePDConfigInvalid}
)

func PDConfigInvalid(originErr error, ext context) error {
	return CustomError{
		prefix:  errPDConfigPrefix,
		code:    codePDConfigInvalid,
		err:     originErr,
		context: ext,
	}
}
