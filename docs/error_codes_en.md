# Error Codes

This document lists all the custom error codes defined in the project, categorized by module.

## Account Errors

### `ACT-ERR-0`

- **Error Code:** `ACT-ERR-0`
- **Error Name:** `insufficientBalance`
- **Description:** The user's account balance is insufficient to complete the requested transaction or operation.

---

### `ACT-ERR-1`

- **Error Code:** `ACT-ERR-1`
- **Error Name:** `subscriptionExist`
- **Description:** The user is attempting to subscribe to a service for which they already have an active subscription.

---

### `ACT-ERR-2`

- **Error Code:** `ACT-ERR-2`
- **Error Name:** `invalidUnitType`
- **Description:** The unit type specified in the request (e.g., for billing) is not recognized or supported.

---

### `ACT-ERR-3`

- **Error Code:** `ACT-ERR-3`
- **Error Name:** `wrongTimeRange`
- **Description:** The specified time range is invalid, for example, the start time is after the end time.

## Auth Errors

### `AUTH-ERR-0`

- **Error Code:** `AUTH-ERR-0`
- **Error Name:** `unauthorized`
- **Description:** The user is not logged in. Please log in to access this resource.

---

### `AUTH-ERR-1`

- **Error Code:** `AUTH-ERR-1`
- **Error Name:** `userNotFound`
- **Description:** The user account specified could not be found.

---

### `AUTH-ERR-2`

- **Error Code:** `AUTH-ERR-2`
- **Error Name:** `forbidden`
- **Description:** The current user does not have sufficient permissions to perform this action.

---

### `AUTH-ERR-3`

- **Error Code:** `AUTH-ERR-3`
- **Error Name:** `noEmail`
- **Description:** The user's account does not have an associated email address, which is required for this operation.

---

### `AUTH-ERR-4`

- **Error Code:** `AUTH-ERR-4`
- **Error Name:** `invalidJWT`
- **Description:** The authentication token (JWT) is malformed, invalid, or has expired. Please log in again.

---

### `AUTH-ERR-5`

- **Error Code:** `AUTH-ERR-5`
- **Error Name:** `invalidAuthHeader`
- **Description:** The Authorization header is missing or incorrectly formatted. It should typically be in the format 'Bearer `{token}`'.

---

### `AUTH-ERR-6`

- **Error Code:** `AUTH-ERR-6`
- **Error Name:** `notAdmin`
- **Description:** This operation requires administrator privileges, but the current user is not an administrator.

---

### `AUTH-ERR-7`

- **Error Code:** `AUTH-ERR-7`
- **Error Name:** `userNotMatch`
- **Description:** You can only perform this action on your own account.

---

### `AUTH-ERR-8`

- **Error Code:** `AUTH-ERR-8`
- **Error Name:** `needUUID`
- **Description:** The request must include the user's UUID in the header or body to identify the target account.

---

### `AUTH-ERR-9`

- **Error Code:** `AUTH-ERR-9`
- **Error Name:** `needAPIKey`
- **Description:** The request must include an API Key in the header or body for authentication.

## Dataset Errors

### `DAT-ERR-0`

- **Error Code:** `DAT-ERR-0`
- **Error Name:** `dataviewerCardNotFound`
- **Description:** The requested dataviewer card could not be located within the system or the specified dataset.

---

### `DAT-ERR-1`

- **Error Code:** `DAT-ERR-1`
- **Error Name:** `datasetBadFormat`
- **Description:** The uploaded or specified dataset is not in a valid or expected format. Please check the file structure and data types.

---

### `DAT-ERR-2`

- **Error Code:** `DAT-ERR-2`
- **Error Name:** `noValidParquetFile`
- **Description:** The dataset does not contain any valid Parquet files, which are required for this operation.

## Git Errors

### `GIT-ERR-0`

- **Error Code:** `GIT-ERR-0`
- **Error Name:** `gitCloneFailed`
- **Description:** The attempt to clone a remote Git repository to the local system failed. This could be due to network issues, incorrect repository URL, or insufficient permissions.

