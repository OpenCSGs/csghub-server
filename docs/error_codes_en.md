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

---

### `ACT-ERR-4`

- **Error Code:** `ACT-ERR-4`
- **Error Name:** `negativePrice`
- **Description:** The price specified in the request is negative.

## Agent Errors

### `AGENT-ERR-0`

- **Error Code:** `AGENT-ERR-0`
- **Error Name:** `instanceQuotaExceeded`
- **Description:** The instance quota exceeded. Includes agent type, instance count, and quota in the error message.

---

### `AGENT-ERR-1`

- **Error Code:** `AGENT-ERR-1`
- **Error Name:** `instanceNameAlreadyExists`
- **Description:** You have an instance with the same name.

---

### `AGENT-ERR-2`

- **Error Code:** `AGENT-ERR-2`
- **Error Name:** `knowledgeBaseNameAlreadyExists`
- **Description:** You have a knowledge base with the same name.

---

### `AGENT-ERR-3`

- **Error Code:** `AGENT-ERR-3`
- **Error Name:** `mcpServerNameAlreadyExists`
- **Description:** You have an MCP server with the same name.

---

### `AGENT-ERR-4`

- **Error Code:** `AGENT-ERR-4`
- **Error Name:** `pinLimitExceeded`
- **Description:** The pin limit exceeded. Maximum 5 items can be pinned per entity type.

---

### `AGENT-ERR-5`

- **Error Code:** `AGENT-ERR-5`
- **Error Name:** `invalidShareSessionUUID`
- **Description:** The share session uuid is invalid.

---

### `AGENT-ERR-6`

- **Error Code:** `AGENT-ERR-6`
- **Error Name:** `shareSessionUUIDExpired`
- **Description:** The share session uuid expired.

---

### `AGENT-ERR-7`

- **Error Code:** `AGENT-ERR-7`
- **Error Name:** `schedulerQuotaExceeded`
- **Description:** The scheduled task creation quota exceeded. User has reached the limit of schedulers they can create.

---

### `AGENT-ERR-8`

- **Error Code:** `AGENT-ERR-8`
- **Error Name:** `schedulerInstanceNoCapability`
- **Description:** The agent instance does not support scheduling. The "scheduler" capability must be added to the instance metadata.

---

### `AGENT-ERR-9`

- **Error Code:** `AGENT-ERR-9`
- **Error Name:** `schedulerStartTimeInPast`
- **Description:** The specified start time is in the past. One-time schedules must use a future date/time.

---

### `AGENT-ERR-10`

- **Error Code:** `AGENT-ERR-10`
- **Error Name:** `credentialNameAlreadyExists`
- **Description:** You have a credential with the same name.

---

### `AGENT-ERR-11`

- **Error Code:** `AGENT-ERR-11`
- **Error Name:** `runtimeCredentialTokenInvalid`
- **Description:** The runtime credential token is missing, invalid, or expired.

---

### `AGENT-ERR-12`

- **Error Code:** `AGENT-ERR-12`
- **Error Name:** `runtimeCredentialGrantUnavailable`
- **Description:** The runtime credential token is valid, but the requested credential is not granted, revoked, expired, or unavailable.

---

### `AGENT-ERR-13`

- **Error Code:** `AGENT-ERR-13`
- **Error Name:** `credentialVerifyURLInvalid`
- **Description:** The credential verification URL or API endpoint is invalid.

---

### `AGENT-ERR-14`

- **Error Code:** `AGENT-ERR-14`
- **Error Name:** `credentialVerifyTokenInvalid`
- **Description:** The credential token is invalid, expired, or missing required permissions.

---

### `AGENT-ERR-15`

- **Error Code:** `AGENT-ERR-15`
- **Error Name:** `credentialVerifyFailed`
- **Description:** Credential verification failed.

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

---

### `AUTH-ERR-10`

- **Error Code:** `AUTH-ERR-10`
- **Error Name:** `userPhoneNotVerified`
- **Description:** Your phone number has not been verified yet. Please verify your phone number before making any requests.

---

### `AUTH-ERR-11`

- **Error Code:** `AUTH-ERR-11`
- **Error Name:** `needAccessToken`
- **Description:** The request must be authenticated with an access token.

---

### `AUTH-ERR-12`

- **Error Code:** `AUTH-ERR-12`
- **Error Name:** `quotaExceeded`
- **Description:** The request quota limit has been exceeded.

---

### `AUTH-ERR-13`

- **Error Code:** `AUTH-ERR-13`
- **Error Name:** `needOldToken`
- **Description:** Refreshing a token requires the old token.

---

### `AUTH-ERR-14`

