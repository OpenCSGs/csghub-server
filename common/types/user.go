package types

import (
	"strings"
	"time"
)

type CreateUserRequest struct {
	// Display name of the user
	Name string `json:"name"`
	// the login name
	Username string `json:"username"`
	Email    string `json:"email" binding:"email"`
	Phone    string `json:"phone"`
	UUID     string `json:"uuid"`
	// user registered from default login page, from casdoor, etc. Possible values:
	//
	// - "default"
	// - "casdoor"
	RegProvider string `json:"reg_provider"`
}

type UpdateUserRequest struct {
	// Display name of the user
	Nickname *string `json:"name,omitempty"`
	// the login name
	Username              string  `json:"-"`
	Email                 *string `json:"email,omitempty" binding:"omitnil,email"`
	EmailVerificationCode *string `json:"email_verification_code,omitempty"`
	UUID                  *string `json:"-"`
	// should be updated by admin
	Roles    *[]string `json:"roles,omitempty" example:"[super_user, admin, personal_user]"`
	Avatar   *string   `json:"avatar,omitempty"`
	Homepage *string   `json:"homepage,omitempty"`
	Bio      *string   `json:"bio,omitempty"`

	//if use want to change username, this should be the only field to send in request body
	NewUserName *string `json:"new_username,omitempty"`

	OpUser string  `json:"-"` // the user who perform this action, used for audit and permission check
	TagIDs []int64 `json:"tag_ids,omitempty"`
}

type SendSMSCodeRequest struct {
	PhoneArea string `json:"phone_area" binding:"required"`
	Phone     string `json:"phone" binding:"required"`
}

type SendSMSCodeResponse struct {
	ExpiredAt time.Time `json:"expired_at"`
}

type SendPublicSMSCodeRequest struct {
	Scene     string `json:"scene" binding:"required"`
	PhoneArea string `json:"phone_area" binding:"required"`
	Phone     string `json:"phone" binding:"required"`
}

type VerifyPublicSMSCodeRequest struct {
	Scene            string `json:"scene" binding:"required"`
	Phone            string `json:"phone" binding:"required"`
	PhoneArea        string `json:"phone_area" binding:"required"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
}

type UpdateUserPhoneRequest struct {
	Phone            *string `json:"phone" binding:"required"`
	PhoneArea        *string `json:"phone_area,omitempty"`
	VerificationCode *string `json:"verification_code" binding:"required"`
}

var _ SensitiveRequestV2 = (*UpdateUserRequest)(nil)

func (u *UpdateUserRequest) GetSensitiveFields() []SensitiveField {
	var fields []SensitiveField
	if u.NewUserName != nil {
		fields = append(fields, SensitiveField{
			Name: "new_username",
			Value: func() string {
				return *u.NewUserName
			},
			Scenario: "nickname_detection",
		})
	}

	if u.Nickname != nil {
		fields = append(fields, SensitiveField{
			Name: "nickname",
			Value: func() string {
				return *u.Nickname
			},
			Scenario: "nickname_detection",
		})
	}

	if u.Bio != nil {
		fields = append(fields, SensitiveField{
			Name: "bio",
			Value: func() string {
				return *u.Bio
			},
			Scenario: "comment_detection",
		})
	}

	if u.Homepage != nil {
		fields = append(fields, SensitiveField{
			Name: "homepage",
			Value: func() string {
				return *u.Homepage
			},
			Scenario: "chat_detection",
		})
	}
	return fields
}

type UpdateUserResp struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CreateUserTokenRequest struct {
	Username  string `json:"-" `
	TokenName string `json:"name"`
	// default to csghub
	Application AccessTokenApp `json:"application,omitempty"`
	// default to empty, means full permission
	Permission string    `json:"permission,omitempty"`
	ExpiredAt  time.Time `json:"expired_at"`
}

// CreateUserTokenRequest implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*CreateUserTokenRequest)(nil)

func (c *CreateUserTokenRequest) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "name",
			Value: func() string {
				return c.TokenName
			},
			Scenario: "nickname_detection",
		},
	}
}

type CheckAccessTokenReq struct {
	Token string `json:"token" binding:"required"`
	// Optional, if given only check the token belongs to this application
	Application string `json:"application"`
}

type CheckAccessTokenResp struct {
	Token       string         `json:"token"`
	TokenName   string         `json:"token_name"`
	Application AccessTokenApp `json:"application"`
	Permission  string         `json:"permission,omitempty"`
	// the login name
	Username string    `json:"user_name"`
	UserUUID string    `json:"user_uuid"`
	ExpireAt time.Time `json:"expire_at"`
}

type UserDatasetsReq struct {
	Owner       string `json:"owner"`
	CurrentUser string `json:"current_user"`
	PageOpts
}

type UserSpacesReq struct {
	SDK         string `json:"sdk"`
	Owner       string `json:"owner"`
	CurrentUser string `json:"current_user"`
	PageOpts
}

type (
	UserModelsReq          = UserDatasetsReq
	UserCodesReq           = UserDatasetsReq
	UserCollectionReq      = UserSpacesReq
	DeleteUserTokenRequest = CreateUserTokenRequest
	UserPromptsReq         = UserDatasetsReq
	UserEvaluationReq      = UserDatasetsReq
	UserMCPsReq            = UserDatasetsReq
)

type PageOpts struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type OffsetPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type User struct {
	ID                int64          `json:"id,omitempty"`
	Username          string         `json:"username"`
	Nickname          string         `json:"nickname"`
	Phone             string         `json:"phone,omitempty"`
	PhoneArea         string         `json:"phone_area,omitempty"`
	Email             string         `json:"email,omitempty"`
	UUID              string         `json:"uuid,omitempty"`
	Avatar            string         `json:"avatar,omitempty"`
	Bio               string         `json:"bio,omitempty"`
	Homepage          string         `json:"homepage,omitempty"`
	Roles             []string       `json:"roles,omitempty"`
	LastLoginAt       string         `json:"last_login_at,omitempty"`
	Orgs              []Organization `json:"orgs,omitempty"`
	CanChangeUserName bool           `json:"can_change_username,omitempty"`
	VerifyStatus      string         `json:"verify_status,omitempty"`
	Labels            []string       `json:"labels,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	Tags              []RepoTag      `json:"tags,omitempty"`
}

