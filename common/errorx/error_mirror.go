package errorx

const errMirrorPrefix = "MIRROR-ERR"

const (
	mirrorSourceConflict = iota
)

var (
	// mirror source conflicts with an existing target repository mirror
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
