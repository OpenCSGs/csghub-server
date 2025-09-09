# 错误代码

本文档列出了项目中定义的所有自定义错误码，按模块分类。

## Account 错误

### `ACT-ERR-0`

- **错误代码:** `ACT-ERR-0`
- **错误名:** `insufficientBalance`
- **描述:** 用户账户余额不足，无法完成所请求的交易或操作。

---

### `ACT-ERR-1`

- **错误代码:** `ACT-ERR-1`
- **错误名:** `subscriptionExist`
- **描述:** 用户试图订阅一个他们已经拥有有效订阅的服务。

---

### `ACT-ERR-2`

- **错误代码:** `ACT-ERR-2`
- **错误名:** `invalidUnitType`
- **描述:** 请求中指定的单位类型（例如，用于计费的单位）不被系统识别或支持。

---

### `ACT-ERR-3`

- **错误代码:** `ACT-ERR-3`
- **错误名:** `wrongTimeRange`
- **描述:** 指定的时间范围无效，例如开始时间晚于结束时间。

## Auth 错误

### `AUTH-ERR-0`

- **错误代码:** `AUTH-ERR-0`
- **错误名:** `unauthorized`
- **描述:** 用户没有登录，请登录后访问资源

---

### `AUTH-ERR-1`

- **错误代码:** `AUTH-ERR-1`
- **错误名:** `userNotFound`
- **描述:** 找不到指定的用户帐户。

---

### `AUTH-ERR-2`

- **错误代码:** `AUTH-ERR-2`
- **错误名:** `forbidden`
- **描述:** 当前用户没有足够的权限来执行此操作。

---

### `AUTH-ERR-3`

- **错误代码:** `AUTH-ERR-3`
- **错误名:** `noEmail`
- **描述:** 用户的帐户没有关联的电子邮件地址，而此操作需要该地址。

---

### `AUTH-ERR-4`

- **错误代码:** `AUTH-ERR-4`
- **错误名:** `invalidJWT`
- **描述:** 身份验证令牌（JWT）格式错误、无效或已过期。请重新登录。

---

### `AUTH-ERR-5`

- **错误代码:** `AUTH-ERR-5`
- **错误名:** `invalidAuthHeader`
- **描述:** Authorization请求头缺失或格式不正确。通常应为 'Bearer `{token}`' 格式。

---

### `AUTH-ERR-6`

- **错误代码:** `AUTH-ERR-6`
- **错误名:** `notAdmin`
- **描述:** 此操作需要管理员权限，但当前用户不是管理员。

---

### `AUTH-ERR-7`

- **错误代码:** `AUTH-ERR-7`
- **错误名:** `userNotMatch`
- **描述:** 您只能在自己的账户上执行此操作。

---

### `AUTH-ERR-8`

- **错误代码:** `AUTH-ERR-8`
- **错误名:** `needUUID`
- **描述:** 请求必须在请求头或正文中包含用户的UUID以识别目标账户。

---

### `AUTH-ERR-9`

- **错误代码:** `AUTH-ERR-9`
- **错误名:** `needAPIKey`
- **描述:** 请求必须在请求头或正文中包含API密钥以进行身份验证。

## Dataset 错误

### `DAT-ERR-0`

- **错误代码:** `DAT-ERR-0`
- **错误名:** `dataviewerCardNotFound`
- **描述:** 在系统或指定的数据集中找不到所请求的数据可视化卡片。

---

### `DAT-ERR-1`

- **错误代码:** `DAT-ERR-1`
- **错误名:** `datasetBadFormat`
- **描述:** 上传或指定的数据集格式无效或不符合预期。请检查文件结构和数据类型。

---

### `DAT-ERR-2`

- **错误代码:** `DAT-ERR-2`
- **错误名:** `noValidParquetFile`
- **描述:** 数据集中不包含任何有效的Parquet文件，而此操作需要该文件格式。

## Git 错误

### `GIT-ERR-0`

