package errorx

const errAccountPrefix = "ACT-ERR"

const (
	insufficientBalance = iota
	subscriptionExist
	invalidUnitType
	wrongTimeRange
)

var (
	// account balance is not enough for the operation
	//
	// Description: The user's account balance is insufficient to complete the requested transaction or operation.
	//
	// Description_ZH: 用户账户余额不足，无法完成所请求的交易或操作。
	//
	// en-US: Insufficient balance
	//
	// zh-CN: 帐户余额不足
	//
	// zh-HK: 帳戶餘額不足
	ErrInsufficientBalance error = CustomError{prefix: errAccountPrefix, code: insufficientBalance}
	// user already has an active subscription
	//
	// Description: The user is attempting to subscribe to a service for which they already have an active subscription.
	//
	// Description_ZH: 用户试图订阅一个他们已经拥有有效订阅的服务。
	//
	// en-US: Exist active subscription
	//
	// zh-CN: 存在一个活动订阅, 不能重复创建
	//
	// zh-HK: 存在一個活動訂閱，不能重複創建
	ErrSubscriptionExist error = CustomError{prefix: errAccountPrefix, code: subscriptionExist}
	// the unit type provided is not valid
	//
	// Description: The unit type specified in the request (e.g., for billing) is not recognized or supported.
	//
	// Description_ZH: 请求中指定的单位类型（例如，用于计费的单位）不被系统识别或支持。
	//
	// en-US: Invalid unit type. Must be one of the following: day, week, month, year
	//
	// zh-CN: 非法时间单位. 必须是以下之一: day, week, month, year
	//
	// zh-HK: 非法時間單位。必須是以下之一: day, week, month, year
	ErrInvalidUnitType error = CustomError{prefix: errAccountPrefix, code: invalidUnitType}
	// the time range provided is invalid
	//
	// Description: The specified time range is invalid, for example, the start time is after the end time.
	//
	// Description_ZH: 指定的时间范围无效，例如开始时间晚于结束时间。
	//
	// en-US: Invalid subscription time range
	//
	// zh-CN: 不在订阅有效期内
	//
	// zh-HK: 不在訂閱有效期內
	ErrWrongTimeRange error = CustomError{prefix: errAccountPrefix, code: wrongTimeRange}
)