- **Error Code:** `AUTH-ERR-14`
- **Error Name:** `noSourceTransferPermission`
- **Description:** The user does not have write permission on the source namespace and cannot transfer the repository away from it.

---

### `AUTH-ERR-15`

- **Error Code:** `AUTH-ERR-15`
- **Error Name:** `noTargetTransferPermission`
- **Description:** The user does not have write permission on the target namespace and cannot transfer the repository to it.

## Collection Errors

### `COLL-ERR-0`

- **Error Code:** `COLL-ERR-0`
- **Error Name:** `repoAlreadyInCollection`
- **Description:** The repository you are trying to add is already present in this collection.

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

---

### `DAT-ERR-3`

- **Error Code:** `DAT-ERR-3`
- **Error Name:** `applicationStatusNotAllowed`
- **Description:** The dataset application is not in a pending state and cannot be approved or rejected.

---

### `DAT-ERR-4`

- **Error Code:** `DAT-ERR-4`
- **Error Name:** `datasetStatusNotAllowed`
- **Description:** The dataset is not in a state that allows the application action to be applied.

---

### `DAT-ERR-5`

- **Error Code:** `DAT-ERR-5`
- **Error Name:** `datasetAlreadyReferenced`
- **Description:** The dataset is already referenced by another dataset as a related dataset and cannot be referenced again.

---

### `DAT-ERR-6`

- **Error Code:** `DAT-ERR-6`
- **Error Name:** `relatedDatasetAlreadyReferenced`
- **Description:** The related dataset is already referenced by another dataset and cannot be used for this application.

---

### `DAT-ERR-7`

- **Error Code:** `DAT-ERR-7`
- **Error Name:** `pendingApplicationExists`
- **Description:** There is already a pending application for this dataset. Only one pending application is allowed per dataset at a time.

## Deploy Errors

### `DEPLOY-ERR-0`

- **Error Code:** `DEPLOY-ERR-0`
- **Error Name:** `codeDeployNameAlreadyExistsErr`
- **Description:** A deploy with the same name already exists for this deploy type.

## Federation_adapter Errors

### `FEDAP-ERR-0`

- **Error Code:** `FEDAP-ERR-0`
- **Error Name:** `tokenExpired`
- **Description:** Both access token and refresh token have expired. Please re-authorize to continue accessing the remote site.

---

### `FEDAP-ERR-1`

- **Error Code:** `FEDAP-ERR-1`
- **Error Name:** `tokenExchangeFailed`
- **Description:** Failed to exchange the access token for a scoped token via RFC 8693 Token Exchange.

---

### `FEDAP-ERR-2`

- **Error Code:** `FEDAP-ERR-2`
- **Error Name:** `siteFetchFailed`
- **Description:** Failed to load the specified federation site configuration. The site may not exist, or the upstream site registry lookup may have failed.

---

### `FEDAP-ERR-3`

- **Error Code:** `FEDAP-ERR-3`
- **Error Name:** `siteUnavailable`
- **Description:** The remote federation site service is currently unavailable.

---

### `FEDAP-ERR-4`

- **Error Code:** `FEDAP-ERR-4`
- **Error Name:** `oauthAuthenticationFailed`
- **Description:** A general error occurred during the OAuth authentication flow that does not fall into a more specific federation adapter error category.

---

### `FEDAP-ERR-5`

- **Error Code:** `FEDAP-ERR-5`
- **Error Name:** `invalidToken`
- **Description:** The provided token is invalid or unauthorized and cannot be used to access the requested resource. It may be malformed, rejected by the remote service, or otherwise not accepted.

---

### `FEDAP-ERR-6`

- **Error Code:** `FEDAP-ERR-6`
- **Error Name:** `userInfoFetchFailed`
- **Description:** Failed to fetch or parse user information from the remote OAuth provider after token exchange.

---

### `FEDAP-ERR-7`

- **Error Code:** `FEDAP-ERR-7`
- **Error Name:** `proxyRequestProcessFailed`
- **Description:** Failed to process the fedap proxy request. This includes proxy URL build failures, outbound request construction or execution failures, upstream response handling failures, and response header processing failures.

---

### `FEDAP-ERR-8`

- **Error Code:** `FEDAP-ERR-8`
- **Error Name:** `oauthAccessDenied`
- **Description:** The upstream OAuth provider returned access_denied because the user declined the authorization request.

---

### `FEDAP-ERR-9`

- **Error Code:** `FEDAP-ERR-9`
- **Error Name:** `oauthCredentialProcessingFailed`
- **Description:** Failed to process locally issued OAuth credentials, such as encrypting tokens or persisting authorization records.

---

### `FEDAP-ERR-10`