- **错误代码:** `GIT-ERR-0`
- **错误名:** `gitCloneFailed`
- **描述:** 尝试将远程 Git 仓库克隆到本地系统失败。这可能是由于网络问题、不正确的仓库 URL 或权限不足造成的。

---

### `GIT-ERR-1`

- **错误代码:** `GIT-ERR-1`
- **错误名:** `gitPullFailed`
- **描述:** 从另一个仓库或本地分支获取并集成失败。这可能是由合并冲突、网络问题或身份验证问题引起的。

---

### `GIT-ERR-2`

- **错误代码:** `GIT-ERR-2`
- **错误名:** `gitPushFailed`
- **描述:** 更新远程引用及其关联对象失败。如果远程分支有新的提交，或者推送权限不足，可能会发生这种情况。

---

### `GIT-ERR-3`

- **错误代码:** `GIT-ERR-3`
- **错误名:** `gitCommitFailed`
- **描述:** The attempt to record changes to the repository failed. This could be due to an empty staging area, a pre-commit hook failure, or incorrect user configuration.

---

### `GIT-ERR-4`

- **错误代码:** `GIT-ERR-4`
- **错误名:** `gitFindCommitFailed`
- **描述:** 搜索特定提交时发生错误。提交哈希可能格式错误，或者搜索操作本身失败。

---

### `GIT-ERR-5`

- **错误代码:** `GIT-ERR-5`
- **错误名:** `gitCountCommitsFailed`
- **描述:** 尝试统计分支或仓库中的提交数量时发生错误。

---

### `GIT-ERR-6`

- **错误代码:** `GIT-ERR-6`
- **错误名:** `gitCommitNotFound`
- **描述:** 在仓库的历史记录中找不到由所提供的哈希或引用指向的提交。

---

### `GIT-ERR-7`

- **错误代码:** `GIT-ERR-7`
- **错误名:** `gitDiffFailed`
- **描述:** 在生成两个提交、分支或文件之间的差异时发生错误。

---

### `GIT-ERR-8`

- **错误代码:** `GIT-ERR-8`
- **错误名:** `gitAuthFailed`
- **描述:** 与远程 Git 服务器的身份验证失败。请检查您的凭据（例如，令牌、SSH 密钥）和权限。

---

### `GIT-ERR-9`

- **错误代码:** `GIT-ERR-9`
- **错误名:** `gitRepoNotFound`
- **描述:** 找不到指定的远程 Git 仓库。请验证 URL，并确保仓库存在且可访问。

---

### `GIT-ERR-10`

- **错误代码:** `GIT-ERR-10`
- **错误名:** `gitFindBranchFailed`
- **描述:** 搜索特定分支时发生错误。分支名称可能格式错误，或者搜索操作本身失败。

---

### `GIT-ERR-11`

- **错误代码:** `GIT-ERR-11`
- **错误名:** `gitBranchNotFound`
- **描述:** 在仓库中找不到指定的分支名称。

---

### `GIT-ERR-12`

- **错误代码:** `GIT-ERR-12`
- **错误名:** `gitDeleteBranchFailed`
- **描述:** 尝试删除本地或远程分支失败。这可能是由于权限不足或该分支受保护。

---

### `GIT-ERR-13`

- **错误代码:** `GIT-ERR-13`
- **错误名:** `gitFileNotFound`
- **描述:** 在 Git 仓库的指定分支或提交的指定路径下找不到所请求的文件。

---

### `GIT-ERR-14`

- **错误代码:** `GIT-ERR-14`
- **错误名:** `gitUploadFailed`
- **描述:** 尝试将文件上传到 Git 仓库时发生错误。

---

### `GIT-ERR-15`

- **错误代码:** `GIT-ERR-15`
- **错误名:** `gitDownloadFailed`
- **描述:** 尝试从 Git 仓库下载文件时发生错误。请检查文件路径、权限和网络连接。

---

### `GIT-ERR-16`

- **错误代码:** `GIT-ERR-16`
- **错误名:** `gitConnectionFailed`
- **描述:** 无法建立到远程 Git 服务器的连接。请检查您的网络连接、防火墙设置以及远程服务器的状态。

