package errorx

import "fmt"

const errGitPrefix = "GIT-ERR"

type errGit struct {
	code errGitCode
	msg  string
}

func (err errGit) Error() string {
	return err.msg
}

func (err errGit) ErrorWithCode() string {
	return errGitPrefix + "-" + fmt.Sprintf("%d", err.code) + ":" + err.msg
}

type errGitCode int

const (
	gitCloneFailed errGitCode = iota
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
	ErrGitCloneFailed      = errGit{code: gitCloneFailed, msg: "git clone failed"}
	ErrGitPullFailed       = errGit{code: gitPullFailed, msg: "git pull failed"}
	ErrGitPushFailed       = errGit{code: gitPushFailed, msg: "git push failed"}
	ErrGitCommitFailed     = errGit{code: gitCommitFailed, msg: "git commit failed"}
	ErrGitAuthFailed       = errGit{code: gitAuthFailed, msg: "git authentication failed"}
	ErrGitRepoNotFound     = errGit{code: gitRepoNotFound, msg: "git repository not found"}
	ErrGitBranchNotFound   = errGit{code: gitBranchNotFound, msg: "git branch not found"}
	ErrGitFileNotFound     = errGit{code: gitFileNotFound, msg: "file not found in git repository"}
	ErrGitUploadFailed     = errGit{code: gitUploadFailed, msg: "file upload failed"}
	ErrGitDownloadFailed   = errGit{code: gitDownloadFailed, msg: "file download failed"}
	ErrGitConnectionFailed = errGit{code: gitConnectionFailed, msg: "git service connection failed"}
	ErrGitLfsError         = errGit{code: gitLfsError, msg: "git lfs operation error"}
)