- **Error Code:** `FEDAP-ERR-10`
- **Error Name:** `federationAdapterUnauthorized`
- **Description:** The current user has not authorized the requested federation site, or the local authorization is not usable.

---

### `FEDAP-ERR-11`

- **Error Code:** `FEDAP-ERR-11`
- **Error Name:** `federationAdapterSyncRepoFailed`
- **Description:** Failed to sync the remote repository into a local repository, or failed to query the repository sync status.

---

### `FEDAP-ERR-12`

- **Error Code:** `FEDAP-ERR-12`
- **Error Name:** `applicationScopesFetchFailed`
- **Description:** Failed to fetch custom scopes from the remote Casdoor application. This may be caused by an unreachable server, invalid application ID, or an unexpected server response.

---

### `FEDAP-ERR-13`

- **Error Code:** `FEDAP-ERR-13`
- **Error Name:** `federationAdapterRepositoryAlreadyExists`
- **Description:** The requested federation repository already exists locally, or the existing federation sync mapping conflicts with the requested repository.

## Federation_site Errors

### `FS-ERR-0`

- **Error Code:** `FS-ERR-0`
- **Error Name:** `codeFederationSiteNotFound`
- **Description:** The requested federation site was not found.

---

### `FS-ERR-1`

- **Error Code:** `FS-ERR-1`
- **Error Name:** `codeFederationSiteOwnerOrgNotFound`
- **Description:** The specified owner organization (parent site) was not found.

---

### `FS-ERR-2`

- **Error Code:** `FS-ERR-2`
- **Error Name:** `codeFederationSiteSelfReference`
- **Description:** A federation site cannot reference itself as its owner organization.

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
- **Error Name:** `gitCreateBranchFailed`
- **Description:** 

---

### `GIT-ERR-14`

- **Error Code:** `GIT-ERR-14`
- **Error Name:** `gitSetDefaultBranchFailed`
- **Description:** 

---

### `GIT-ERR-15`

- **Error Code:** `GIT-ERR-15`
- **Error Name:** `gitFileNotFound`
- **Description:** The requested file could not be found at the specified path within the given branch or commit of the Git repository.

---

### `GIT-ERR-16`

- **Error Code:** `GIT-ERR-16`
- **Error Name:** `gitUploadFailed`
- **Description:** An error occurred while attempting to upload a file to the Git repository.

---

### `GIT-ERR-17`

- **Error Code:** `GIT-ERR-17`
- **Error Name:** `gitDownloadFailed`
- **Description:** An error occurred while attempting to download a file from the Git repository. Check file path, permissions, and network connectivity.

---

### `GIT-ERR-18`

- **Error Code:** `GIT-ERR-18`
- **Error Name:** `gitConnectionFailed`
- **Description:** A connection to the remote Git server could not be established. Please check your network connection, firewall settings, and the remote server's status.

---

### `GIT-ERR-19`

- **Error Code:** `GIT-ERR-19`
- **Error Name:** `gitLfsError`
- **Description:** An unspecified error occurred during a Git LFS (Large File Storage) operation. Check LFS configuration and logs for more details.

---

### `GIT-ERR-20`

- **Error Code:** `GIT-ERR-20`
- **Error Name:** `fileTooLarge`
- **Description:** The file exceeds the configured maximum size limit for this operation. Consider using Git LFS for large files.

---

### `GIT-ERR-21`

- **Error Code:** `GIT-ERR-21`
- **Error Name:** `gitGetTreeEntryFailed`
- **Description:** Get git tree entry failed. This can be caused by network problems, authentication issues, or the specified tree entry does not exist.

---

### `GIT-ERR-22`

- **Error Code:** `GIT-ERR-22`
- **Error Name:** `gitCommitFilesFailed`
- **Description:** Commit git files failed. This can be caused by network problems, authentication issues, or the specified files do not exist.

---

### `GIT-ERR-23`

- **Error Code:** `GIT-ERR-23`
- **Error Name:** `gitGetBlobsFailed`
- **Description:** Get git blobs failed. This can be caused by network problems, authentication issues, or the specified blobs do not exist.

---

### `GIT-ERR-24`

- **Error Code:** `GIT-ERR-24`
- **Error Name:** `gitGetLfsPointersFailed`
- **Description:** Get git lfs pointers failed. This can be caused by network problems, authentication issues, or the specified lfs pointers do not exist.

---

### `GIT-ERR-25`

- **Error Code:** `GIT-ERR-25`
- **Error Name:** `gitListLastCommitsForTreeFailed`
- **Description:** Get git tree last commit failed. This can be caused by network problems, authentication issues, or the specified tree does not exist.

---

### `GIT-ERR-26`

