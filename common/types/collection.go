package types

import "time"

var CollectionSorts = []string{"trending", "recently_update", "most_favorite"}

type Collection struct {
	ID           int64                  `json:"id"`
	Username     string                 `json:"username"`
	Theme        string                 `json:"theme"`
	Name         string                 `json:"name"`
	Nickname     string                 `json:"nickname"`
	Description  string                 `json:"description"`
	Private      bool                   `json:"private"`
	Repositories []CollectionRepository `json:"repositories"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Likes        int64                  `json:"likes"`
	UserLikes    bool                   `json:"user_likes"`
	CanWrite     bool                   `json:"can_write"`
	CanManage    bool                   `json:"can_manage"`
}

type CollectionRepository struct {
	ID             int64          `json:"id"`
	UserID         int64          `json:"user_id"`
	Path           string         `json:"path"`
	Name           string         `json:"name"`
	Nickname       string         `json:"nickname"`
	Description    string         `json:"description"`
	Private        bool           `json:"private"`
	Likes          int64          `json:"likes"`
	DownloadCount  int64          `json:"download_count"`
	Tags           []RepoTag      `json:"tags"`
	RepositoryType RepositoryType `json:"repository_type"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type UpdateCollectionReposReq struct {
	RepoIDs  []int64 `json:"repo_ids"`
	Username string  `json:"-"`
	UserID   int64   `json:"-"`
	ID       int64   `json:"-"` //collection ID
}

type CreateCollectionReq struct {
	Username    string `json:"-"`
	UserUUID    string `json:"-"`
	UserID      int64  `json:"-"`
	ID          int64  `json:"-"`
	Namespace   string `json:"namespace" example:"user_or_org_name"`
	Theme       string `json:"theme" example:"#fff000"`
	Name        string `json:"name" example:"collection1"`
	Nickname    string `json:"nickname" example:"collection nick name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

func (c *CreateCollectionReq) SensName() string {
	return c.Name
}

func (c *CreateCollectionReq) SensNickName() string {
	return c.Nickname
}

func (c *CreateCollectionReq) SensDescription() string {
	return c.Description
}

func (c *CreateCollectionReq) SensHomepage() string {
	return ""
}

type CollectionFilter struct {
	Sort   string
	Search string
}
