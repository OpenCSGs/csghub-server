package errorx

const errLicensePrefix = "LICENSE-ERR"

const (
	noActiveLicense = iota
	licenseExpired
	licenseFeatureDisabled
	licenseLimitExceeded
	invalidLicenseExtra
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
	// feature is disabled by license
	//
	// Description: The feature is not enabled by the current license.
	//
	// Description_ZH: 当前许可证未启用该功能。
	//
	// en-US: Feature is not enabled by license
	//
	// zh-CN: 当前许可证未启用该功能。
	//
	// zh-HK: 當前許可證未啟用該功能。
	ErrLicenseFeatureDisabled error = CustomError{prefix: errLicensePrefix, code: licenseFeatureDisabled}
	// resource limit exceeded
	//
	// Description: The operation exceeds the limit allowed by the current license.
	//
	// Description_ZH: 当前操作超出许可证允许的限制。
	//
	// en-US: License limit exceeded
	//
	// zh-CN: 当前操作超出许可证允许的限制。
	//
	// zh-HK: 當前操作超出許可證允許的限制。
	ErrLicenseLimitExceeded error = CustomError{prefix: errLicensePrefix, code: licenseLimitExceeded}
	// license extra is invalid
	//
	// Description: The license extra data is invalid, could not be created, updated or imported.
	//
	// Description_ZH: 许可证扩展数据无效，无法创建、更新或导入。
	//
	// en-US: License extra is invalid
	//
	// zh-CN: 许可证扩展数据无效。
	//
	// zh-HK: 許可證擴展數據無效。
	ErrInvalidLicenseExtra error = CustomError{prefix: errLicensePrefix, code: invalidLicenseExtra}
)

// NewInvalidLicenseExtraError wraps the underlying validation detail (unknown
// flag/limit key or bad JSON) so the 400 response carries an actionable
// message while remaining identifiable via errors.Is(err, ErrInvalidLicenseExtra).
func NewInvalidLicenseExtraError(err error) error {
	return NewCustomError(errLicensePrefix, invalidLicenseExtra, err, nil)
}
