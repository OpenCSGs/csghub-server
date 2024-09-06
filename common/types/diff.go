package types

type Diff struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

type GetDiffBetweenTwoCommitsReq struct {
	Namespace     string         `json:"namespace"`
	Name          string         `json:"name"`
	RepoType      RepositoryType `json:"repo_type"`
	Ref           string         `json:"ref"`
	LeftCommitId  string         `json:"left_commit_id"`
	RightCommitId string         `json:"right_commit_id"`
}

type PostReceiveReq struct {
	Changes      string `json:"changes"`
	GlRepository string `json:"gl_repository"`
	Identifier   string `json:"identifier"`
}