---

### `GIT-ERR-1`

- **Error Code:** `GIT-ERR-1`
- **Error Name:** `gitPullFailed`
- **Description:** Failed to fetch from and integrate with another repository or a local branch. This can be caused by merge conflicts, network problems, or authentication issues.

---

### `GIT-ERR-2`

- **Error Code:** `GIT-ERR-2`
- **Error Name:** `gitPushFailed`
- **Description:** Failed to update remote refs along with associated objects. This might happen if the remote branch has new commits, or due to insufficient push permissions.

---

### `GIT-ERR-3`

- **Error Code:** `GIT-ERR-3`
- **Error Name:** `gitCommitFailed`
- **Description:** The attempt to record changes to the repository failed. This could be due to an empty staging area, a pre-commit hook failure, or incorrect user configuration.

---

### `GIT-ERR-4`

- **Error Code:** `GIT-ERR-4`
- **Error Name:** `gitFindCommitFailed`
- **Description:** An error occurred while searching for a specific commit. The commit hash may be malformed or the search operation itself failed.

---

### `GIT-ERR-5`

- **Error Code:** `GIT-ERR-5`
- **Error Name:** `gitCountCommitsFailed`
- **Description:** An error occurred while trying to count the number of commits in a branch or repository.

---

### `GIT-ERR-6`

- **Error Code:** `GIT-ERR-6`
- **Error Name:** `gitCommitNotFound`
- **Description:** The commit referenced by the provided hash or reference could not be found in the repository's history.

---

### `GIT-ERR-7`

- **Error Code:** `GIT-ERR-7`
- **Error Name:** `gitDiffFailed`
- **Description:** An error occurred while generating a diff between two commits, branches, or files.

---

### `GIT-ERR-8`

- **Error Code:** `GIT-ERR-8`
- **Error Name:** `gitAuthFailed`
- **Description:** Authentication with the remote Git server failed. Please check your credentials (e.g., token, SSH key) and permissions.

---

### `GIT-ERR-9`

- **Error Code:** `GIT-ERR-9`
- **Error Name:** `gitRepoNotFound`
- **Description:** The specified remote Git repository could not be found. Please verify the URL and ensure the repository exists and is accessible.

---

### `GIT-ERR-10`

- **Error Code:** `GIT-ERR-10`
- **Error Name:** `gitFindBranchFailed`
- **Description:** An error occurred while searching for a specific branch. The branch name may be malformed or the search operation itself failed.

---

### `GIT-ERR-11`

- **Error Code:** `GIT-ERR-11`
- **Error Name:** `gitBranchNotFound`
- **Description:** The specified branch name could not be found in the repository.

---

### `GIT-ERR-12`

- **Error Code:** `GIT-ERR-12`
- **Error Name:** `gitDeleteBranchFailed`
- **Description:** The attempt to delete a local or remote branch failed. This may be due to insufficient permissions or because the branch is protected.

---

### `GIT-ERR-13`

- **Error Code:** `GIT-ERR-13`
- **Error Name:** `gitFileNotFound`
- **Description:** The requested file could not be found at the specified path within the given branch or commit of the Git repository.

---

### `GIT-ERR-14`

- **Error Code:** `GIT-ERR-14`
- **Error Name:** `gitUploadFailed`
- **Description:** An error occurred while attempting to upload a file to the Git repository.

---

### `GIT-ERR-15`

- **Error Code:** `GIT-ERR-15`
- **Error Name:** `gitDownloadFailed`
- **Description:** An error occurred while attempting to download a file from the Git repository. Check file path, permissions, and network connectivity.

---

### `GIT-ERR-16`

- **Error Code:** `GIT-ERR-16`
- **Error Name:** `gitConnectionFailed`
- **Description:** A connection to the remote Git server could not be established. Please check your network connection, firewall settings, and the remote server's status.

---

### `GIT-ERR-17`

