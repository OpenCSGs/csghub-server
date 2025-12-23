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
	gitCreateBranchFailed
	gitSetDefaultBranchFailed
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
	gitCopyRepositoryFailed
	gitReplicateRepositoryFailed
	gitUsingGitInXnetRepository
)

var (
	// --- GIT-ERR-xxx: Git/Upload, Download, Resource Synchronization ---
	// git clone operation failed
	//
	// Description: The attempt to clone a remote Git repository to the local system failed. This could be due to network issues, incorrect repository URL, or insufficient permissions.
	//
	// Description_ZH: 尝试将远程 Git 仓库克隆到本地系统失败。这可能是由于网络问题、不正确的仓库 URL 或权限不足造成的。
	//
	// en-US: Failed to clone repository
	//
	// zh-CN: 克隆仓库失败
	//
	// zh-HK: 克隆儲存庫失敗
	ErrGitCloneFailed error = CustomError{prefix: errGitPrefix, code: gitCloneFailed}
	// git pull operation failed
	//
	// Description: Failed to fetch from and integrate with another repository or a local branch. This can be caused by merge conflicts, network problems, or authentication issues.
	//
	// Description_ZH: 从另一个仓库或本地分支获取并集成失败。这可能是由合并冲突、网络问题或身份验证问题引起的。
	//
	// en-US: Failed to pull changes from repository
	//
	// zh-CN: 从仓库拉取更新失败
	//
	// zh-HK: 從儲存庫拉取更新失敗
	ErrGitPullFailed error = CustomError{prefix: errGitPrefix, code: gitPullFailed}
	// git push operation failed
	//
	// Description: Failed to update remote refs along with associated objects. This might happen if the remote branch has new commits, or due to insufficient push permissions.
	//
	// Description_ZH: 更新远程引用及其关联对象失败。如果远程分支有新的提交，或者推送权限不足，可能会发生这种情况。
	//
	// en-US: Failed to push changes to repository
	//
	// zh-CN: 推送更新到仓库失败
	//
	// zh-HK: 推送更新到儲存庫失敗
	ErrGitPushFailed error = CustomError{prefix: errGitPrefix, code: gitPushFailed}
	// git commit operation failed
	//
	// Description: The attempt to record changes to the repository failed. This could be due to an empty staging area, a pre-commit hook failure, or incorrect user configuration.
	//
	// Beschrijving_zh: 尝试将更改记录到仓库失败。这可能是由于暂存区为空、提交前挂钩（pre-commit hook）失败或不正确的用户配置造成的。
	//
	// en-US: Failed to commit changes
	//
	// zh-CN: 提交更改失败
	//
	// zh-HK: 提交變更失敗
	ErrGitCommitFailed error = CustomError{prefix: errGitPrefix, code: gitCommitFailed}
	// failed to find a specific git commit
	//
	// Description: An error occurred while searching for a specific commit. The commit hash may be malformed or the search operation itself failed.
	//
	// Description_ZH: 搜索特定提交时发生错误。提交哈希可能格式错误，或者搜索操作本身失败。
	//
	// en-US: Failed to find commit
	//
	// zh-CN: 查找提交失败
	//
	// zh-HK: 查找提交失敗
	ErrGitFindCommitFailed error = CustomError{prefix: errGitPrefix, code: gitFindCommitFailed}
	// failed to count git commits
	//
	// Description: An error occurred while trying to count the number of commits in a branch or repository.
	//
	// Description_ZH: 尝试统计分支或仓库中的提交数量时发生错误。
	//
	// en-US: Failed to count commits
	//
	// zh-CN: 统计提交数量失败
	//
	// zh-HK: 統計提交數量失敗
	ErrGitCountCommitsFailed error = CustomError{prefix: errGitPrefix, code: gitCountCommitsFailed}
	// the specified git commit does not exist
	//
	// Description: The commit referenced by the provided hash or reference could not be found in the repository's history.
	//
	// Description_ZH: 在仓库的历史记录中找不到由所提供的哈希或引用指向的提交。
	//
	// en-US: Commit not found
	//
	// zh-CN: 未找到该提交
	//
	// zh-HK: 未找到該提交
	ErrGitCommitNotFound error = CustomError{prefix: errGitPrefix, code: gitCommitNotFound}
	// git diff operation failed
	//
	// Description: An error occurred while generating a diff between two commits, branches, or files.
	//
	// Description_ZH: 在生成两个提交、分支或文件之间的差异时发生错误。
	//
	// en-US: Failed to generate diff
	//
	// zh-CN: 生成差异对比失败
	//
	// zh-HK: 生成差異對比失敗
	ErrGitDiffFailed error = CustomError{prefix: errGitPrefix, code: gitDiffFailed}
	// git authentication failed
	//
	// Description: Authentication with the remote Git server failed. Please check your credentials (e.g., token, SSH key) and permissions.
	//
	// Description_ZH: 与远程 Git 服务器的身份验证失败。请检查您的凭据（例如，令牌、SSH 密钥）和权限。
	//
	// en-US: Git authentication failed
	//
	// zh-CN: Git身份验证失败
	//
	// zh-HK: Git身份驗證失敗
	ErrGitAuthFailed error = CustomError{prefix: errGitPrefix, code: gitAuthFailed}
	// git repository not found
	//
	// Description: The specified remote Git repository could not be found. Please verify the URL and ensure the repository exists and is accessible.
	//
	// Description_ZH: 找不到指定的远程 Git 仓库。请验证 URL，并确保仓库存在且可访问。
	//
	// en-US: Repository not found
	//
	// zh-CN: 仓库未找到
	//
	// zh-HK: 儲存庫未找到
	ErrGitRepoNotFound error = CustomError{prefix: errGitPrefix, code: gitRepoNotFound}
	// failed to find a specific git branch
	//
	// Description: An error occurred while searching for a specific branch. The branch name may be malformed or the search operation itself failed.
	//
	// Description_ZH: 搜索特定分支时发生错误。分支名称可能格式错误，或者搜索操作本身失败。
	//
	// en-US: Failed to find branch
	//
	// zh-CN: 查找分支失败
	//
	// zh-HK: 查找分支失敗
	ErrGitFindBranchFailed error = CustomError{prefix: errGitPrefix, code: gitFindBranchFailed}
	// the specified git branch does not exist
	//
	// Description: The specified branch name could not be found in the repository.
	//
	// Description_ZH: 在仓库中找不到指定的分支名称。
	//
	// en-US: Branch not found
	//
	// zh-CN: 未找到该分支
	//
	// zh-HK: 未找到該分支
	ErrGitBranchNotFound error = CustomError{prefix: errGitPrefix, code: gitBranchNotFound}
	// failed to delete a git branch
	//
	// Description: The attempt to delete a local or remote branch failed. This may be due to insufficient permissions or because the branch is protected.
	//
	// Description_ZH: 尝试删除本地或远程分支失败。这可能是由于权限不足或该分支受保护。
	//
	// en-US: Failed to delete branch
	//
	// zh-CN: 删除分支失败
	//
	// zh-HK: 刪除分支失敗
	ErrGitDeleteBranchFailed error = CustomError{prefix: errGitPrefix, code: gitDeleteBranchFailed}
	// file not found in the git repository at the specified path or commit
	//
	// Description: The requested file could not be found at the specified path within the given branch or commit of the Git repository.
	//
	// Description_ZH: 在 Git 仓库的指定分支或提交的指定路径下找不到所请求的文件。
	//
	// en-US: File not found in repository
	//
	// zh-CN: 在仓库中未找到该文件
	//
	// zh-HK: 在儲存庫中未找到該檔案
	ErrGitFileNotFound error = CustomError{prefix: errGitPrefix, code: gitFileNotFound}
	// file upload to the git repository failed
	//
	// Description: An error occurred while attempting to upload a file to the Git repository.
	//
	// Description_ZH: 尝试将文件上传到 Git 仓库时发生错误。
	//
	// en-US: File upload failed
	//
	// zh-CN: 文件上传失败
	//
	// zh-HK: 檔案上傳失敗
	ErrGitUploadFailed error = CustomError{prefix: errGitPrefix, code: gitUploadFailed}
	// file download from the git repository failed
	//
	// Description: An error occurred while attempting to download a file from the Git repository. Check file path, permissions, and network connectivity.
	//
	// Description_ZH: 尝试从 Git 仓库下载文件时发生错误。请检查文件路径、权限和网络连接。
	//
	// en-US: File download failed
	//
	// zh-CN: 文件下载失败
	//
	// zh-HK: 檔案下載失敗
	ErrGitDownloadFailed error = CustomError{prefix: errGitPrefix, code: gitDownloadFailed}
	// failed to connect to the git remote server
	//
	// Description: A connection to the remote Git server could not be established. Please check your network connection, firewall settings, and the remote server's status.
	//
	// Description_ZH: 无法建立到远程 Git 服务器的连接。请检查您的网络连接、防火墙设置以及远程服务器的状态。
	//
	// en-US: Failed to connect to Git server
	//
	// zh-CN: 连接Git服务器失败
	//
	// zh-HK: 連接Git伺服器失敗
	ErrGitConnectionFailed error = CustomError{prefix: errGitPrefix, code: gitConnectionFailed}
	// an error occurred with git-lfs
	//
	// Description: An unspecified error occurred during a Git LFS (Large File Storage) operation. Check LFS configuration and logs for more details.
	//
	// Description_ZH: 在 Git LFS（大文件存储）操作期间发生未指定的错误。请检查 LFS 配置和日志以获取更多详细信息。
	//
	// en-US: Git LFS operation failed
	//
	// zh-CN: Git LFS操作失败
	//
	// zh-HK: Git LFS操作失敗
	ErrGitLfsError error = CustomError{prefix: errGitPrefix, code: gitLfsError}
	// the file is too large to be processed or uploaded
	//
	// Description: The file exceeds the configured maximum size limit for this operation. Consider using Git LFS for large files.
	//
	// Description_ZH: 文件大小超出了此操作配置的最大限制。请考虑对大文件使用 Git LFS。
	//
	// en-US: File is too large
	//
	// zh-CN: 文件过大
	//
	// zh-HK: 檔案過大
	ErrFileTooLarge error = CustomError{prefix: errGitPrefix, code: fileTooLarge} // Custom error for file size limit exceeded
	// the git service is currently unavailable
	//
	// Description: The Git hosting service is temporarily unavailable or unreachable. Please try again later.
	//
	// Description_ZH: Git 托管服务暂时不可用或无法访问。请稍后再试。
	//
	// en-US: Git service is unavailable
	//
	// zh-CN: Git服务不可用
	//
	// zh-HK: Git服務不可用
	ErrServiceUnavaliable error = CustomError{prefix: errGitPrefix, code: gitServiceUnavaliable}

	// get git tree entry failed
	//
	// Description: Get git tree entry failed. This can be caused by network problems, authentication issues, or the specified tree entry does not exist.
	//
	// Description_ZH: 获取 git tree entry 失败。这可能由网络问题、身份验证问题或指定的 tree entry 不存在引起。
	//
	// en-US: Get git tree entry failed
	//
	// zh-CN: 获取 git tree entry 失败
	//
	// zh-HK: 獲取 git tree entry 失敗
	ErrGetTreeEntryFailed error = CustomError{prefix: errGitPrefix, code: gitGetTreeEntryFailed}

	// commit git files failed
	//
	// Description: Commit git files failed. This can be caused by network problems, authentication issues, or the specified files do not exist.
	//
	// Description_ZH: 提交 git 文件失败。这可能由网络问题、身份验证问题或指定的文件不存在引起。
	//
	// en-US: Commit git files failed
	//
	// zh-CN: 提交 git 文件失败
	//
	// zh-HK: 提交 git 文件失敗
	ErrCommitFilesFailed error = CustomError{prefix: errGitPrefix, code: gitCommitFilesFailed}
	// get git blobs failed
	//
	// Description: Get git blobs failed. This can be caused by network problems, authentication issues, or the specified blobs do not exist.
	//
	// Description_ZH: 获取 git blobs 失败。这可能由网络问题、身份验证问题或指定的 blobs 不存在引起。
	//
	// en-US: Get git blobs failed
	//
	// zh-CN: 获取 git blobs 失败
	//
	// zh-HK: 獲取 git blobs 失敗
	ErrGetBlobsFailed error = CustomError{prefix: errGitPrefix, code: gitGetBlobsFailed}
	// get git lfs pointers failed
	//
	// Description: Get git lfs pointers failed. This can be caused by network problems, authentication issues, or the specified lfs pointers do not exist.
	//
	// Description_ZH: 获取 git lfs pointers 失败。这可能由网络问题、身份验证问题或指定的 lfs pointers 不存在引起。
	//
	// en-US: Get git lfs pointers failed
	//
	// zh-CN: 获取 git lfs pointers 失败
	//
	// zh-HK: 獲取 git lfs pointers 失敗
	ErrGetLfsPointersFailed error = CustomError{prefix: errGitPrefix, code: gitGetLfsPointersFailed}
	// get git tree last commit failed
	//
	// Description: Get git tree last commit failed. This can be caused by network problems, authentication issues, or the specified tree does not exist.
	//
	// Description_ZH: 获取 git tree 最后一次提交失败。这可能由网络问题、身份验证问题或指定的 tree 不存在引起。
	//
	// en-US: Get git tree last commit failed
	//
	// zh-CN: 获取 git tree 最后一次提交失败
	//
	// zh-HK: 獲取 git tree 最後一次提交失敗
	ErrListLastCommitsForTreeFailed error = CustomError{prefix: errGitPrefix, code: gitListLastCommitsForTreeFailed}
	// get git blob info failed
	//
	// Description: Get git blob info failed. This can be caused by network problems, authentication issues, or the specified blob does not exist.
	//
	// Description_ZH: 获取 git blob 信息失败。这可能由网络问题、身份验证问题或指定的 blob 不存在引起。
	//
	// en-US: Get git blob info failed
	//
	// zh-CN: 获取 git blob 信息失败
	//
	// zh-HK: 獲取 git blob 信息失敗
	ErrGetBlobInfoFailed error = CustomError{prefix: errGitPrefix, code: gitGetBlobInfoFailed}
	// get git files failed
	//
	// Description: Get git files failed. This can be caused by network problems, authentication issues, or the specified files do not exist.
	//
	// Description_ZH: 获取 git 文件失败。这可能由网络问题、身份验证问题或指定的文件不存在引起。
	//
	// en-US: Get git files failed
	//
	// zh-CN: 获取 git 文件失败
	//
	// zh-HK: 獲取 git 文件失敗
	ErrListFilesFailed error = CustomError{prefix: errGitPrefix, code: gitListFilesFailed}
	// create mirror failed
	//
	// Description: Create mirror failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.
	//
	// Description_ZH: 创建镜像失败。这可能由网络问题、身份验证问题或指定的仓库不存在引起。
	//
	// en-US: Create mirror failed
	//
	// zh-CN: 创建镜像失败
	//
	// zh-HK: 建立鏡像失敗
	ErrCreateMirrorFailed error = CustomError{prefix: errGitPrefix, code: gitCreateMirrorFailed}
	// sync mirror failed
	//
	// Description: Sync mirror failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.
	//
	// Description_ZH: 同步镜像失败。这可能由网络问题、身份验证问题或指定的仓库不存在引起。
	//
	// en-US: Sync mirror failed
	//
	// zh-CN: 同步镜像失败
	//
	// zh-HK: 同步鏡像失敗
	ErrMirrorSyncFailed error = CustomError{prefix: errGitPrefix, code: gitMirrorSyncFailed}
	// check repository exists failed
	//
	// Description: Check repository exists failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.
	//
	// Description_ZH: 检查仓库是否存在失败。这可能由网络问题、身份验证问题或指定的仓库不存在引起。
	//
	// en-US: Check repository exists failed
	//
	// zh-CN: 检查仓库是否存在失败
	//
	// zh-HK: 檢查倉庫是否存在失敗
	ErrCheckRepositoryExistsFailed error = CustomError{prefix: errGitPrefix, code: gitCheckRepositoryExistsFailed}
	// create repository failed
	//
	// Description: Create repository failed. This can be caused by network problems, authentication issues.
	//
	// Description_ZH: 创建仓库失败。这可能由网络问题、身份验证问题引起。
	//
	// en-US: Create repository failed
	//
	// zh-CN: 创建仓库失败
	//
	// zh-HK: 創建倉庫失敗
	ErrCreateRepositoryFailed error = CustomError{prefix: errGitPrefix, code: gitCreateRepositoryFailed}
	//  delete repository failed
	//
	// Description: delete repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.
	//
	// Description_ZH: 删除仓库失败。这可能由网络问题、身份验证问题或指定的仓库不存在引起。
	//
	// en-US: delete repository failed
	//
	// zh-CN: 删除仓库失败
	//
	// zh-HK: 刪除倉庫失敗
	ErrDeleteRepositoryFailed error = CustomError{prefix: errGitPrefix, code: gitDeleteRepositoryFailed}
	// get repository failed
	//
	// Description: get repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.
	//
	// Description_ZH: 获取仓库失败。这可能由网络问题、身份验证问题或指定的仓库不存在引起。
	//
	// en-US: get repository failed
	//
	// zh-CN: 获取仓库失败
	//
	// zh-HK: 取得倉庫失敗
	ErrGetRepositoryFailed error = CustomError{prefix: errGitPrefix, code: gitGetRepositoryFailed}
	// copy repository failed
	//
	// Description: copy repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.
	//
	// Description_ZH: 复制仓库失败。这可能由网络问题、身份验证问题或指定的仓库不存在引起。
	//
	// en-US: copy repository failed
	//
	// zh-CN: 复制仓库失败
	//
	// zh-HK: 複製倉庫失敗
	ErrCopyRepositoryFailed error = CustomError{prefix: errGitPrefix, code: gitCopyRepositoryFailed}
	// replicate repository failed
	//
	// Description: replicate repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.
	//
	// Description_ZH: 转移仓库失败。这可能由网络问题、身份验证问题或指定的仓库不存在引起。
	//
	// en-US: replicate repository failed
	//
	// zh-CN: 转移仓库失败
	//
	// zh-HK: 转移倉庫失敗
	ErrGitReplicateRepositoryFailed error = CustomError{prefix: errGitPrefix, code: gitReplicateRepositoryFailed}
	// --- GIT-ERR-xxx: Git/Upload, Download, Resource Synchronization ---
	// using git in xnet-enabled repository error
	//
	// Description: Using git in xnet-enabled repository error. Git operations are not supported in repositories enabled with xnet.
	//
	// Description_ZH: 使用 git 操作 xnet 启用的仓库。
	//
	// en-US: Using git in xnet-enabled repository error
	//
	// zh-CN: 在 xnet 启用的仓库中使用 git 失败
	//
	// zh-HK: 在 xnet 啟用的倉庫中使用 git 失敗
	ErrUsingGitInXnetRepository error = CustomError{prefix: errGitPrefix, code: gitUsingGitInXnetRepository}
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

func CreateBranchFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCreateBranchFailed,
		err:     err,
		context: ctx,
	}
}

func SetDefaultBranchFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitSetDefaultBranchFailed,
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

func ErrGitCopyRepositoryFailed(err error, ctx context) error {
	return CustomError{
		prefix:  errGitPrefix,
		code:    gitCopyRepositoryFailed,
		err:     err,
		context: ctx,
	}
}
