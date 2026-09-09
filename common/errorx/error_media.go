package errorx

const errMediaPrefix = "MEDIA-ERR"

const (
	codeMediaModerationPending = iota
	codeMediaModerationUnavailable
)

var (
	// Description: The comment is under media moderation and cannot be edited.
	//
	// Description_ZH: 评论正在审核中，无法编辑
	//
	// en-US: The comment is under media moderation and cannot be edited.
	//
	// zh-CN: 评论正在审核中，无法编辑
	//
	// zh-HK: 評論正在審核中，無法編輯
	ErrMediaModerationPending error = CustomError{prefix: errMediaPrefix, code: codeMediaModerationPending}

	// Description: The media moderation service is unavailable.
	//
	// Description_ZH: 媒体审核服务不可用
	//
	// en-US: The media moderation service is unavailable.
	//
	// zh-CN: 媒体审核服务不可用
	//
	// zh-HK: 媒體審核服務不可用
	ErrMediaModerationUnavailable error = CustomError{prefix: errMediaPrefix, code: codeMediaModerationUnavailable}
)
