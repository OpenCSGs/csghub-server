package errorx

import "fmt"

const errImportPrefix = "IMPORT-ERR"

const (
	importGitlabInstanceInvalid = iota
	importGitlabInstanceUnreachable
	importGitlabInstanceUnauthorized
)

var (
	// ErrImportGitlabInstanceInvalid indicates that the provided base URL is not a valid GitLab instance.
	//
	// Description: The provided base URL is not a valid GitLab instance. The GitLab version endpoint did not return a recognizable GitLab response.
	//
	// Description_ZH: 提供的地址不是有效的 GitLab 实例。GitLab 版本接口未返回可识别的 GitLab 响应。
	//
	// en-US: The provided address is not a valid GitLab instance
	//
	// zh-CN: 提供的地址不是有效的 GitLab 实例
	//
	// zh-HK: 提供的地址不是有效的 GitLab 實例
	ErrImportGitlabInstanceInvalid error = CustomError{prefix: errImportPrefix, code: importGitlabInstanceInvalid}

	// ErrImportGitlabInstanceUnreachable indicates that the GitLab instance could not be reached.
	//
	// Description: The GitLab instance could not be reached. The request to the GitLab version endpoint failed due to a network or URL error.
	//
	// Description_ZH: 无法访问 GitLab 实例。由于网络或地址错误，请求 GitLab 版本接口失败。
	//
	// en-US: The GitLab instance is unreachable
	//
	// zh-CN: 无法访问 GitLab 实例
	//
	// zh-HK: 無法訪問 GitLab 實例
	ErrImportGitlabInstanceUnreachable error = CustomError{prefix: errImportPrefix, code: importGitlabInstanceUnreachable}

	// ErrImportGitlabInstanceUnauthorized indicates that the access token was rejected by the GitLab instance.
	//
	// Description: The access token was rejected by the GitLab instance. The GitLab version endpoint returned an authentication error.
	//
	// Description_ZH: 访问令牌被 GitLab 实例拒绝。GitLab 版本接口返回了鉴权错误。
	//
	// en-US: Unauthorized to access the GitLab instance, please check the access token
	//
	// zh-CN: 无权访问 GitLab 实例，请检查访问令牌
	//
	// zh-HK: 無權訪問 GitLab 實例，請檢查訪問令牌
	ErrImportGitlabInstanceUnauthorized error = CustomError{prefix: errImportPrefix, code: importGitlabInstanceUnauthorized}
)

// ImportGitlabInstanceInvalid wraps a non-GitLab base URL with the provided address as context.
func ImportGitlabInstanceInvalid(baseURL string, cause error) error {
	return CustomError{
		prefix:  errImportPrefix,
		code:    importGitlabInstanceInvalid,
		err:     fmt.Errorf("the address %q is not a valid gitlab instance: %w", baseURL, cause),
		context: Ctx().Set("base_url", baseURL),
	}
}

// ImportGitlabInstanceUnreachable wraps a network or URL error encountered while reaching the GitLab instance.
func ImportGitlabInstanceUnreachable(baseURL string, cause error) error {
	return CustomError{
		prefix:  errImportPrefix,
		code:    importGitlabInstanceUnreachable,
		err:     fmt.Errorf("failed to reach gitlab instance at %q: %w", baseURL, cause),
		context: Ctx().Set("base_url", baseURL),
	}
}

// ImportGitlabInstanceUnauthorized wraps an authentication failure returned by the GitLab instance.
func ImportGitlabInstanceUnauthorized(baseURL string, cause error) error {
	return CustomError{
		prefix:  errImportPrefix,
		code:    importGitlabInstanceUnauthorized,
		err:     fmt.Errorf("unauthorized to access gitlab instance at %q, please check the access token: %w", baseURL, cause),
		context: Ctx().Set("base_url", baseURL),
	}
}
