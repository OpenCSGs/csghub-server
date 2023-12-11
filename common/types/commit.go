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
