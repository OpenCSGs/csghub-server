package types

type Commit struct {
	ID             string `json:"id"`
	CommitterName  string `json:"committer_name"`
	CommitterEmail string `json:"committer_email"`
	CommitterDate  string `json:"committer_date"`
	CreatedAt      string `json:"created_at"`
	Message        string `json:"message"`
	AuthorName     string `json:"author_name"`
	AuthorEmail    string `json:"author_email"`
	AuthoredDate   string `json:"authored_date"`
}

type CommitMeta struct {
	SHA string `json:"sha"`
}

type CommitStats struct {
	Total     int `json:"total"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type CommitResponse struct {
	*Commit
	Files   []string      `json:"files"`
	Parents []*CommitMeta `json:"parents"`
	Diff    []byte        `json:"diff"`
	Stats   *CommitStats  `json:"stats"`
}
