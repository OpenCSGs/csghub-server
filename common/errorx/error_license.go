package errorx

const errLicensePrefix = "LICENSE-ERR"

const (
	noActiveLicense = iota
	licenseExpired
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
	// zh-CN: 未找到有效的许可证。
	//
	// zh-HK: 未找到有效的許可證。
	ErrNoActiveLicense error = CustomError{prefix: errLicensePrefix, code: noActiveLicense}
	// license is expired
	//
	// Description: The license is expired, could not be verified and imported.
	//
	// Description_ZH: 许可证已过期，无法验证和导入。
	//
	// en-US: License is expired, could not be verified and imported.
	//
	// zh-CN: 许可证已过期，无法验证和导入。
	//
	// zh-HK: 許可證已過期，無法驗證和導入。
	ErrLicenseExpired error = CustomError{prefix: errLicensePrefix, code: licenseExpired}
)
