package errorx

const errDataSetPrefix = "DAT-ERR"

const (
	dataviewerCardNotFound = iota
	datasetBadFormat
	noValidParquetFile
)

var (
	// dataviewer card not found
	//
	// Description: The requested dataviewer card could not be located within the system or the specified dataset.
	//
	// Description_ZH: 在系统或指定的数据集中找不到所请求的数据可视化卡片。
	//
	// en-US: Dataviewer card not found
	//
	// zh-CN: 未找到数据可视化卡片
	//
	// zh-HK: 未找到數據可視化卡片
	ErrDataviewerCardNotFound = CustomError{prefix: errDataSetPrefix, code: dataviewerCardNotFound}
	// dataset has a bad format
	//
	// Description: The uploaded or specified dataset is not in a valid or expected format. Please check the file structure and data types.
	//
	// Description_ZH: 上传或指定的数据集格式无效或不符合预期。请检查文件结构和数据类型。
	//
	// en-US: Dataset format is invalid
	//
	// zh-CN: 数据集格式错误
	//
	// zh-HK: 數據集格式錯誤
	ErrDatasetBadFormat = CustomError{prefix: errDataSetPrefix, code: datasetBadFormat}
	// no valid parquet file found in the dataset
	//
	// Description: The dataset does not contain any valid Parquet files, which are required for this operation.
	//
	// Description_ZH: 数据集中不包含任何有效的Parquet文件，而此操作需要该文件格式。
	//
	// en-US: No valid Parquet file found
	//
	// zh-CN: 未找到有效的Parquet文件
	//
	// zh-HK: 未找到有效的Parquet檔案
	ErrNoValidParquetFile = CustomError{prefix: errDataSetPrefix, code: noValidParquetFile}
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
