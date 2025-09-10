package importer

type Importer interface {
	GetRepositoryList(baseURL, accessToken, search string, page, per int) ([]Repository, error)
	GetUser(baseURL, accessToken string) (*User, error)
}

type Repository struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	ImportUrl string `json:"import_url"`
	Private   bool   `json:"private"`
}

type User struct {
	Username string `json:"username"`
}