- **Error Code:** `GIT-ERR-17`
- **Error Name:** `gitLfsError`
- **Description:** An unspecified error occurred during a Git LFS (Large File Storage) operation. Check LFS configuration and logs for more details.

---

### `GIT-ERR-18`

- **Error Code:** `GIT-ERR-18`
- **Error Name:** `fileTooLarge`
- **Description:** The file exceeds the configured maximum size limit for this operation. Consider using Git LFS for large files.

---

### `GIT-ERR-19`

- **Error Code:** `GIT-ERR-19`
- **Error Name:** `gitGetTreeEntryFailed`
- **Description:** 

---

### `GIT-ERR-20`

- **Error Code:** `GIT-ERR-20`
- **Error Name:** `gitCommitFilesFailed`
- **Description:** 

---

### `GIT-ERR-21`

- **Error Code:** `GIT-ERR-21`
- **Error Name:** `gitGetBlobsFailed`
- **Description:** 

---

### `GIT-ERR-22`

- **Error Code:** `GIT-ERR-22`
- **Error Name:** `gitGetLfsPointersFailed`
- **Description:** 

---

### `GIT-ERR-23`

- **Error Code:** `GIT-ERR-23`
- **Error Name:** `gitListLastCommitsForTreeFailed`
- **Description:** 

---

### `GIT-ERR-24`

- **Error Code:** `GIT-ERR-24`
- **Error Name:** `gitGetBlobInfoFailed`
- **Description:** 

---

### `GIT-ERR-25`

- **Error Code:** `GIT-ERR-25`
- **Error Name:** `gitListFilesFailed`
- **Description:** 

---

### `GIT-ERR-26`

- **Error Code:** `GIT-ERR-26`
- **Error Name:** `gitCreateMirrorFailed`
- **Description:** 

---

### `GIT-ERR-27`

- **Error Code:** `GIT-ERR-27`
- **Error Name:** `gitMirrorSyncFailed`
- **Description:** 

---

### `GIT-ERR-28`

- **Error Code:** `GIT-ERR-28`
- **Error Name:** `gitCheckRepositoryExistsFailed`
- **Description:** 

---

### `GIT-ERR-29`

- **Error Code:** `GIT-ERR-29`
- **Error Name:** `gitCreateRepositoryFailed`
- **Description:** 

---

### `GIT-ERR-30`

- **Error Code:** `GIT-ERR-30`
- **Error Name:** `gitDeleteRepositoryFailed`
- **Description:** 

---

### `GIT-ERR-31`

- **Error Code:** `GIT-ERR-31`
- **Error Name:** `gitGetRepositoryFailed`
- **Description:** 

---

### `GIT-ERR-32`

- **Error Code:** `GIT-ERR-32`
- **Error Name:** `gitServiceUnavaliable`
- **Description:** The Git hosting service is temporarily unavailable or unreachable. Please try again later.

## Req Errors

### `REQ-ERR-0`

- **Error Code:** `REQ-ERR-0`
- **Error Name:** `errBadRequest`
- **Description:** The server could not understand the request due to malformed syntax or invalid request message framing.

---

### `REQ-ERR-1`

- **Error Code:** `REQ-ERR-1`
- **Error Name:** `errReqBodyFormat`
- **Description:** The format of the request body is invalid or cannot be parsed. For example, the provided JSON is malformed.

---

### `REQ-ERR-2`

- **Error Code:** `REQ-ERR-2`
- **Error Name:** `errReqBodyEmpty`
- **Description:** The request body is empty, but this endpoint requires a non-empty body to proceed.

---

### `REQ-ERR-3`

- **Error Code:** `REQ-ERR-3`
- **Error Name:** `errReqBodyTooLarge`
- **Description:** The size of the request body exceeds the server's configured limit for this endpoint.

---

### `REQ-ERR-4`

- **Error Code:** `REQ-ERR-4`
- **Error Name:** `errReqParamMissing`
- **Description:** 

