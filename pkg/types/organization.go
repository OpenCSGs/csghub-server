package types

import "git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"

type CreateOrgReq struct {
	Name        string        `json:"name"`
	FullName    string        `json:"full_name"`
	Description string        `json:"description"`
	Username    string        `json:"username"`
	User        database.User `json:"user"`
}

type EditOrgReq struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
}
