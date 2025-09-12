package errorx

const errUserPrefix = "USER-ERR"

const (
	needPhone = iota
	needDifferentPhone
	phoneAlreadyExistsInSSO
	forbidChangePhone
	failedToUpdatePhone
	forbidSendPhoneVerifyCodeFrequently
	failedSendPhoneVerifyCode
	phoneVerifyCodeExpiredOrNotFound
	phoneVerifyCodeInvalid
)

var (
	// phone number is required
	//
	// Description: The request must include a phone number in the header or body to identify the target account.
	//
	// Description_ZH: 请求必须在请求头或正文中包含电话号码以识别目标账户。
	//
	// en-US: Phone number is required
	//
	// zh-CN: 需要提供电话号码
	//
	// zh-HK: 需要電話號碼
	ErrNeedPhone error = CustomError{prefix: errUserPrefix, code: needPhone}
	// new phone number must be different from current phone number
	//
	// Description: The new phone number must be different from the current phone number.
	//
	// Description_ZH: 新电话号码必须与当前电话号码不同。
	//
	// en-US: New phone number must be different from current phone number
	//
	// zh-CN: 新电话号码必须与当前电话号码不同
	//
	// zh-HK: 新電話號碼必須與當前電話號碼不同
	ErrNeedDifferentPhone error = CustomError{prefix: errUserPrefix, code: needDifferentPhone}
	// new phone number already exists in sso service
	//
	// Description: The new phone number already exists in sso service.
	//
	// Description_ZH: 新电话号码已经存在于sso服务中。
	//
	// en-US: New phone number already exists in sso service
	//
	// zh-CN: 新电话号码已经存在于sso服务中
	//
	// zh-HK: 新電話號碼已經存在於sso服務中
	ErrPhoneAlreadyExistsInSSO error = CustomError{prefix: errUserPrefix, code: phoneAlreadyExistsInSSO}
	// forbid change phone number
	//
	// Description: The phone number cannot be changed.
	//
	// Description_ZH: 电话号码不能被更改。
	//
	// en-US: Forbid change phone number
	//
	// zh-CN: 禁止更改电话号码
	//
	// zh-HK: 禁止更改電話號碼
	ErrForbidChangePhone error = CustomError{prefix: errUserPrefix, code: forbidChangePhone}
	// failed to update phone number
	//
	// Description: Failed to update phone number.
	//
	// Description_ZH: 更新电话号码失败。
	//
	// en-US: Failed to update phone number
	//
	// zh-CN: 更新电话号码失败
	//
	// zh-HK: 更新電話號碼失敗
	ErrFailedToUpdatePhone error = CustomError{prefix: errUserPrefix, code: failedToUpdatePhone}
	// forbid send phone verify code frequently
	//
	// Description: Send phone verify code frequently.
	//
	// Description_ZH: 发送手机验证码过于频繁。
	//
	// en-US: Forbid send phone verify code frequently
	//
	// zh-CN: 禁止频繁发送手机验证码
	//
	// zh-HK: 禁止頻繁發送手機驗證碼
	ErrForbidSendPhoneVerifyCodeFrequently error = CustomError{prefix: errUserPrefix, code: forbidSendPhoneVerifyCodeFrequently}
	// failed to send phone verify code
	//
	// Description: Failed to send phone verify code.
	//
	// Description_ZH: 发送手机验证码失败。
	//
	// en-US: Failed to send phone verify code
	//
	// zh-CN: 发送手机验证码失败
	//
	// zh-HK: 發送手機驗證碼失敗
	ErrFailedSendPhoneVerifyCode error = CustomError{prefix: errUserPrefix, code: failedSendPhoneVerifyCode}
	// phone verify code expired or not found
	//
	// Description: Phone verify code expired or not found.
	//
	// Description_ZH: 手机验证码已过期或不存在。
	//
	// en-US: Phone verify code expired or not found
	//
	// zh-CN: 手机验证码已过期或不存在
	//
	// zh-HK: 手機驗證碼已過期或不存在
	ErrPhoneVerifyCodeExpiredOrNotFound error = CustomError{prefix: errUserPrefix, code: phoneVerifyCodeExpiredOrNotFound}
	// phone verify code is invalid
	//
	// Description: Phone verify code is invalid.
	//
	// Description_ZH: 手机验证码无效。
	//
	// en-US: Phone verify code is invalid
	//
	// zh-CN: 手机验证码无效
	//
	// zh-HK: 手機驗證碼無效
	ErrPhoneVerifyCodeInvalid error = CustomError{prefix: errUserPrefix, code: phoneVerifyCodeInvalid}
)