---

### `REQ-ERR-5`

- **Error Code:** `REQ-ERR-5`
- **Error Name:** `errReqParamDuplicate`
- **Description:** A parameter was provided more than once in the request, which is not allowed for this endpoint.

---

### `REQ-ERR-6`

- **Error Code:** `REQ-ERR-6`
- **Error Name:** `errReqParamInvalid`
- **Description:** A request parameter is invalid. It may be of the wrong type, outside the allowed range, or a value that is not permissible.

---

### `REQ-ERR-7`

- **Error Code:** `REQ-ERR-7`
- **Error Name:** `errReqParamOutOfRange`
- **Description:** 

---

### `REQ-ERR-8`

- **Error Code:** `REQ-ERR-8`
- **Error Name:** `errReqParamTypeError`
- **Description:** 

---

### `REQ-ERR-9`

- **Error Code:** `REQ-ERR-9`
- **Error Name:** `errReqContentTypeUnsupported`
- **Description:** The 'Content-Type' of the request is not supported by this endpoint. Please check the API documentation for allowed content types.

## System Errors

### `SYS-ERR-0`

- **Error Code:** `SYS-ERR-0`
- **Error Name:** `internalServerError`
- **Description:** An unexpected condition was encountered on the server that prevented it from fulfilling the request. This is a catch-all for unhandled exceptions, such as marshalling errors or type conversion failures.

---

### `SYS-ERR-1`

- **Error Code:** `SYS-ERR-1`
- **Error Name:** `remoteServiceFail`
- **Description:** A request to a dependent downstream or external service failed. This is a generic error that should be converted to a more specific error in the calling component.

---

### `SYS-ERR-2`

- **Error Code:** `SYS-ERR-2`
- **Error Name:** `databaseFailure`
- **Description:** An unhandled or unexpected error occurred during a database operation, such as a lost connection (`sql.ErrConnDone`).

---

### `SYS-ERR-3`

- **Error Code:** `SYS-ERR-3`
- **Error Name:** `databaseNoRows`
- **Description:** A database query that was expected to return at least one row found no matching records. This is a system-level wrapper for `sql.ErrNoRows`.

---

### `SYS-ERR-4`

- **Error Code:** `SYS-ERR-4`
- **Error Name:** `databaseDuplicateKey`
- **Description:** An `INSERT` or `UPDATE` operation failed because it would have created a duplicate value in a column with a unique constraint.

---

### `SYS-ERR-5`

- **Error Code:** `SYS-ERR-5`
- **Error Name:** `lfsNotFound`
- **Description:** The system could not find or connect to the configured LFS service. This indicates a system configuration issue.

---

### `SYS-ERR-6`

- **Error Code:** `SYS-ERR-6`
- **Error Name:** `lastOrgAdmin`
- **Description:** The requested action to remove a user's admin role is prohibited because they are the sole administrator of an organization. This prevents the organization from being locked.

---

### `SYS-ERR-7`

- **Error Code:** `SYS-ERR-7`
- **Error Name:** `cannotPromoteSelfToAdmin`
- **Description:** The requested action to promote yourself to an administrator is prohibited.

## Task Errors

### `TASK-ERR-0`

- **Error Code:** `TASK-ERR-0`
- **Error Name:** `noEntryFile`
- **Description:** The task requires a specific entry file to start execution (e.g., 'main.py' or 'app.js'), but no such file could be found in the specified source directory.

---

### `TASK-ERR-1`

- **Error Code:** `TASK-ERR-1`
- **Error Name:** `multiHostInferenceNotSupported`
- **Description:** The multi-host inference feature is currently only available for VLLM and SGLang frameworks. Other frameworks do not support this functionality.

---

### `TASK-ERR-2`

- **Error Code:** `TASK-ERR-2`
- **Error Name:** `multiHostInferenceReplicaCount`
- **Description:** For multi-host inference configuration, the minimum number of replicas must be greater than zero to ensure proper service operation.

