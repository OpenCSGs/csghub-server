package errorx

const errLicensePrefix = "LICENSE-ERR"

const (
	noActiveLicense = iota
)

var (
	// no active license found
	//
	// Description: No active license found for the current system.
	//
	// Description_ZH: 当前系统没有有效的许可证。
	//
	// en-US: No active license found
	//
	// zh-CN: 未找到有效的许可证
	//
	// zh-HK: 未找到有效的許可證
	ErrNoActiveLicense error = CustomError{prefix: errLicensePrefix, code: noActiveLicense}
)
