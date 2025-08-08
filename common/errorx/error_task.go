package errorx

const errTaskPrefix = "TASK-ERR"

const (
	noEntryFile = iota
)

var (
	ErrNoEntryFile error = CustomError{prefix: errTaskPrefix, code: noEntryFile}
)

func NoEntryFile(err error, ctx context) error {
	return CustomError{
		prefix:  errTaskPrefix,
		context: ctx,
		err:     err,
		code:    noEntryFile,
	}
}