---

### `GIT-ERR-17`

- **错误代码:** `GIT-ERR-17`
- **错误名:** `gitLfsError`
- **描述:** 在 Git LFS（大文件存储）操作期间发生未指定的错误。请检查 LFS 配置和日志以获取更多详细信息。

---

### `GIT-ERR-18`

- **错误代码:** `GIT-ERR-18`
- **错误名:** `fileTooLarge`
- **描述:** 文件大小超出了此操作配置的最大限制。请考虑对大文件使用 Git LFS。

---

### `GIT-ERR-19`

- **错误代码:** `GIT-ERR-19`
- **错误名:** `gitGetTreeEntryFailed`
- **描述:** 

---

### `GIT-ERR-20`

- **错误代码:** `GIT-ERR-20`
- **错误名:** `gitCommitFilesFailed`
- **描述:** 

---

### `GIT-ERR-21`

- **错误代码:** `GIT-ERR-21`
- **错误名:** `gitGetBlobsFailed`
- **描述:** 

---

### `GIT-ERR-22`

- **错误代码:** `GIT-ERR-22`
- **错误名:** `gitGetLfsPointersFailed`
- **描述:** 

---

### `GIT-ERR-23`

- **错误代码:** `GIT-ERR-23`
- **错误名:** `gitListLastCommitsForTreeFailed`
- **描述:** 

---

### `GIT-ERR-24`

- **错误代码:** `GIT-ERR-24`
- **错误名:** `gitGetBlobInfoFailed`
- **描述:** 

---

### `GIT-ERR-25`

- **错误代码:** `GIT-ERR-25`
- **错误名:** `gitListFilesFailed`
- **描述:** 

---

### `GIT-ERR-26`

- **错误代码:** `GIT-ERR-26`
- **错误名:** `gitCreateMirrorFailed`
- **描述:** 

---

### `GIT-ERR-27`

- **错误代码:** `GIT-ERR-27`
- **错误名:** `gitMirrorSyncFailed`
- **描述:** 

---

### `GIT-ERR-28`

- **错误代码:** `GIT-ERR-28`
- **错误名:** `gitCheckRepositoryExistsFailed`
- **描述:** 

---

### `GIT-ERR-29`

- **错误代码:** `GIT-ERR-29`
- **错误名:** `gitCreateRepositoryFailed`
- **描述:** 

---

### `GIT-ERR-30`

- **错误代码:** `GIT-ERR-30`
- **错误名:** `gitDeleteRepositoryFailed`
- **描述:** 

---

### `GIT-ERR-31`

- **错误代码:** `GIT-ERR-31`
- **错误名:** `gitGetRepositoryFailed`
- **描述:** 

---

### `GIT-ERR-32`

- **错误代码:** `GIT-ERR-32`
- **错误名:** `gitServiceUnavaliable`
- **描述:** Git 托管服务暂时不可用或无法访问。请稍后再试。

## Req 错误

### `REQ-ERR-0`

- **错误代码:** `REQ-ERR-0`
- **错误名:** `errBadRequest`
- **描述:** 由于语法格式错误或无效的请求消息，服务器无法理解该请求。

---

### `REQ-ERR-1`

- **错误代码:** `REQ-ERR-1`
- **错误名:** `errReqBodyFormat`
- **描述:** 请求正文的格式无效或无法解析。例如，提供的JSON格式不正确。

---

### `REQ-ERR-2`

- **错误代码:** `REQ-ERR-2`
- **错误名:** `errReqBodyEmpty`
- **描述:** 请求正文为空，但此接口需要非空的正文才能继续操作。

---

### `REQ-ERR-3`

- **错误代码:** `REQ-ERR-3`
- **错误名:** `errReqBodyTooLarge`
- **描述:** 请求正文的大小超过了服务器为此接口配置的限制。

---

### `REQ-ERR-4`

