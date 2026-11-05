package types

import (
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/rebac"
)

// UserQueryOption controls optional data loaded alongside a user's active roles.
type UserQueryOption uint8

// UserWithAdminPermissions loads administrator definitions and grants for permission checks.
// Without this option, user queries load role codes only.
const UserWithAdminPermissions UserQueryOption = 1

// UserRole represents a user's permission level in an organization.
type UserRole string

const (
	// UserAdmin grants organization administration permission.
	UserAdmin UserRole = "admin"
	// UserWrite grants organization write permission.
	UserWrite UserRole = "write"
	// UserRead grants organization read permission.
	UserRead UserRole = "read"
)

// ReBACRelation maps the organization role to the corresponding direct ReBAC relation.
func (role UserRole) ReBACRelation() (rebac.Relation, bool) {
	switch role {
	case UserAdmin:
		return rebac.RelationAdmin, true
	case UserWrite:
		return rebac.RelationWriter, true
	case UserRead:
		return rebac.RelationReader, true
	default:
		return "", false
	}
}

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

// UpdateUserRequest contains editable user profile fields.
type UpdateUserRequest struct {
	// Display name of the user
	Nickname *string `json:"name,omitempty"`
	// the login name
	Username              string  `json:"-"`
	Email                 *string `json:"email,omitempty" binding:"omitnil,email"`
	EmailVerificationCode *string `json:"email_verification_code,omitempty"`
	Phone                 *string `json:"phone,omitempty"`
	PhoneArea             *string `json:"phone_area,omitempty"`
	UUID                  *string `json:"-"`
	Avatar                *string `json:"avatar,omitempty"`
	Homepage              *string `json:"homepage,omitempty"`
	Bio                   *string `json:"bio,omitempty"`

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

// OAuthExchangeTokenReq contains the payload for exchanging a Casdoor JWT
// access token for a locally issued JWT.
type OAuthExchangeTokenReq struct {
	// AccessToken is the Casdoor-issued JWT access token.
	AccessToken string `json:"access_token" binding:"required"`
	// Scopes is a reserved field for future authorization checks.
	// The current implementation keeps the parameter for forward compatibility
	// but does not validate or enforce any scope values yet.
	Scopes []string `json:"scopes"`
}

// OAuthExchangeTokenResp contains the locally issued JWT and mapped user
// identity after a successful token exchange.
type OAuthExchangeTokenResp struct {
	JWT         string `json:"jwt"`
	CurrentUser string `json:"current_user"`
	UserUUID    string `json:"user_uuid"`
}

type CreateUserTokenRequest struct {
	ID        int64  `json:"-"`
	Username  string `json:"-" `
	OpUUID    string `json:"-"`
	NSUUID    string `json:"-"`
	TokenName string `json:"name"`
	// default to csghub
	Application AccessTokenApp `json:"application,omitempty"`
	// default to empty, means full permission
	Permission string    `json:"permission,omitempty"`
	ExpiredAt  time.Time `json:"expired_at"`
	// Quotas supports submitting multiple quota records in one create request.
	Quotas []UpdateAPIKeyQuotaItem `json:"quotas"`
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
	ID          int64          `json:"id"`
	Token       string         `json:"token"`
	TokenName   string         `json:"token_name"`
	Application AccessTokenApp `json:"application"`
	Permission  string         `json:"permission,omitempty"`
	// the login name
	Username  string                        `json:"user_name"`
	UserUUID  string                        `json:"user_uuid"`
	ExpireAt  time.Time                     `json:"expire_at"`
	NSUUID    string                        `json:"ns_uuid"`
	Quotas    []AccountAccessTokenQuotaResp `json:"quotas"`
	CreatedAt time.Time                     `json:"created_at"`
	UpdatedAt time.Time                     `json:"updated_at"`
	TokenType string                        `json:"token_type"`
	IsActive  bool                          `json:"is_active"`
}

// AccountAccessTokenQuotaResp is the response representation of a single quota
// record for an API key.
type AccountAccessTokenQuotaResp struct {
	ID             int64                    `json:"id"`
	QuotaType      AccountingQuotaType      `json:"quota_type"`
	QuotaValueType AccountingQuotaValueType `json:"quota_value_type"`
	Usage          float64                  `json:"usage"`
	Quota          float64                  `json:"quota"`
	PeriodStart    int64                    `json:"period_start"`
	PeriodEnd      int64                    `json:"period_end"`
	Allocation     string                   `json:"allocation"`
	LastUsedAt     *time.Time               `json:"last_used_at"`
}

type GetAccessTokenRequest struct {
	Username    string         `json:"username"`
	OpUUID      string         `json:"op_uuid"`
	NSUUID      string         `json:"ns_uuid"`
	Application AccessTokenApp `json:"application"`
}

type CreateAPIKeyRequest struct {
	KeyName   string     `json:"name" binding:"required"`
	ExpiredAt *time.Time `json:"expired_at"`
	// Quotas supports submitting multiple quota records in one request.
	Quotas []UpdateAPIKeyQuotaItem `json:"quotas" binding:"required,min=1,dive"`
}

type UpdateAPIKeyRequest struct {
	CurrentUser string     `json:"-"`
	OpUUID      string     `json:"-"`
	NSUUID      string     `json:"-"`
	ID          int64      `json:"-"`
	KeyName     *string    `json:"name"`
	ExpiredAt   *time.Time `json:"expired_at"`
	// Quotas supports submitting multiple quota records in one request.
	Quotas []UpdateAPIKeyQuotaItem `json:"quotas"`
}

// UpdateAPIKeyStatusRequest enables or disables an API key without deleting it.
type UpdateAPIKeyStatusRequest struct {
	CurrentUser string `json:"-"`
	OpUUID      string `json:"-"`
	NSUUID      string `json:"-"`
	ID          int64  `json:"-"`
	IsActive    *bool  `json:"is_active" binding:"required"`
}

// UpdateAPIKeyQuotaItem describes a single quota entry within a batch update.
type UpdateAPIKeyQuotaItem struct {
	ID         int64                    `json:"id"`
	QuotaType  AccountingQuotaType      `json:"quota_type"`
	ValueType  AccountingQuotaValueType `json:"quota_value_type"`
	Quota      float64                  `json:"quota"`
	Allocation string                   `json:"allocation"`
}

type UserDatasetsReq struct {
	Owner       string `json:"owner"`
	CurrentUser string `json:"current_user"`
	Filter      *RepoFilter
	PageOpts
}

type UserSpacesReq struct {
	SDK         string `json:"sdk"`
	Owner       string `json:"owner"`
	CurrentUser string `json:"current_user"`
	Filter      *RepoFilter
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
	UserSkillsReq          = UserDatasetsReq
)

type PageOpts struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type OffsetPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// UserChoice contains only identifiers and display fields needed by administrator selectors.
// It intentionally excludes account contact details, roles, tags, and authentication metadata.
type UserChoice struct {
	ID       int64  `json:"id"`
	UUID     string `json:"uuid"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// UserChoiceDetail adds only active personal namespace identifiers needed to issue a voucher.
type UserChoiceDetail struct {
	UserChoice
	Namespaces []NamespaceChoice `json:"namespaces"`
}

// UserChoiceList preserves the existing nested pagination envelope for user selectors.
type UserChoiceList struct {
	Data  []UserChoice `json:"data"`
	Total int          `json:"total"`
}

// ContactRiskStatus describes a contact's current match against risk rules.
type ContactRiskStatus string

const (
	ContactRiskNormal     ContactRiskStatus = "normal"
	ContactRiskSuspicious ContactRiskStatus = "suspicious"
	ContactRiskMissing    ContactRiskStatus = "missing"
	ContactRiskUnknown    ContactRiskStatus = "unknown"
)

// User contains visible profile data and authorization facts computed by the user service.
type User struct {
	ID                int64             `json:"id,omitempty"`
	Username          string            `json:"username"`
	Nickname          string            `json:"nickname"`
	Phone             string            `json:"phone,omitempty"`
	PhoneArea         string            `json:"phone_area,omitempty"`
	Email             string            `json:"email,omitempty"`
	EmailRiskStatus   ContactRiskStatus `json:"email_risk_status,omitempty"`
	PhoneRiskStatus   ContactRiskStatus `json:"phone_risk_status,omitempty"`
	UUID              string            `json:"uuid,omitempty"`
	Avatar            string            `json:"avatar,omitempty"`
	Bio               string            `json:"bio,omitempty"`
	Homepage          string            `json:"homepage,omitempty"`
	Roles             []string          `json:"roles,omitempty"`
	LastLoginAt       string            `json:"last_login_at,omitempty"`
	Orgs              []Organization    `json:"orgs,omitempty"`
	CanChangeUserName bool              `json:"can_change_username,omitempty"`
	VerifyStatus      string            `json:"verify_status,omitempty"`
	Labels            []string          `json:"labels,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	Tags              []RepoTag         `json:"tags,omitempty"`
	Namespaces        []Namespace       `json:"namespaces,omitempty"`
	// Admin identifies membership in an existing administrator role, regardless of its grants.
	Admin bool `json:"is_admin,omitempty"`
	// SuperAdmin identifies an active role assignment granting the global wildcard permission.
	SuperAdmin bool `json:"is_super_admin,omitempty"`
}

// IsSuperAdmin reports global administrator authorization verified by the user service.
// Role names alone never grant the unrestricted access required by legacy consumers.
func (u User) IsSuperAdmin() bool {
	return u.SuperAdmin
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
	AccessTokenAppGit       AccessTokenApp = "git"
	AccessTokenAppCSGHub                   = AccessTokenAppGit
	AccessTokenAppMirror    AccessTokenApp = "mirror"
	AccessTokenAppStarship  AccessTokenApp = "starship"
	AccessTokenAppAIGateway AccessTokenApp = "aigateway"
)

type AccessTokenType string

const (
	AccessTokenTypeBuiltIn AccessTokenType = "builtin"
	AccessTokenTypeOwner   AccessTokenType = "owner"
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

// RiskCheckAction identifies the authentication transition casdoor is about to
// complete when it calls the risk-check endpoint.
type RiskCheckAction string

const (
	RiskCheckActionSignin RiskCheckAction = "signin"
	RiskCheckActionSignup RiskCheckAction = "signup"
	// RiskCheckActionRequest gates resource requests such as sending email
	// or SMS verification codes; it consults the block_request switches only.
	RiskCheckActionRequest RiskCheckAction = "request"
	// RiskCheckActionUpdate gates in-process profile contact updates (email
	// or phone rebinding on starhub-server); it consults the block_signup
	// switches, since binding a new contact brings it into the system the
	// same way registration does.
	RiskCheckActionUpdate RiskCheckAction = "update"
)

// RiskCheck reason codes returned to the SSO provider.
const (
	// RiskCheckReasonOK indicates the transition is allowed.
	RiskCheckReasonOK = "ok"
	// RiskCheckReasonDisposableEmail blocks a registration whose email domain is
	// on the disposable email list.
	RiskCheckReasonDisposableEmail = "disposable_email"
	// RiskCheckReasonVirtualPhone blocks a registration whose phone number falls
	// under a virtual phone prefix.
	RiskCheckReasonVirtualPhone = "virtual_phone"
	// RiskCheckReasonIPRegistrationLimit denies a sign-up because the source
	// IP has exceeded the configured registration limit within the window.
	RiskCheckReasonIPRegistrationLimit = "ip_registration_limit"
	// RiskCheckReasonUsernameExists denies a sign-up whose username is already
	// taken in the users table, including soft-deleted accounts.
	RiskCheckReasonUsernameExists = "username_exists"
	// RiskCheckReasonEmailExists denies a sign-up whose email is already
	// taken in the users table, including soft-deleted accounts.
	RiskCheckReasonEmailExists = "email_exists"
	// RiskCheckReasonPhoneExists denies a sign-up whose phone number is
	// already taken in the users table, including soft-deleted accounts.
	RiskCheckReasonPhoneExists = "phone_exists"
	// RiskCheckReasonPolicyDenied denies the transition because no risk rule
	// matched and the configured default policy is deny.
	RiskCheckReasonPolicyDenied = "policy_denied"
)

// RiskCheckRequest is the identity snapshot casdoor sends before completing a
// third-party sign-in or sign-up.
type RiskCheckRequest struct {
	Provider   string          `json:"provider" binding:"required"`
	ProviderID string          `json:"provider_id" binding:"required"`
	Username   string          `json:"username"`
	Email      string          `json:"email"`
	Phone      string          `json:"phone"`
	PhoneArea  string          `json:"phone_area"`
	ClientIP   string          `json:"client_ip"`
	Action     RiskCheckAction `json:"action" binding:"required,oneof=signin signup request update"`
}

// RiskCheckResult is the allow/deny decision returned to casdoor. Matched*
// fields are populated when the check links the identity to an existing
// account; TraceID carries the distributed tracing ID for cross-service
// correlation in Grafana/Loki.
type RiskCheckResult struct {
	Allowed         bool   `json:"allowed"`
	ReasonCode      string `json:"reason_code"`
	Message         string `json:"message"`
	TraceID         string `json:"trace_id"`
	MatchedUsername string `json:"matched_username,omitempty"`
	MatchedUUID     string `json:"matched_uuid,omitempty"`
}

// RiskCheckHitFilter queries persisted risk-check decisions.
type RiskCheckHitFilter struct {
	// Decision filters by outcome: "allow" or "deny". Empty matches both.
	Decision string `json:"decision" binding:"omitempty,oneof=allow deny"`
	// Action filters by transition: "signin", "signup", or "request". Empty matches all.
	Action string `json:"action" binding:"omitempty,oneof=signin signup request update"`
	// Search matches trace_id, matched_rule, email, phone, or provider.
	Search string `json:"search"`
	Per    int    `json:"per"`
	Page   int    `json:"page"`
}

// RiskCheckHit is one persisted risk-check decision for the administration
// API.
type RiskCheckHit struct {
	TraceID     string    `json:"trace_id"`
	Provider    string    `json:"provider"`
	ProviderID  string    `json:"provider_id,omitempty"`
	Action      string    `json:"action"`
	Decision    string    `json:"decision"`
	ReasonCode  string    `json:"reason_code"`
	CheckCode   string    `json:"check_code"`
	MatchedRule string    `json:"matched_rule"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	ClientIP    string    `json:"client_ip"`
	CreatedAt   time.Time `json:"created_at"`
}

// RiskCheckPolicy is the runtime global policy for the risk-check endpoint:
// the kill switch and the default behavior when no rule matches. Per-model
// scene switches live in RiskCheckModel. The zero value disables the check;
// the seeded defaults keep the check enabled with fail-open behavior.
type RiskCheckPolicy struct {
	Enabled      bool      `json:"enabled"`
	DefaultAllow bool      `json:"default_allow"`
	UpdatedAt    time.Time `json:"updated_at"`
	UpdatedBy    string    `json:"updated_by"`
}

// UpdateRiskCheckPolicyReq updates the global policy. UpdatedBy is filled in
// by the server from the authenticated administrator, not from the body.
type UpdateRiskCheckPolicyReq struct {
	Enabled      bool   `json:"enabled"`
	DefaultAllow bool   `json:"default_allow"`
	UpdatedBy    string `json:"-"`
}

// RiskCheckItem is the per-check configuration for the risk-check endpoint.
// One row per risk check item; the scene switches select which transitions the
// check applies to. BlockRequest is a reserved switch with no consumer yet.
type RiskCheckItem struct {
	CheckCode    string    `json:"check_code"`
	Enabled      bool      `json:"enabled"`
	BlockSignin  bool      `json:"block_signin"`
	BlockSignup  bool      `json:"block_signup"`
	BlockRequest bool      `json:"block_request"`
	Threshold    int       `json:"threshold"`
	UpdatedAt    time.Time `json:"updated_at"`
	UpdatedBy    string    `json:"updated_by"`
}

// UpdateRiskCheckItemReq updates one check item's switches. The display name
// is seed-owned and cannot be changed. UpdatedBy is filled in by the server
// from the authenticated administrator, not from the body.
type UpdateRiskCheckItemReq struct {
	Enabled      bool   `json:"enabled"`
	BlockSignin  bool   `json:"block_signin"`
	BlockSignup  bool   `json:"block_signup"`
	BlockRequest bool   `json:"block_request"`
	Threshold    int    `json:"threshold"`
	UpdatedBy    string `json:"-"`
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

type RefreshTokenReq struct {
	NSUUID       string    `json:"ns_uuid"`
	OpUUID       string    `json:"op_uuid"`
	Username     string    `json:"username"`
	TokenName    string    `json:"token_name"`
	App          string    `json:"app"`
	NewExpiredAt time.Time `json:"new_expired_at"`
}

// IdentityStartRequest contains provider-neutral identity verification input.
// Sensitive fields must never be logged or serialized in API responses.
type IdentityStartRequest struct {
	OrderNo        string
	RealName       string
	IDNumber       string
	ReturnURL      string
	VerificationID string
}

// IdentityStartResult identifies a flow and its browser entry point.
type IdentityStartResult struct {
	VerificationID string
	URL            string
}

// IdentityQueryResult distinguishes a verified identity from an unfinished flow.
// A negative result is not terminal: providers may report it before interaction.
type IdentityQueryResult struct {
	Passed  bool
	Expired bool
}

// IdentityCreateRequest is submitted by the authenticated account owner.
type IdentityCreateRequest struct {
	RealName string `json:"real_name" binding:"required"`
	IDNumber string `json:"id_number" binding:"required"`
}

// IdentityResponse exposes only flow metadata, never credentials or identity data.
type IdentityResponse struct {
	ID         int64        `json:"attempt_id"`
	Provider   string       `json:"provider"`
	Status     VerifyStatus `json:"status"`
	CertifyURL string       `json:"certify_url,omitempty"`
	ExpiresAt  *time.Time   `json:"expires_at,omitempty"`
	Retryable  bool         `json:"retryable"`
	// Message contains a USER-ERR code until httpbase.OK localizes the response.
	Message string `json:"message,omitempty" i18n:"error"`
}

// RiskEmailDomainRule is one disposable email domain rule for the
// administration API. Disabling a rule also stops the registration reward
// deferral for the same domain because both consumers share the table.
type RiskEmailDomainRule struct {
	Domain    string    `json:"domain"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// RiskPhonePrefixRule is one virtual phone prefix rule for the
// administration API.
type RiskPhonePrefixRule struct {
	PhoneArea string    `json:"phone_area"`
	Prefix    string    `json:"prefix"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// RiskRuleListFilter queries rule rows.
type RiskRuleListFilter struct {
	Search  string `form:"search"`
	Enabled string `form:"enabled" binding:"omitempty,oneof=true false"`
	Per     int    `form:"per"`
	Page    int    `form:"page"`
}

// BulkCreateEmailDomainReq imports email domain rules; duplicates are skipped.
type BulkCreateEmailDomainReq struct {
	Domains []string `json:"domains" binding:"required,min=1"`
}

// BulkCreatePhonePrefixReq imports phone prefix rules; duplicates are skipped.
type BulkCreatePhonePrefixReq struct {
	PhoneArea string   `json:"phone_area" binding:"required"`
	Prefixes  []string `json:"prefixes" binding:"required,min=1"`
}

// SetRiskRuleEnabledReq enables or disables one rule.
type SetRiskRuleEnabledReq struct {
	Enabled bool `json:"enabled"`
}

// BulkCreateResult reports the import outcome.
type BulkCreateResult struct {
	Inserted int `json:"inserted"`
	Skipped  int `json:"skipped"`
}
