package types

// ClawHubResponse is the response structure for ClawHub API
type ClawHubResponse struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp string      `json:"timestamp"`
	RequestId string      `json:"requestId"`
}

// ClawHubSearchResult is the search result for ClawHub API
type ClawHubSearchResult struct {
	Slug        string  `json:"slug"`
	DisplayName string  `json:"displayName"`
	Summary     string  `json:"summary"`
	Version     string  `json:"version"`
	Score       float64 `json:"score"`
	UpdatedAt   int64   `json:"updatedAt,omitempty"`
}

// ClawHubSearchResponse is the search response for ClawHub API
type ClawHubSearchResponse struct {
	Results []ClawHubSearchResult `json:"results"`
}

// ClawHubSkillResponse is the skill detail response for ClawHub API
type ClawHubSkillResponse struct {
	Skill         *ClawHubSkillInfo      `json:"skill"`
	LatestVersion *ClawHubVersionInfo    `json:"latestVersion"`
	Versions      []*ClawHubVersionInfo  `json:"versions"`
	Owner         *ClawHubOwnerInfo      `json:"owner"`
	Moderation    *ClawHubModerationInfo `json:"moderation"`
}

// ClawHubSkillInfo contains skill metadata
type ClawHubSkillInfo struct {
	Slug        string      `json:"slug"`
	DisplayName string      `json:"displayName"`
	Summary     string      `json:"summary"`
	Tags        interface{} `json:"tags"`
	Stats       interface{} `json:"stats"`
	CreatedAt   int64       `json:"createdAt"`
	UpdatedAt   int64       `json:"updatedAt"`
}

// ClawHubVersionInfo contains version metadata
type ClawHubVersionInfo struct {
	Version   string  `json:"version"`
	Commit    string  `json:"commit,omitempty"`
	CreatedAt int64   `json:"createdAt"`
	Changelog string  `json:"changelog"`
	License   *string `json:"license"`
}

// ClawHubSkillVersionResponse is the version detail response for ClawHub API
type ClawHubSkillVersionResponse struct {
	Version *ClawHubSkillVersionInfo `json:"version"`
	Skill   *ClawHubVersionSkillInfo `json:"skill"`
}

// ClawHubSkillVersionInfo contains metadata for one skill version
type ClawHubSkillVersionInfo struct {
	Version         string      `json:"version"`
	CreatedAt       int64       `json:"createdAt"`
	Changelog       string      `json:"changelog"`
	ChangelogSource *string     `json:"changelogSource,omitempty"`
	License         *string     `json:"license"`
	Files           interface{} `json:"files,omitempty"`
}

// ClawHubVersionSkillInfo contains skill identity for one version response
type ClawHubVersionSkillInfo struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
}

// ClawHubOwnerInfo contains owner metadata
type ClawHubOwnerInfo struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Image       string `json:"image,omitempty"`
}

// ClawHubModerationInfo contains moderation status
type ClawHubModerationInfo struct {
	IsSuspicious     bool     `json:"isSuspicious"`
	IsMalwareBlocked bool     `json:"isMalwareBlocked"`
	Verdict          string   `json:"verdict,omitempty"`
	ReasonCodes      []string `json:"reasonCodes,omitempty"`
	UpdatedAt        int64    `json:"updatedAt,omitempty"`
	EngineVersion    string   `json:"engineVersion,omitempty"`
	Summary          string   `json:"summary,omitempty"`
}

// ClawHubSkillListResponse is the skill list response for ClawHub API
type ClawHubSkillListResponse struct {
	Skills []ClawHubSkillResponse `json:"skills"`
	Total  int                    `json:"total"`
}

// ClawHubPublishResponse is the publish response for ClawHub API
type ClawHubPublishResponse struct {
	VersionId string `json:"versionId"`
	Slug      string `json:"slug"`
	Version   string `json:"version"`
}

// ClawHubTokenResponse is the token response for ClawHub API
type ClawHubTokenResponse struct {
	Token string `json:"token"`
}

// ClawHubStatusResponse is the status response for ClawHub API
type ClawHubStatusResponse struct {
	Status string `json:"status"`
}

// ClawHubUserResponse is the user response for ClawHub API
type ClawHubUserResponse struct {
	User ClawHubUserInfo `json:"user"`
}

// ClawHubUserInfo contains user information
type ClawHubUserInfo struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Image       string `json:"image"`
}

// ClawHubUploadURLResponse is the upload URL response for ClawHub API
type ClawHubUploadURLResponse struct {
	URL      string            `json:"url"`
	UUID     string            `json:"uuid,omitempty"`
	FormData map[string]string `json:"formData,omitempty"`
}

// ClawHubResolveResponse is the resolve response for ClawHub API
type ClawHubResolveResponse struct {
	Match         *ClawHubResolveVersionInfo `json:"match"`
	LatestVersion *ClawHubResolveVersionInfo `json:"latestVersion"`
}

// ClawHubResolveVersionInfo contains version info for resolve response
type ClawHubResolveVersionInfo struct {
	Version string `json:"version"`
}

// ClawHubStarResponse is the star response for ClawHub API
type ClawHubStarResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ClawHubUnstarResponse is the unstar response for ClawHub API
type ClawHubUnstarResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ClawHubDeleteResponse is the delete response for ClawHub API
type ClawHubDeleteResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ClawHubTransferResponse is the transfer response for ClawHub API
type ClawHubTransferResponse struct {
	Status     string `json:"status"`
	TransferId string `json:"transferId,omitempty"`
}

// ClawHubPackageResponse is the package response for ClawHub API
type ClawHubPackageResponse struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Version     string               `json:"version"`
	CreatedAt   int64                `json:"createdAt"`
	UpdatedAt   int64                `json:"updatedAt"`
	Owner       ClawHubOwnerInfo     `json:"owner"`
	Versions    []ClawHubVersionInfo `json:"versions,omitempty"`
}

// ClawHubPackageListResponse is the package list response for ClawHub API
type ClawHubPackageListResponse struct {
	Packages []ClawHubPackageResponse `json:"packages"`
	Total    int                      `json:"total"`
}

// ClawHubTelemetryData is the telemetry data structure
type ClawHubTelemetryData struct {
	SkillSlug string                 `json:"skillSlug"`
	Version   string                 `json:"version"`
	EventType string                 `json:"eventType"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ClawHubPublishRequest is the request for publishing a skill
type ClawHubPublishRequest struct {
	Slug               string   `json:"slug"`
	DisplayName        string   `json:"displayName"`
	Version            string   `json:"version"`
	Changelog          string   `json:"changelog"`
	AcceptLicenseTerms bool     `json:"acceptLicenseTerms"`
	Tags               []string `json:"tags"`
	ForkOf             *struct {
		Slug    string `json:"slug"`
		Version string `json:"version,omitempty"`
	} `json:"forkOf,omitempty"`
}

// ClawHubPublishSkillResponse is the response for publishing a skill
type ClawHubPublishSkillResponse struct {
	Ok        bool   `json:"ok"`
	SkillId   string `json:"skillId"`
	VersionId string `json:"versionId"`
}