- **错误代码:** `REQ-ERR-4`
- **错误名:** `errReqParamMissing`
- **描述:** 

---

### `REQ-ERR-5`

- **错误代码:** `REQ-ERR-5`
- **错误名:** `errReqParamDuplicate`
- **描述:** 请求中多次提供了同一个参数，而此接口不允许这样做。

---

### `REQ-ERR-6`

- **错误代码:** `REQ-ERR-6`
- **错误名:** `errReqParamInvalid`
- **描述:** 请求参数无效。它可能是错误的类型、超出允许范围或是不允许的值。

---

### `REQ-ERR-7`

- **错误代码:** `REQ-ERR-7`
- **错误名:** `errReqParamOutOfRange`
- **描述:** 

---

### `REQ-ERR-8`

- **错误代码:** `REQ-ERR-8`
- **错误名:** `errReqParamTypeError`
- **描述:** 

---

### `REQ-ERR-9`

- **错误代码:** `REQ-ERR-9`
- **错误名:** `errReqContentTypeUnsupported`
- **描述:** 此接口不支持请求的'Content-Type'。请查阅API文档以了解允许的内容类型。

## System 错误

### `SYS-ERR-0`

- **错误代码:** `SYS-ERR-0`
- **错误名:** `internalServerError`
- **描述:** 服务器上遇到了意外情况，导致无法完成请求。这是一个用于捕获未处理异常的通用错误，例如序列化错误或类型转换失败。

---

### `SYS-ERR-1`

- **错误代码:** `SYS-ERR-1`
- **错误名:** `remoteServiceFail`
- **描述:** 对下游依赖或外部服务的请求失败。这是一个通用错误，应在调用组件中转换为更具体的错误。

---

### `SYS-ERR-2`

- **错误代码:** `SYS-ERR-2`
- **错误名:** `databaseFailure`
- **描述:** 在数据库操作期间发生了未处理或意外的错误，例如连接丢失（`sql.ErrConnDone`）。

---

### `SYS-ERR-3`

- **错误代码:** `SYS-ERR-3`
- **错误名:** `databaseNoRows`
- **描述:** 期望至少返回一行的数据库查询没有找到匹配的记录。这是 `sql.ErrNoRows` 的系统级封装。

---

### `SYS-ERR-4`

- **错误代码:** `SYS-ERR-4`
- **错误名:** `databaseDuplicateKey`
- **描述:** `INSERT` 或 `UPDATE` 操作失败，因为它会在具有唯一约束的列中创建重复值。

---

### `SYS-ERR-5`

- **错误代码:** `SYS-ERR-5`
- **错误名:** `lfsNotFound`
- **描述:** 系统无法找到或连接到配置的LFS（大文件存储）服务。这表明存在系统配置问题。

---

### `SYS-ERR-6`

- **错误代码:** `SYS-ERR-6`
- **错误名:** `lastOrgAdmin`
- **描述:** 禁止移除用户管理员角色的请求，因为他们是组织的唯一管理员。此举可防止组织被锁定而无法管理。

---

### `SYS-ERR-7`

- **错误代码:** `SYS-ERR-7`
- **错误名:** `cannotPromoteSelfToAdmin`
- **描述:** 禁止将自身提升为管理员。

## Task 错误

### `TASK-ERR-0`

- **错误代码:** `TASK-ERR-0`
- **错误名:** `noEntryFile`
- **描述:** 该任务需要一个特定的入口文件来开始执行（例如 'main.py' 或 'app.js'），但在指定的源目录中找不到这样的文件。

---

### `TASK-ERR-1`

- **错误代码:** `TASK-ERR-1`
- **错误名:** `multiHostInferenceNotSupported`
- **描述:** 多主机推理功能目前仅支持 VLLM 和 SGLang 框架，其他框架暂不支持此功能。

---

### `TASK-ERR-2`

- **错误代码:** `TASK-ERR-2`
- **错误名:** `multiHostInferenceReplicaCount`
- **描述:** 在配置多主机推理时，最小副本数必须大于零以确保服务正常运行。

