package errorx

const errInvitationPrefix = "INVITATION-ERR"

const (
	userPhoneNotSet = iota
	invitationNotFound
	userAlreadyHasInvitationCode
)

var (
	// phone number isn't set, cannot create invitation code
	//
	// Description: The phone number is not set, cannot create invitation code.
	//
	// Description_ZH: 未绑定手机号，不能创建邀请码。
	//
	// en-US: phone number is not set, cannot create invitation code
	//
	// zh-CN: 未绑定手机号，不能创建邀请码
	//
	// zh-HK: 未綁定手機號，不能創建邀請碼
	ErrUserPhoneNotSet error = CustomError{prefix: errInvitationPrefix, code: userPhoneNotSet}
	// invitation not found
	//
	// Description: The invitation not found.
	//
	// Description_ZH: 邀请码不存在。
	//
	// en-US: Invitation not found
	//
	// zh-CN: 邀请码不存在
	//
	// zh-HK: 邀請碼不存在
	ErrInvitationNotFound error = CustomError{prefix: errInvitationPrefix, code: invitationNotFound}
	// invitation code already exists
	//
	// Description: The invitation code already exists.
	//
	// Description_ZH: 邀请码已存在。
	//
	// en-US: Invitation code already exists
	//
	// zh-HK: 邀請碼已存在
	ErrUserAlreadyHasInvitationCode error = CustomError{prefix: errInvitationPrefix, code: userAlreadyHasInvitationCode}
)
