package errorx

const errMirrorPrefix = "MIRROR-ERR"

const (
	mirrorSourceConflict = iota
	mirrorRepoSyncing
	mirrorRepoSyncFailed
	mirrorTaskStateInvalid
	mirrorRepoSyncCanceled
	mirrorSourceRepoAuthInvalid
)

var (
	// ErrMirrorSourceConflict mirror source conflicts with an existing target repository mirror
	//
	// Description: The target repository already has another mirror source.
	//
	// Description_ZH: 目标仓库已绑定其他镜像源。
	//
	// en-US: The target repository already has another mirror source.
	//
	// zh-CN: 目标仓库已绑定其他镜像源。
	//
	// zh-HK: 目標倉庫已綁定其他鏡像源。
	ErrMirrorSourceConflict error = CustomError{prefix: errMirrorPrefix, code: mirrorSourceConflict}

	// ErrMirrorRepoSyncing indicates repository Git data is not ready because synchronization is active.
	//
	// Description: Repository synchronization is in progress.
	//
	// Description_ZH: 仓库正在同步中。
	//
	// en-US: Repository synchronization is in progress.
	//
	// zh-CN: 仓库正在同步中。
	//
	// zh-HK: 倉庫正在同步中。
	ErrMirrorRepoSyncing error = CustomError{prefix: errMirrorPrefix, code: mirrorRepoSyncing}

	// ErrMirrorRepoSyncFailed indicates repository synchronization failed and Git data could not be retrieved.
	//
	// Description: Repository synchronization failed and Git data could not be retrieved.
	//
	// Description_ZH: 仓库同步失败，未能获取 Git 数据。
	//
	// en-US: Repository synchronization failed.
	//
	// zh-CN: 仓库同步失败。
	//
	// zh-HK: 倉庫同步失敗。
	ErrMirrorRepoSyncFailed error = CustomError{prefix: errMirrorPrefix, code: mirrorRepoSyncFailed}

	// ErrMirrorTaskStateInvalid indicates that the current mirror task state is inconsistent with its execution status.
	//
	// Description: The current mirror task state is inconsistent with its execution status and cannot be determined reliably.
	//
	// Description_ZH: 当前镜像任务状态与执行状态不一致，无法可靠确定任务状态。
	//
	// en-US: The mirror task state is invalid.
	//
	// zh-CN: 镜像任务状态异常。
	//
	// zh-HK: 鏡像任務狀態異常。
	ErrMirrorTaskStateInvalid error = CustomError{prefix: errMirrorPrefix, code: mirrorTaskStateInvalid}

	// ErrMirrorRepoSyncCanceled indicates repository synchronization was canceled before Git data became available.
	//
	// Description: Repository synchronization was canceled before Git data became available.
	//
	// Description_ZH: 仓库 Git 数据可用前同步任务已取消。
	//
	// en-US: Repository synchronization was canceled.
	//
	// zh-CN: 仓库同步已取消。
	//
	// zh-HK: 倉庫同步已取消。
	ErrMirrorRepoSyncCanceled error = CustomError{prefix: errMirrorPrefix, code: mirrorRepoSyncCanceled}

	// ErrMirrorSourceRepoAuthInvalid indicates invalid source repository authentication information.
	//
	// Description: Source repository authentication information is invalid.
	//
	// Description_ZH: 源仓库鉴权信息错误。
	//
	// en-US: Source repository authentication information is invalid.
	//
	// zh-CN: 源仓库鉴权信息错误。
	//
	// zh-HK: 源倉庫鑒權信息錯誤。
	ErrMirrorSourceRepoAuthInvalid error = CustomError{prefix: errMirrorPrefix, code: mirrorSourceRepoAuthInvalid}
)

// MirrorSourceConflict wraps a mirror source conflict with optional context.
func MirrorSourceConflict(err error, ctx context) error {
	return CustomError{
		prefix:  errMirrorPrefix,
		context: ctx,
		err:     err,
		code:    int(mirrorSourceConflict),
	}
}

// MirrorRepoSyncing wraps an active repository synchronization result with optional context.
func MirrorRepoSyncing(err error, ctx context) error {
	return CustomError{prefix: errMirrorPrefix, context: ctx, err: err, code: int(mirrorRepoSyncing)}
}

// MirrorRepoSyncFailed wraps a terminal repository synchronization result with optional context.
func MirrorRepoSyncFailed(err error, ctx context) error {
	return CustomError{prefix: errMirrorPrefix, context: ctx, err: err, code: int(mirrorRepoSyncFailed)}
}

// MirrorTaskStateInvalid wraps an inconsistent task state with optional context.
func MirrorTaskStateInvalid(err error, ctx context) error {
	return CustomError{prefix: errMirrorPrefix, context: ctx, err: err, code: int(mirrorTaskStateInvalid)}
}

// MirrorRepoSyncCanceled wraps a canceled repository synchronization result with optional context.
func MirrorRepoSyncCanceled(err error, ctx context) error {
	return CustomError{prefix: errMirrorPrefix, context: ctx, err: err, code: int(mirrorRepoSyncCanceled)}
}

// MirrorSourceRepoAuthInvalid wraps invalid source repository authentication information with optional context.
func MirrorSourceRepoAuthInvalid(err error, ctx context) error {
	return CustomError{prefix: errMirrorPrefix, context: ctx, err: err, code: int(mirrorSourceRepoAuthInvalid)}
}