func (u User) IsAdmin() bool {
	for _, role := range u.Roles {
		if role == "admin" || role == "super_user" {
			return true
		}
	}
	return false
}

type UserLikesRequest struct {
	Username     string `json:"username"`
	RepoID       int64  `json:"repo_id"`
	CollectionID int64  `json:"collection_id"`
	CurrentUser  string `json:"current_user"`
}

/* for HF compatible apis  */
type WhoamiResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Auth  Auth   `json:"auth"`
}

type AccessToken struct {
	DisplayName string `json:"displayName,omitempty"`
	Role        string `json:"role,omitempty"`
}

type Auth struct {
	AccessToken `json:"accessToken,omitempty"`
	Type        string `json:"type,omitempty"`
}

type UserRepoReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
}

type AccessTokenApp string

const (
	AccessTokenAppGit      AccessTokenApp = "git"
	AccessTokenAppCSGHub                  = AccessTokenAppGit
	AccessTokenAppMirror   AccessTokenApp = "mirror"
	AccessTokenAppStarship AccessTokenApp = "starship"
)

type UserRepoPermission struct {
	CanRead  bool `json:"can_read"`
	CanWrite bool `json:"can_write"`
	CanAdmin bool `json:"can_admin"`
}

type VerifyStatus string

const (
	VerifyStatusPending  VerifyStatus = "pending"
	VerifyStatusApproved VerifyStatus = "approved"
	VerifyStatusRejected VerifyStatus = "rejected"
)

type UserVerifyReq struct {
	UUID        string       `json:"id" binding:"required"`
	RealName    string       `json:"real_name" binding:"required"`
	IDCardFront string       `json:"id_card_front" binding:"required"`
	IDCardBack  string       `json:"id_card_back" binding:"required"`
	Username    string       `json:"username"`
	Status      VerifyStatus `json:"status"`
}

type UserVerifyStatusReq struct {
	Status VerifyStatus `json:"status" binding:"required"` // approved,  rejected
	Reason string       `json:"reason"`
}

type UserLabelsRequest struct {
	Labels []string `json:"labels"`
	UUID   string   `json:"id" binding:"required"`
	OpUser string   `json:"-"`
}

var ValidLabels = map[string]bool{
	"basic":     true,
	"advanced":  true,
	"vip":       true,
	"blacklist": true,
}

func ParseLabels(rawLabels []string) []string {
	labelSet := make(map[string]struct{})
	var result []string

	for _, label := range rawLabels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" {
			continue
		}
		if _, exists := labelSet[label]; !exists {
			labelSet[label] = struct{}{}
			result = append(result, label)
		}
	}

	return result
}

type CloseAccountReq struct {
	Repository bool `json:"repository"`
	Discussion bool `json:"discussion"`
}

type UserIndexReq struct {
	Search       string       `json:"search"`
	VerifyStatus VerifyStatus `json:"verify_status"`
	Labels       []string     `json:"labels"`
	Per          int          `json:"per"`
	ExactMatch   bool         `json:"exact_match"`
}

type UserIndexResp struct {
	Users []*User `json:"users"`
	Error error   `json:"error"`
}

type UserListReq struct {
	VisitorName  string   `json:"visitor_name"`
	Search       string   `json:"search"`
	VerifyStatus string   `json:"verify_status"`
	Labels       []string `json:"labels"`
	Per          int      `json:"per"`
	Page         int      `json:"page"`
	SortBy       string   `json:"sort_by"`
	SortOrder    string   `json:"sort_order"`
	ExactMatch   bool     `json:"exact_match"`
}
