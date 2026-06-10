package errorx

const errDataSetPrefix = "DAT-ERR"

const (
	dataviewerCardNotFound = iota
	datasetBadFormat
	noValidParquetFile
	applicationStatusNotAllowed
	datasetStatusNotAllowed
	datasetAlreadyReferenced
	relatedDatasetAlreadyReferenced
	pendingApplicationExists
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
	// application status does not allow the review action
	//
	// Description: The dataset application is not in a pending state and cannot be approved or rejected.
	//
	// Description_ZH: 数据集申请当前状态不允许审核操作，只有待审核状态的申请才能被批准或拒绝。
	//
	// en-US: Application status does not allow this operation
	//
	// zh-CN: 申请状态不允许此操作
	//
	// zh-HK: 申請狀態不允許此操作
	ErrApplicationStatusNotAllowed = CustomError{prefix: errDataSetPrefix, code: applicationStatusNotAllowed}
	// dataset status does not allow the application action
	//
	// Description: The dataset is not in a state that allows the application action to be applied.
	//
	// Description_ZH: 数据集当前状态不允许执行此申请操作。
	//
	// en-US: Dataset status does not allow this operation
	//
	// zh-CN: 数据集状态不允许此操作
	//
	// zh-HK: 數據集狀態不允許此操作
	ErrDatasetStatusNotAllowed = CustomError{prefix: errDataSetPrefix, code: datasetStatusNotAllowed}
	// dataset is already referenced by another dataset
	//
	// Description: The dataset is already referenced by another dataset as a related dataset and cannot be referenced again.
	//
	// Description_ZH: 该数据集已被其他数据集引用为关联数据集，不能再次被引用。
	//
	// en-US: Dataset is already referenced by another dataset
	//
	// zh-CN: 该数据集已被其他数据集引用
	//
	// zh-HK: 該數據集已被其他數據集引用
	ErrDatasetAlreadyReferenced = CustomError{prefix: errDataSetPrefix, code: datasetAlreadyReferenced}
	// related dataset is already referenced by another dataset
	//
	// Description: The related dataset is already referenced by another dataset and cannot be used for this application.
	//
	// Description_ZH: 关联数据集已被其他数据集引用，不能用于本次申请。
	//
	// en-US: Related dataset is already referenced by another dataset
	//
	// zh-CN: 关联数据集已被其他数据集引用
	//
	// zh-HK: 關聯數據集已被其他數據集引用
	ErrRelatedDatasetAlreadyReferenced = CustomError{prefix: errDataSetPrefix, code: relatedDatasetAlreadyReferenced}
	// a pending application already exists for this dataset
	//
	// Description: There is already a pending application for this dataset. Only one pending application is allowed per dataset at a time.
	//
	// Description_ZH: 该数据集已有一个待审核的申请。每个数据集同时只允许一个待审核的申请。
	//
	// en-US: A pending application already exists for this dataset
	//
	// zh-CN: 该数据集已有待审核的申请
	//
	// zh-HK: 該數據集已有待審核的申請
	ErrPendingApplicationExists = CustomError{prefix: errDataSetPrefix, code: pendingApplicationExists}
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

func ApplicationStatusNotAllowed(err error, ctx context) error {
	return CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(applicationStatusNotAllowed),
	}
}

func DatasetStatusNotAllowed(err error, ctx context) error {
	return CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(datasetStatusNotAllowed),
	}
}

func DatasetAlreadyReferenced(err error, ctx context) error {
	return CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(datasetAlreadyReferenced),
	}
}

func RelatedDatasetAlreadyReferenced(err error, ctx context) error {
	return CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(relatedDatasetAlreadyReferenced),
	}
}

func PendingApplicationExists(err error, ctx context) error {
	return CustomError{
		prefix:  errDataSetPrefix,
		context: ctx,
		err:     err,
		code:    int(pendingApplicationExists),
	}
}
