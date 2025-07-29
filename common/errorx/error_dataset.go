package errorx

const errDataSetPrefix = "DAT-ERR"

const (
	dataviewerCardNotFound = iota
	datasetBadFormat
	noValidParquetFile
)

var (
	ErrDataviewerCardNotFound = CustomError{prefix: errDataSetPrefix, code: dataviewerCardNotFound}
	ErrDatasetBadFormat       = CustomError{prefix: errDataSetPrefix, code: datasetBadFormat}
	ErrNoValidParquetFile     = CustomError{prefix: errDataSetPrefix, code: noValidParquetFile}
)

func DataviewerCardNotFound(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(dataviewerCardNotFound),
	}
	return customErr
}

func DatasetBadFormat(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(datasetBadFormat),
	}
	return customErr
}

func NoValidParquetFile(err error, ctx context) error {
	customErr := CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(noValidParquetFile),
	}
	return customErr
}