- **Error Code:** `GIT-ERR-26`
- **Error Name:** `gitGetBlobInfoFailed`
- **Description:** Get git blob info failed. This can be caused by network problems, authentication issues, or the specified blob does not exist.

---

### `GIT-ERR-27`

- **Error Code:** `GIT-ERR-27`
- **Error Name:** `gitListFilesFailed`
- **Description:** Get git files failed. This can be caused by network problems, authentication issues, or the specified files do not exist.

---

### `GIT-ERR-28`

- **Error Code:** `GIT-ERR-28`
- **Error Name:** `gitCreateMirrorFailed`
- **Description:** Create mirror failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-29`

- **Error Code:** `GIT-ERR-29`
- **Error Name:** `gitMirrorSyncFailed`
- **Description:** Sync mirror failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-30`

- **Error Code:** `GIT-ERR-30`
- **Error Name:** `gitCheckRepositoryExistsFailed`
- **Description:** Check repository exists failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-31`

- **Error Code:** `GIT-ERR-31`
- **Error Name:** `gitCreateRepositoryFailed`
- **Description:** Create repository failed. This can be caused by network problems, authentication issues.

---

### `GIT-ERR-32`

- **Error Code:** `GIT-ERR-32`
- **Error Name:** `gitDeleteRepositoryFailed`
- **Description:** delete repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-33`

- **Error Code:** `GIT-ERR-33`
- **Error Name:** `gitGetRepositoryFailed`
- **Description:** get repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-34`

- **Error Code:** `GIT-ERR-34`
- **Error Name:** `gitServiceUnavaliable`
- **Description:** The Git hosting service is temporarily unavailable or unreachable. Please try again later.

---

### `GIT-ERR-35`

- **Error Code:** `GIT-ERR-35`
- **Error Name:** `gitCopyRepositoryFailed`
- **Description:** copy repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-36`

