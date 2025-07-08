package errorx

const errGitPrefix = "GIT-ERR"

const (
	gitCloneFailed = iota
	gitPullFailed
	gitPushFailed
	gitCommitFailed
	gitAuthFailed
	gitRepoNotFound
	gitBranchNotFound
	gitFileNotFound
	gitUploadFailed
	gitDownloadFailed
	gitConnectionFailed
	gitLfsError
)

var (
	// --- GIT-ERR-xxx: Git/Upload, Download, Resource Synchronization ---
	ErrGitCloneFailed      error = CustomError{prefix: errGitPrefix, code: gitCloneFailed}
	ErrGitPullFailed       error = CustomError{prefix: errGitPrefix, code: gitPullFailed}
	ErrGitPushFailed       error = CustomError{prefix: errGitPrefix, code: gitPushFailed}
	ErrGitCommitFailed     error = CustomError{prefix: errGitPrefix, code: gitCommitFailed}
	ErrGitAuthFailed       error = CustomError{prefix: errGitPrefix, code: gitAuthFailed}
	ErrGitRepoNotFound     error = CustomError{prefix: errGitPrefix, code: gitRepoNotFound}
	ErrGitBranchNotFound   error = CustomError{prefix: errGitPrefix, code: gitBranchNotFound}
	ErrGitFileNotFound     error = CustomError{prefix: errGitPrefix, code: gitFileNotFound}
	ErrGitUploadFailed     error = CustomError{prefix: errGitPrefix, code: gitUploadFailed}
	ErrGitDownloadFailed   error = CustomError{prefix: errGitPrefix, code: gitDownloadFailed}
	ErrGitConnectionFailed error = CustomError{prefix: errGitPrefix, code: gitConnectionFailed}
	ErrGitLfsError         error = CustomError{prefix: errGitPrefix, code: gitLfsError}
)
