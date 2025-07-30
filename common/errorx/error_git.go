package errorx

const errGitPrefix = "GIT-ERR"

const (
	gitCloneFailed = iota
	gitPullFailed
	gitPushFailed
	// git commit related errors
	gitCommitFailed
	gitFindCommitFailed
	gitCountCommitsFailed
	gitCommitNotFound
	// git diff related errors
	gitDiffFailed
	gitAuthFailed
	gitRepoNotFound
	// git branch related errors
	gitFindBranchFailed
	gitBranchNotFound
	gitDeleteBranchFailed
	gitFileNotFound
	gitUploadFailed
	gitDownloadFailed
	gitConnectionFailed
	gitLfsError
	fileTooLarge
	gitGetTreeEntryFailed
	gitCommitFilesFailed
	gitGetBlobsFailed
	gitGetLfsPointersFailed
	gitListLastCommitsForTreeFailed
	gitGetBlobInfoFailed
	gitListFilesFailed
	gitCreateMirrorFailed
	gitMirrorSyncFailed
	gitCheckRepositoryExistsFailed
	gitCreateRepositoryFailed
	gitDeleteRepositoryFailed
	gitGetRepositoryFailed
	gitServiceUnavaliable
)

var (
	// --- GIT-ERR-xxx: Git/Upload, Download, Resource Synchronization ---
	ErrGitCloneFailed error = CustomError{prefix: errGitPrefix, code: gitCloneFailed}
	ErrGitPullFailed  error = CustomError{prefix: errGitPrefix, code: gitPullFailed}
	ErrGitPushFailed  error = CustomError{prefix: errGitPrefix, code: gitPushFailed}

	ErrGitCommitFailed       error = CustomError{prefix: errGitPrefix, code: gitCommitFailed}
	ErrGitFindCommitFailed   error = CustomError{prefix: errGitPrefix, code: gitFindCommitFailed}
	ErrGitCountCommitsFailed error = CustomError{prefix: errGitPrefix, code: gitCountCommitsFailed}
	ErrGitCommitNotFound     error = CustomError{prefix: errGitPrefix, code: gitCommitNotFound}

	ErrGitDiffFailed error = CustomError{prefix: errGitPrefix, code: gitDiffFailed}

	ErrGitAuthFailed   error = CustomError{prefix: errGitPrefix, code: gitAuthFailed}
	ErrGitRepoNotFound error = CustomError{prefix: errGitPrefix, code: gitRepoNotFound}

	ErrGitFindBranchFailed   error = CustomError{prefix: errGitPrefix, code: gitFindBranchFailed}
	ErrGitBranchNotFound     error = CustomError{prefix: errGitPrefix, code: gitBranchNotFound}
	ErrGitDeleteBranchFailed error = CustomError{prefix: errGitPrefix, code: gitDeleteBranchFailed}

	ErrGitFileNotFound     error = CustomError{prefix: errGitPrefix, code: gitFileNotFound}
	ErrGitUploadFailed     error = CustomError{prefix: errGitPrefix, code: gitUploadFailed}
	ErrGitDownloadFailed   error = CustomError{prefix: errGitPrefix, code: gitDownloadFailed}
	ErrGitConnectionFailed error = CustomError{prefix: errGitPrefix, code: gitConnectionFailed}
	ErrGitLfsError         error = CustomError{prefix: errGitPrefix, code: gitLfsError}
	ErrFileTooLarge        error = CustomError{prefix: errGitPrefix, code: fileTooLarge} // Custom error for file size limit exceeded
)

func FindCommitFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitFindCommitFailed,
		err:     err,
		context: ctx,
	}
}

func CommitFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCommitFailed,
		err:     err,
		context: ctx,
	}
}

func CountCommitsFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCountCommitsFailed,
		err:     err,
		context: ctx,
	}
}

func CommitNotFound(ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCommitNotFound,
		err:     ErrGitCommitNotFound,
		context: ctx,
	}
}

func DiffFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitDiffFailed,
		err:     err,
		context: ctx,
	}
}

func FindBranchFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitFindBranchFailed,
		err:     err,
		context: ctx,
	}
}

func BranchNotFound(ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitBranchNotFound,
		err:     ErrGitBranchNotFound,
		context: ctx,
	}
}

func DeleteBranchFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitDeleteBranchFailed,
		err:     err,
		context: ctx,
	}
}

func GitFileNotFound(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitFileNotFound,
		err:     err,
		context: ctx,
	}
}

func ErrGitGetTreeEntryFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitGetTreeEntryFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitCommitFilesFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCommitFilesFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitGetBlobsFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitGetBlobsFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitGetLfsPointersFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitGetLfsPointersFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitListLastCommitsForTreeFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitListLastCommitsForTreeFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitGetBlobInfoFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitGetBlobInfoFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitListFilesFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitListFilesFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitCreateMirrorFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCreateMirrorFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitMirrorSyncFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitMirrorSyncFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitCheckRepositoryExistsFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCheckRepositoryExistsFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitCreateRepositoryFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCreateRepositoryFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitDeleteRepositoryFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitDeleteRepositoryFailed,
		err:     err,
		context: ctx,
	}
}

func ErrGitGetRepositoryFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitGetRepositoryFailed,
		err:     err,
		context: ctx,
	}
}
