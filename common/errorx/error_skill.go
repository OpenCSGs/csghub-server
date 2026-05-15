package errorx

const errSkillPrefix = "SKILL-ERR"

const (
	skillNotFound = iota
	skillVersionNotFound
	skillPublishFailed
	skillDownloadFailed
	skillResolveFailed
	skillUserNotFound
	skillVersionCreateFailed
	skillVersionUpdateFailed
	skillPublishFileCountExceeded
	skillPublishFileSizeExceeded
)

var (
	// skill not found
	//
	// Description: The requested skill could not be found.
	//
	// Description_ZH: 找不到请求的技能。
	//
	// en-US: Skill not found
	//
	// zh-CN: 技能未找到
	//
	// zh-HK: 技能未找到
	ErrSkillNotFound error = CustomError{prefix: errSkillPrefix, code: skillNotFound}

	// skill version not found
	//
	// Description: The requested skill version could not be found.
	//
	// Description_ZH: 找不到请求的技能版本。
	//
	// en-US: Skill version not found
	//
	// zh-CN: 技能版本未找到
	//
	// zh-HK: 技能版本未找到
	ErrSkillVersionNotFound error = CustomError{prefix: errSkillPrefix, code: skillVersionNotFound}

	// skill publish failed
	//
	// Description: Failed to publish the skill.
	//
	// Description_ZH: 发布技能失败。
	//
	// en-US: Failed to publish skill
	//
	// zh-CN: 发布技能失败
	//
	// zh-HK: 發佈技能失敗
	ErrSkillPublishFailed error = CustomError{prefix: errSkillPrefix, code: skillPublishFailed}

	// skill download failed
	//
	// Description: Failed to download the skill archive.
	//
	// Description_ZH: 下载技能包失败。
	//
	// en-US: Failed to download skill
	//
	// zh-CN: 下载技能失败
	//
	// zh-HK: 下載技能失敗
	ErrSkillDownloadFailed error = CustomError{prefix: errSkillPrefix, code: skillDownloadFailed}

	// skill resolve failed
	//
	// Description: Failed to resolve the skill.
	//
	// Description_ZH: 解析技能失败。
	//
	// en-US: Failed to resolve skill
	//
	// zh-CN: 解析技能失败
	//
	// zh-HK: 解析技能失敗
	ErrSkillResolveFailed error = CustomError{prefix: errSkillPrefix, code: skillResolveFailed}

	// user not found for skill operation
	//
	// Description: The user associated with the skill operation could not be found.
	//
	// Description_ZH: 找不到与该技能操作关联的用户。
	//
	// en-US: User not found for skill operation
	//
	// zh-CN: 未找到技能操作关联的用户
	//
	// zh-HK: 未找到技能操作關聯的用戶
	ErrSkillUserNotFound error = CustomError{prefix: errSkillPrefix, code: skillUserNotFound}

	// skill version create failed
	//
	// Description: Failed to create skill version record.
	//
	// Description_ZH: 创建技能版本记录失败。
	//
	// en-US: Failed to create skill version
	//
	// zh-CN: 创建技能版本失败
	//
	// zh-HK: 創建技能版本失敗
	ErrSkillVersionCreateFailed error = CustomError{prefix: errSkillPrefix, code: skillVersionCreateFailed}

	// skill version update failed
	//
	// Description: Failed to update skill version record.
	//
	// Description_ZH: 更新技能版本记录失败。
	//
	// en-US: Failed to update skill version
	//
	// zh-CN: 更新技能版本失败
	//
	// zh-HK: 更新技能版本失敗
	ErrSkillVersionUpdateFailed error = CustomError{prefix: errSkillPrefix, code: skillVersionUpdateFailed}

	// publish file count exceeds limit
	//
	// Description: The number of files in the skill publish request exceeds the allowed limit.
	//
	// Description_ZH: 技能发布请求中的文件数量超过了允许的上限。
	//
	// en-US: Too many files in skill publish request
	//
	// zh-CN: 技能发布文件数量超过上限
	//
	// zh-HK: 技能發佈文件數量超過上限
	ErrSkillPublishFileCountExceeded error = CustomError{prefix: errSkillPrefix, code: skillPublishFileCountExceeded}

	// publish file size exceeds limit
	//
	// Description: The total size of files in the skill publish request exceeds the allowed limit.
	//
	// Description_ZH: 技能发布请求中文件的总大小超过了允许的上限。
	//
	// en-US: Total file size too large in skill publish request
	//
	// zh-CN: 技能发布文件总大小超过上限
	//
	// zh-HK: 技能發佈文件總大小超過上限
	ErrSkillPublishFileSizeExceeded error = CustomError{prefix: errSkillPrefix, code: skillPublishFileSizeExceeded}
)

func SkillNotFound(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillNotFound,
		err:     err,
		context: errCtx,
	}
}

func SkillVersionNotFound(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillVersionNotFound,
		err:     err,
		context: errCtx,
	}
}

func SkillPublishFailed(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillPublishFailed,
		err:     err,
		context: errCtx,
	}
}

func SkillDownloadFailed(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillDownloadFailed,
		err:     err,
		context: errCtx,
	}
}

func SkillResolveFailed(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillResolveFailed,
		err:     err,
		context: errCtx,
	}
}

func SkillUserNotFound(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillUserNotFound,
		err:     err,
		context: errCtx,
	}
}

func SkillVersionCreateFailed(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillVersionCreateFailed,
		err:     err,
		context: errCtx,
	}
}

func SkillVersionUpdateFailed(err error, errCtx context) error {
	return CustomError{
		prefix:  errSkillPrefix,
		code:    skillVersionUpdateFailed,
		err:     err,
		context: errCtx,
	}
}

func SkillPublishFileCountExceeded(maxCount, actualCount int) error {
	return CustomError{
		prefix: errSkillPrefix,
		code:   skillPublishFileCountExceeded,
		context: context{
			"max_count":    maxCount,
			"actual_count": actualCount,
		},
	}
}

func SkillPublishFileSizeExceeded(maxSize, actualSize int64) error {
	return CustomError{
		prefix: errSkillPrefix,
		code:   skillPublishFileSizeExceeded,
		context: context{
			"max_size":    maxSize,
			"actual_size": actualSize,
		},
	}
}
