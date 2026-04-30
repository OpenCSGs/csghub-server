package types

type IdentifyIndustryTagsReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	Branch      string         `json:"branch"`
	Description string         `json:"description"`
	Readme      string         `json:"readme"`
}

type IdentifyIndustryTagsResult struct {
	TagIDs    []int64  `json:"tag_ids"`
	TagNames  []string `json:"tag_names"`
	MatchedBy string   `json:"matched_by"`
	Reason    string   `json:"reason"`
}

type ScanRepoIndustryTagsReq struct {
	Namespace string         `json:"namespace"`
	Name      string         `json:"name"`
	Branch    string         `json:"branch"`
	RepoType  RepositoryType `json:"repo_type"`
}

type ClearRepoIndustryTagsReq struct {
	Namespace string         `json:"namespace"`
	Name      string         `json:"name"`
	RepoType  RepositoryType `json:"repo_type"`
}
