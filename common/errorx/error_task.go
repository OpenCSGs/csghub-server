package errorx

const errTaskPrefix = "TASK-ERR"

const (
	noEntryFile = iota
)

var (
	// no entry file found for the task
	//
	// Description: The task requires a specific entry file to start execution (e.g., 'main.py' or 'app.js'), but no such file could be found in the specified source directory.
	//
	// Description_ZH: 该任务需要一个特定的入口文件来开始执行（例如 'main.py' 或 'app.js'），但在指定的源目录中找不到这样的文件。
	//
	// en-US: Task entry file not found
	//
	// zh-CN: 未找到任务入口文件
	//
	// zh-HK: 未找到任務入口檔案
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