- **Error Code:** `GIT-ERR-36`
- **Error Name:** `gitReplicateRepositoryFailed`
- **Description:** replicate repository failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-37`

- **Error Code:** `GIT-ERR-37`
- **Error Name:** `gitUsingGitInXnetRepository`
- **Description:** Using git in xnet-enabled repository error. Git operations are not supported in repositories enabled with xnet.

---

### `GIT-ERR-38`

- **Error Code:** `GIT-ERR-38`
- **Error Name:** `gitGetRepositorySizeFailed`
- **Description:** Get repository size failed. This can be caused by network problems, authentication issues, or the specified repository does not exist.

---

### `GIT-ERR-39`

- **Error Code:** `GIT-ERR-39`
- **Error Name:** `gitCreateForkFailed`
- **Description:** 

---

### `GIT-ERR-40`

- **Error Code:** `GIT-ERR-40`
- **Error Name:** `gitGetArchiveFailed`
- **Description:** Failed to get archive from git repository.

---

### `GIT-ERR-41`

- **Error Code:** `GIT-ERR-41`
- **Error Name:** `gitInvalidURL`
- **Description:** The provided git URL is invalid or malformed. Please check the URL format and try again.

## Invitation Errors

### `INVITATION-ERR-0`

- **Error Code:** `INVITATION-ERR-0`
- **Error Name:** `userPhoneNotSet`
- **Description:** The phone number is not set, cannot create invitation code.

---

### `INVITATION-ERR-1`

- **Error Code:** `INVITATION-ERR-1`
- **Error Name:** `invitationNotFound`
- **Description:** The invitation not found.

---

### `INVITATION-ERR-2`

- **Error Code:** `INVITATION-ERR-2`
- **Error Name:** `userAlreadyHasInvitationCode`
- **Description:** The invitation code already exists.

## License Errors

### `LICENSE-ERR-0`

- **Error Code:** `LICENSE-ERR-0`
- **Error Name:** `noActiveLicense`
- **Description:** No active license found for the current system.

---

### `LICENSE-ERR-1`

- **Error Code:** `LICENSE-ERR-1`
- **Error Name:** `licenseExpired`
- **Description:** The license is expired, could not be verified and imported.

## Mcp_gateway Errors

### `MCPGW-ERR-0`

- **Error Code:** `MCPGW-ERR-0`
- **Error Name:** `gatewayMCPServerNameAlreadyExists`
- **Description:** An MCP server with this name already exists in the gateway.

---

### `MCPGW-ERR-1`

- **Error Code:** `MCPGW-ERR-1`
- **Error Name:** `gatewayMCPServerInvalidName`
- **Description:** The MCP server name does not meet naming requirements.

## Mirror Errors

### `MIRROR-ERR-0`

- **Error Code:** `MIRROR-ERR-0`
- **Error Name:** `mirrorSourceConflict`
- **Description:** The target repository already has another mirror source.

---

### `MIRROR-ERR-1`

- **Error Code:** `MIRROR-ERR-1`
- **Error Name:** `mirrorRepoSyncing`
- **Description:** Repository synchronization is in progress.

---

### `MIRROR-ERR-2`

- **Error Code:** `MIRROR-ERR-2`
- **Error Name:** `mirrorRepoSyncFailed`
- **Description:** Repository synchronization failed and Git data could not be retrieved.

---

### `MIRROR-ERR-3`

- **Error Code:** `MIRROR-ERR-3`
- **Error Name:** `mirrorTaskStateInvalid`
- **Description:** The current mirror task state is inconsistent with its execution status and cannot be determined reliably.

---

### `MIRROR-ERR-4`

- **Error Code:** `MIRROR-ERR-4`
- **Error Name:** `mirrorRepoSyncCanceled`
- **Description:** Repository synchronization was canceled before Git data became available.

---

### `MIRROR-ERR-5`

- **Error Code:** `MIRROR-ERR-5`
- **Error Name:** `mirrorSourceRepoAuthInvalid`
- **Description:** Source repository authentication information is invalid.

---

### `MIRROR-ERR-6`

- **Error Code:** `MIRROR-ERR-6`
- **Error Name:** `sourceNamespaceMappingExists`
- **Description:** The source namespace already has a mapping.

---

### `MIRROR-ERR-7`

- **Error Code:** `MIRROR-ERR-7`
- **Error Name:** `sourceNamespaceMappingNotFound`
- **Description:** The source namespace mapping does not exist.

## Moderation Errors

### `MOD-ERR-0`

- **Error Code:** `MOD-ERR-0`
- **Error Name:** `codeNameRequire`
- **Description:** The request parameter does not match the server requirements, and the server cannot process the request.

---

### `MOD-ERR-1`

- **Error Code:** `MOD-ERR-1`
- **Error Name:** `codeWordRequire`
- **Description:** The request parameter does not match the server requirements, and the server cannot process the request.

## Pd Errors

### `PD-ERR-0`

- **Error Code:** `PD-ERR-0`
- **Error Name:** `codePDConfigInvalid`
- **Description:** The PD disaggregation configuration provided in the request is invalid. This includes missing PD config, missing prefill or decode role, invalid TP/DP/EP/PodsSize values, or GPU count mismatch.

## Repo Errors

### `REPO-ERR-0`

- **Error Code:** `REPO-ERR-0`
- **Error Name:** `codeRepoAlreadyExistErr`
- **Description:** A repository with the same name already exists.

---

### `REPO-ERR-1`

- **Error Code:** `REPO-ERR-1`
- **Error Name:** `codeRepoNameInvalidErr`
- **Description:** The repository name is invalid.

---

### `REPO-ERR-2`

- **Error Code:** `REPO-ERR-2`
- **Error Name:** `codeNamespaceNotFoundErr`
- **Description:** The namespace does not exist.

---

### `REPO-ERR-3`

- **Error Code:** `REPO-ERR-3`
- **Error Name:** `codeRepoNotFoundErr`
- **Description:** The repository was not found.

---

### `REPO-ERR-4`

- **Error Code:** `REPO-ERR-4`
- **Error Name:** `codeRepoNoDefaultBranchErr`
- **Description:** No revision specified and repository has no default branch. Please specify a revision.

---

### `REPO-ERR-5`

- **Error Code:** `REPO-ERR-5`
- **Error Name:** `codeCodeZipDownloadFailedErr`
- **Description:** Failed to download code repository as zip archive.

---

### `REPO-ERR-6`

- **Error Code:** `REPO-ERR-6`
- **Error Name:** `codeBatchGetRepoExtraFailedErr`
- **Description:** Failed to batch get repository extra information.

---

### `REPO-ERR-7`

- **Error Code:** `REPO-ERR-7`
- **Error Name:** `codeChangePathBlockedErr`
- **Description:** Cannot change repository path because dependent entities exist. Please remove them first.

---

### `REPO-ERR-8`

- **Error Code:** `REPO-ERR-8`
- **Error Name:** `codeSensitiveCheckNotPassedErr`
- **Description:** The repository cannot be made public because the compliance scan has not passed.

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

---

### `REQ-ERR-10`

- **Error Code:** `REQ-ERR-10`
- **Error Name:** `errRateLimitExceeded`
- **Description:** The user has sent too many requests in a given amount of time. Further requests will be blocked until the rate limit resets or a valid captcha is provided.

---

### `REQ-ERR-11`

- **Error Code:** `REQ-ERR-11`
- **Error Name:** `errLimitedIPLocation`
- **Description:** Requests originating from this IP location are restricted. To proceed, please complete a captcha verification.

---

### `REQ-ERR-12`

- **Error Code:** `REQ-ERR-12`
- **Error Name:** `errCaptchaIncorrect`
- **Description:** The provided captcha verification failed. Please try again with a valid captcha.

---

### `REQ-ERR-13`

- **Error Code:** `REQ-ERR-13`
- **Error Name:** `errTargetNamespaceNotFound`
- **Description:** The specified target namespace was not found in the system. Please verify the namespace exists before creating or updating the mapping.

---

### `REQ-ERR-14`

- **Error Code:** `REQ-ERR-14`
- **Error Name:** `errTransferSameNamespace`
- **Description:** The target namespace for transfer is the same as the current namespace. Ownership transfer requires a different namespace.

---

### `REQ-ERR-15`

- **Error Code:** `REQ-ERR-15`
- **Error Name:** `errTransferTargetExists`
- **Description:** A repository with the same name already exists in the target namespace. The transfer cannot proceed because of the naming conflict.

---

### `REQ-ERR-16`

- **Error Code:** `REQ-ERR-16`
- **Error Name:** `errTransferNotSupported`
- **Description:** The repository cannot be transferred because it does not have a hashed path. Only repositories with hashed paths support ownership transfer.

## Resource Errors

### `RESOURCE-ERR-0`

- **Error Code:** `RESOURCE-ERR-0`
- **Error Name:** `codeResourceNotFoundErr`
- **Description:** The resource was not found.

---

### `RESOURCE-ERR-1`

- **Error Code:** `RESOURCE-ERR-1`
- **Error Name:** `codeResourceUnavailableErr`
- **Description:** The resource is temporarily unavailable.

## Runner Errors

### `RUNNER-ERR-0`

- **Error Code:** `RUNNER-ERR-0`
- **Error Name:** `codeRunnerMaxRevisionErr`
- **Description:** The max revision number exceeds the max replica number.

---

### `RUNNER-ERR-1`

- **Error Code:** `RUNNER-ERR-1`
- **Error Name:** `codeRunnerGetMaxScaleFailedErr`
- **Description:** Failed to get max scale.

---

### `RUNNER-ERR-2`

- **Error Code:** `RUNNER-ERR-2`
- **Error Name:** `codeRunnerDuplicateRevisionErr`
- **Description:** The revision with commit already exists.

---

### `RUNNER-ERR-3`

- **Error Code:** `RUNNER-ERR-3`
- **Error Name:** `codeRevisionNotReadyErr`
- **Description:** The revision is not ready.

---

### `RUNNER-ERR-4`

- **Error Code:** `RUNNER-ERR-4`
- **Error Name:** `codeTrafficPercentNotZeroErr`
- **Description:** The traffic percent is not zero.

## Sandbox Errors

### `SANDBOX-ERR-0`

- **Error Code:** `SANDBOX-ERR-0`
- **Error Name:** `codeSandboxNameEmptyErr`
- **Description:** Sandbox name is empty.

---

### `SANDBOX-ERR-1`

- **Error Code:** `SANDBOX-ERR-1`
- **Error Name:** `codeSandboxNameTooLongErr`
- **Description:** Sandbox name exceeds maximum length (253 characters).

---

### `SANDBOX-ERR-2`

- **Error Code:** `SANDBOX-ERR-2`
- **Error Name:** `codeSandboxNameUppercaseErr`
- **Description:** Sandbox name contains uppercase letters (only lowercase allowed).

---

### `SANDBOX-ERR-3`

- **Error Code:** `SANDBOX-ERR-3`
- **Error Name:** `codeSandboxNameInvalidCharErr`
- **Description:** Sandbox name has invalid characters or format (only a-z, 0-9, -, . allowed; cannot start/end with -/. or have consecutive -/. ).

## Sensitive Errors

### `SENSITIVE-ERR-0`

- **Error Code:** `SENSITIVE-ERR-0`
- **Error Name:** `codeSensitiveInfoNotAllowedErr`
- **Description:** The sensitive information is not allowed.

## Serverless Errors

### `SERVERLESS-ERR-0`

- **Error Code:** `SERVERLESS-ERR-0`
- **Error Name:** `codeStrategyTypeErr`
- **Description:** The request parameter does not match the server requirements, and the server cannot process the request.

---

### `SERVERLESS-ERR-1`

- **Error Code:** `SERVERLESS-ERR-1`
- **Error Name:** `codeDeployNotFoundErr`
- **Description:** The deploy not found.

---

### `SERVERLESS-ERR-2`

- **Error Code:** `SERVERLESS-ERR-2`
- **Error Name:** `codeDeployStatusNotMatchErr`
- **Description:** The deploy status not match.

---

### `SERVERLESS-ERR-3`

- **Error Code:** `SERVERLESS-ERR-3`
- **Error Name:** `codeDeployMaxReplicaErr`
- **Description:** The deploy max replica not match.

---

### `SERVERLESS-ERR-4`

- **Error Code:** `SERVERLESS-ERR-4`
- **Error Name:** `codeRevisionNotFoundErr`
- **Description:** The revision not found.

---

### `SERVERLESS-ERR-5`

- **Error Code:** `SERVERLESS-ERR-5`
- **Error Name:** `codeInvalidPercentErr`
- **Description:** The percent not match.

---

### `SERVERLESS-ERR-6`

- **Error Code:** `SERVERLESS-ERR-6`
- **Error Name:** `codeCommitIDEmptyErr`
- **Description:** The commit id is empty.

---

### `SERVERLESS-ERR-7`

- **Error Code:** `SERVERLESS-ERR-7`
- **Error Name:** `codeTrafficInvalidErr`
- **Description:** no other valid revision except

---

### `SERVERLESS-ERR-8`

- **Error Code:** `SERVERLESS-ERR-8`
- **Error Name:** `codeInvalidCommitIDErr`
- **Description:** The commit id is invalid.

## Skill Errors

### `SKILL-ERR-0`

- **Error Code:** `SKILL-ERR-0`
- **Error Name:** `skillNotFound`
- **Description:** The requested skill could not be found.

---

### `SKILL-ERR-1`

- **Error Code:** `SKILL-ERR-1`
- **Error Name:** `skillVersionNotFound`
- **Description:** The requested skill version could not be found.

---

### `SKILL-ERR-2`

- **Error Code:** `SKILL-ERR-2`
- **Error Name:** `skillPublishFailed`
- **Description:** Failed to publish the skill.

---

### `SKILL-ERR-3`

- **Error Code:** `SKILL-ERR-3`
- **Error Name:** `skillDownloadFailed`
- **Description:** Failed to download the skill archive.

---

### `SKILL-ERR-4`

- **Error Code:** `SKILL-ERR-4`
- **Error Name:** `skillResolveFailed`
- **Description:** Failed to resolve the skill.

---

### `SKILL-ERR-5`

- **Error Code:** `SKILL-ERR-5`
- **Error Name:** `skillUserNotFound`
- **Description:** The user associated with the skill operation could not be found.

---

### `SKILL-ERR-6`

- **Error Code:** `SKILL-ERR-6`
- **Error Name:** `skillVersionCreateFailed`
- **Description:** Failed to create skill version record.

---

### `SKILL-ERR-7`

- **Error Code:** `SKILL-ERR-7`
- **Error Name:** `skillVersionUpdateFailed`
- **Description:** Failed to update skill version record.

---

### `SKILL-ERR-8`

- **Error Code:** `SKILL-ERR-8`
- **Error Name:** `skillPublishFileCountExceeded`
- **Description:** The number of files in the skill publish request exceeds the allowed limit.

---

### `SKILL-ERR-9`

- **Error Code:** `SKILL-ERR-9`
- **Error Name:** `skillPublishFileSizeExceeded`
- **Description:** The total size of files in the skill publish request exceeds the allowed limit.

## Spaces Errors

### `SPACE-ERR-0`

- **Error Code:** `SPACE-ERR-0`
- **Error Name:** `codeGetSpaceDockerTemplatePathFailedErr`
- **Description:** Failed to get the space Docker template path.

---

### `SPACE-ERR-1`

- **Error Code:** `SPACE-ERR-1`
- **Error Name:** `codeSpaceNameAlreadyExistErr`
- **Description:** The space name already exists.

---

### `SPACE-ERR-2`

- **Error Code:** `SPACE-ERR-2`
- **Error Name:** `codeSpaceInitFailedErr`
- **Description:** Failed to initialize the space.

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

---

### `SYS-ERR-8`

- **Error Code:** `SYS-ERR-8`
- **Error Name:** `cannotSetRepoVisibility`
- **Description:** The requested action to change the visibility setting of a repository is prohibited. Because sensitive check not passed.

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

---

### `TASK-ERR-3`

- **Error Code:** `TASK-ERR-3`
- **Error Name:** `multiHostNotebookNotSupported`
- **Description:** The multi-host notebook feature (running notebook tasks across multiple hosts) is not supported. Use single-host notebook execution instead. This limitation applies to distributed notebook sessions which require synchronized kernel/state across hosts and is currently not implemented.

---

### `TASK-ERR-4`

- **Error Code:** `TASK-ERR-4`
- **Error Name:** `notEnoughResource`
- **Description:** The task requires more resources than are available in the cluster. This error occurs when the cluster does not have sufficient capacity to run the task.

---

### `TASK-ERR-5`

- **Error Code:** `TASK-ERR-5`
- **Error Name:** `clusterUnavailable`
- **Description:** The cluster is currently unavailable, either due to maintenance or other reasons. This error occurs when the cluster is not ready to accept new tasks.

---

### `TASK-ERR-6`

- **Error Code:** `TASK-ERR-6`
- **Error Name:** `resourceStatusUncertain`
- **Description:** The resource availability could not be determined because the cluster lacks ClusterRole permissions to query node resources and no ResourceQuota is configured in the namespace. This error occurs when the system cannot verify if enough resources are available for the task.

## User Errors

### `USER-ERR-0`

- **Error Code:** `USER-ERR-0`
- **Error Name:** `needPhone`
- **Description:** The request must include a phone number in the header or body to identify the target account.

---

### `USER-ERR-1`

- **Error Code:** `USER-ERR-1`
- **Error Name:** `needDifferentPhone`
- **Description:** The new phone number must be different from the current phone number.

---

### `USER-ERR-2`

- **Error Code:** `USER-ERR-2`
- **Error Name:** `phoneAlreadyExistsInSSO`
- **Description:** The new phone number already exists in sso service.

---

### `USER-ERR-3`

- **Error Code:** `USER-ERR-3`
- **Error Name:** `forbidChangePhone`
- **Description:** The phone number cannot be changed.

---

### `USER-ERR-4`

- **Error Code:** `USER-ERR-4`
- **Error Name:** `failedToUpdatePhone`
- **Description:** Failed to update phone number.

---

### `USER-ERR-5`

- **Error Code:** `USER-ERR-5`
- **Error Name:** `forbidSendPhoneVerifyCodeFrequently`
- **Description:** Send phone verify code frequently.

---

### `USER-ERR-6`

- **Error Code:** `USER-ERR-6`
- **Error Name:** `failedSendPhoneVerifyCode`
- **Description:** Failed to send phone verify code.

---

### `USER-ERR-7`

- **Error Code:** `USER-ERR-7`
- **Error Name:** `phoneVerifyCodeExpiredOrNotFound`
- **Description:** Phone verify code expired or not found.

---

### `USER-ERR-8`

- **Error Code:** `USER-ERR-8`
- **Error Name:** `phoneVerifyCodeInvalid`
- **Description:** Phone verify code is invalid.

---

### `USER-ERR-9`

- **Error Code:** `USER-ERR-9`
- **Error Name:** `verificationCodeRequired`
- **Description:** Verification code can not be empty.

---

### `USER-ERR-10`

- **Error Code:** `USER-ERR-10`
- **Error Name:** `verificationCodeLengthInvalid`
- **Description:** Verification code length must be 6.

---

### `USER-ERR-11`

- **Error Code:** `USER-ERR-11`
- **Error Name:** `invalidPhoneNumber`
- **Description:** Invalid phone number.

---

### `USER-ERR-12`

- **Error Code:** `USER-ERR-12`
- **Error Name:** `usernameExists`
- **Description:** The username provided already exists in the system.

---

### `USER-ERR-13`

- **Error Code:** `USER-ERR-13`
- **Error Name:** `emailExists`
- **Description:** The email address provided already exists in the system.

---

### `USER-ERR-14`

- **Error Code:** `USER-ERR-14`
- **Error Name:** `adminUserCannotBeDeleted`
- **Description:** The admin user cannot be deleted.

---

### `USER-ERR-15`

- **Error Code:** `USER-ERR-15`
- **Error Name:** `userHasOrganizations`
- **Description:** The user who owns organizations cannot be deleted.

---

### `USER-ERR-16`

- **Error Code:** `USER-ERR-16`
- **Error Name:** `userHasDeployments`
- **Description:** The user who owns deployments cannot be deleted.

---

### `USER-ERR-17`

- **Error Code:** `USER-ERR-17`
- **Error Name:** `userHasBills`
- **Description:** The user who owns bills cannot be deleted.

---

### `USER-ERR-18`

- **Error Code:** `USER-ERR-18`
- **Error Name:** `userEmailEmpty`
- **Description:** User email is empty.

---

### `USER-ERR-19`

- **Error Code:** `USER-ERR-19`
- **Error Name:** `uuidConflict`
- **Description:** The UUID generated for the organization already exists in the system.

---

### `USER-ERR-20`

- **Error Code:** `USER-ERR-20`
- **Error Name:** `namespaceAlreadyExists`
- **Description:** The namespace already exists in the system.
