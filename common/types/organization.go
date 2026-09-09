package types

import (
	"time"

	"github.com/google/uuid"
)

const (
	// OrganizationUnitMaxDepth limits a department tree to a predictable depth.
	OrganizationUnitMaxDepth = 32
)

// OrganizationUnit is the public representation of a child organization in a hierarchy.
type OrganizationUnit struct {
	// OrganizationID is the internal database identifier used to load organization relations.
	OrganizationID int64 `json:"-"`
	// UUID is the stable UUID of the child organization and hierarchy unit.
	UUID string `json:"uuid"`
	// RootOrganizationUUID identifies the top-level organization that owns the tree.
	RootOrganizationUUID string `json:"root_organization_uuid"`
	// OrganizationUUID identifies the real child organization represented by this unit.
	OrganizationUUID string     `json:"organization_uuid"`
	ParentUnitUUID   *string    `json:"parent_unit_uuid"`
	Name             string     `json:"name"`
	Nickname         string     `json:"nickname"`
	Description      string     `json:"description"`
	Homepage         string     `json:"homepage,omitempty"`
	Logo             string     `json:"logo,omitempty"`
	Verified         bool       `json:"verified"`
	OrgType          string     `json:"org_type,omitempty"`
	UserID           int64      `json:"user_id,omitempty"`
	IsRoot           bool       `json:"is_root"`
	IsUnit           bool       `json:"is_unit"`
	Namespace        *Namespace `json:"namespace,omitempty"`
	SortOrder        int        `json:"sort_order"`
	Depth            int        `json:"depth"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	// Tags contains the organization-scoped tags assigned to this hierarchy unit.
	Tags []RepoTag `json:"tags"`
}

// OrganizationUnitSummary augments a department list item with direct-child and subtree-member counts.
type OrganizationUnitSummary struct {
	OrganizationUnit
	DirectChildrenCount int `json:"direct_children_count"`
	SubtreeMemberCount  int `json:"subtree_member_count"`
}

// CreateOrganizationUnitReq contains organization metadata and structural placement.
type CreateOrganizationUnitReq struct {
	OrganizationUUID string  `json:"-"`
	ParentUnitUUID   *string `json:"parent_unit_uuid"`
	Name             string  `json:"name" example:"child_org" binding:"required,lt=30"`
	Nickname         string  `json:"nickname" example:"Child Organization" binding:"lt=30"`
	Description      string  `json:"description" example:"child organization description" binding:"lt=500"`
	Homepage         string  `json:"homepage,omitempty" example:"https://www.example.com" binding:"lt=100"`
	Logo             string  `json:"logo,omitempty" example:"https://www.example.com/logo.png"`
	Verified         bool    `json:"verified" example:"false"`
	OrgType          string  `json:"org_type" example:"company or school etc"`
	TagIDs           []int64 `json:"tag_ids,omitempty"`
	SortOrder        int     `json:"sort_order"`
	CurrentUser      string  `json:"-"`
}

// CreateOrganizationUnitReq implements SensitiveRequestV2.
var _ SensitiveRequestV2 = (*CreateOrganizationUnitReq)(nil)

// GetSensitiveFields returns child organization text fields requiring moderation.
func (r *CreateOrganizationUnitReq) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{Name: "name", Value: func() string { return r.Name }, Scenario: "nickname_detection"},
		{Name: "nickname", Value: func() string { return r.Nickname }, Scenario: "nickname_detection"},
		{Name: "description", Value: func() string { return r.Description }, Scenario: "comment_detection"},
		{Name: "homepage", Value: func() string { return r.Homepage }, Scenario: "chat_detection"},
	}
}

// UpdateOrganizationUnitReq contains mutable root or child organization metadata and sort order.
type UpdateOrganizationUnitReq struct {
	UnitUUID string `json:"-"`
	// Name is accepted for compatibility with the original department API and updates the display name.
	Name        *string `json:"name"`
	Nickname    *string `json:"nickname"`
	Description *string `json:"description"`
	Homepage    *string `json:"homepage,omitempty"`
	Logo        *string `json:"logo,omitempty"`
	Verified    *bool   `json:"verified"`
	OrgType     *string `json:"org_type"`
	TagIDs      []int64 `json:"tag_ids,omitempty"`
	SortOrder   *int    `json:"sort_order"`
	CurrentUser string  `json:"-"`
}

// UpdateOrganizationUnitReq implements SensitiveRequestV2.
var _ SensitiveRequestV2 = (*UpdateOrganizationUnitReq)(nil)

// GetSensitiveFields returns changed organization text fields requiring moderation.
func (r *UpdateOrganizationUnitReq) GetSensitiveFields() []SensitiveField {
	fields := make([]SensitiveField, 0, 3)
	if r.Nickname != nil {
		fields = append(fields, SensitiveField{Name: "nickname", Value: func() string { return *r.Nickname }, Scenario: "nickname_detection"})
	}
	if r.Description != nil {
		fields = append(fields, SensitiveField{Name: "description", Value: func() string { return *r.Description }, Scenario: "comment_detection"})
	}
	if r.Homepage != nil {
		fields = append(fields, SensitiveField{Name: "homepage", Value: func() string { return *r.Homepage }, Scenario: "chat_detection"})
	}
	return fields
}

// DeleteOrganizationUnitReq identifies a department subtree deletion.
type DeleteOrganizationUnitReq struct {
	UnitUUID    string `json:"-"`
	CurrentUser string `json:"-"`
}

// DeleteRootOrganizationReq identifies a top-level organization hierarchy deletion.
type DeleteRootOrganizationReq struct {
	OrganizationUUID string `json:"-"`
	CurrentUser      string `json:"-"`
}

// ListOrganizationUnitReq contains tree list filters and pagination.
type ListOrganizationUnitReq struct {
	OrganizationUUID string `json:"-"`
	CurrentUser      string `json:"-"`
	Per              int    `json:"-"`
	Page             int    `json:"-"`
}

// OrganizationUnitSummaryListResp is a paginated one-level department summary result.
type OrganizationUnitSummaryListResp struct {
	Data  []OrganizationUnitSummary `json:"data"`
	Total int                       `json:"total"`
}

// OrganizationHierarchyRelationship identifies one direct parent-child organization edge.
type OrganizationHierarchyRelationship struct {
	ParentOrganizationUUID string `json:"-"`
	ChildOrganizationUUID  string `json:"-"`
}

// OrganizationReBACCleanup contains direct tuples that must be removed after an organization is deleted.
type OrganizationReBACCleanup struct {
	OrganizationUUID string   `json:"-"`
	NamespaceUUID    string   `json:"-"`
	UserUUIDs        []string `json:"-"`
}

// DeleteOrganizationUnitResp reports the impact of deleting a department subtree.
type DeleteOrganizationUnitResp struct {
	OrganizationsDeleted int `json:"organizations_deleted"`
	UnitsDeleted         int `json:"units_deleted"`
	UnitMembersRemoved   int `json:"unit_members_removed"`
	UsersAffected        int `json:"users_affected"`
	// DeletedOrganizationUUIDs is used internally for SSO cleanup and is not exposed by the API.
	DeletedOrganizationUUIDs []string `json:"-"`
	// DeletedHierarchyRelationships contains the direct edges removed with this subtree.
	DeletedHierarchyRelationships []OrganizationHierarchyRelationship `json:"-"`
	// DeletedReBACRelationships contains direct member and namespace tuples removed with this subtree.
	DeletedReBACRelationships []OrganizationReBACCleanup `json:"-"`
}

// DeleteRootOrganizationResp reports the impact of deleting an entire organization hierarchy.
type DeleteRootOrganizationResp struct {
	OrganizationsDeleted int `json:"organizations_deleted"`
	UnitsDeleted         int `json:"units_deleted"`
	MembersRemoved       int `json:"members_removed"`
	UsersAffected        int `json:"users_affected"`
	// DeletedOrganizationUUIDs is used internally for SSO cleanup and is not exposed by the API.
	DeletedOrganizationUUIDs []string `json:"-"`
	// DeletedHierarchyRelationships contains every direct edge removed with this hierarchy.
	DeletedHierarchyRelationships []OrganizationHierarchyRelationship `json:"-"`
	// DeletedReBACRelationships contains direct member and namespace tuples removed with this hierarchy.
	DeletedReBACRelationships []OrganizationReBACCleanup `json:"-"`
	// AlreadyDeleted makes repeated deletion idempotent and is not exposed by the API.
	AlreadyDeleted bool `json:"-"`
}

// AddOrganizationMembersReq creates or updates direct organization memberships.
type AddOrganizationMembersReq struct {
	OrganizationUUID string   `json:"-"`
	UserUUIDs        []string `json:"user_uuids" binding:"required,min=1,max=100,dive,required"`
	Role             UserRole `json:"role" binding:"required,oneof=read write admin"`
	CurrentUser      string   `json:"-"`
}

// RemoveOrganizationMembersReq removes direct memberships from one organization.
type RemoveOrganizationMembersReq struct {
	OrganizationUUID string   `json:"-"`
	UserUUIDs        []string `json:"user_uuids" binding:"required,min=1,max=100,dive,required"`
	CurrentUser      string   `json:"-"`
}

// UpdateOrganizationMemberRoleReq changes one user's direct role in one organization.
type UpdateOrganizationMemberRoleReq struct {
	OrganizationUUID string   `json:"-"`
	UserUUID         string   `json:"-"`
	Role             UserRole `json:"role" binding:"required,oneof=read write admin"`
	CurrentUser      string   `json:"-"`
}

// UpdateOrganizationMemberRoleResp reports the updated direct organization role.
type UpdateOrganizationMemberRoleResp struct {
	OrganizationUUID string   `json:"organization_uuid"`
	UserID           int64    `json:"user_id"`
	Role             UserRole `json:"role"`
}

// OrganizationMemberMutationResp reports an idempotent batch mutation.
type OrganizationMemberMutationResp struct {
	MembersAdded   []string `json:"members_added,omitempty"`
	MembersRemoved []string `json:"members_removed,omitempty"`
	Skipped        []string `json:"skipped"`
}

// OrganizationMember is a user returned from an organization membership query.
type OrganizationMember struct {
	UserUUID string   `json:"user_uuid"`
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	Email    string   `json:"email"`
	Avatar   string   `json:"avatar"`
	UserRole UserRole `json:"user_role"`
}

// OrganizationMemberListResp is a paginated organization member result.
type OrganizationMemberListResp struct {
	Data  []OrganizationMember `json:"data"`
	Total int                  `json:"total"`
}

// ListOrganizationMembersReq identifies an organization-wide member query.
type ListOrganizationMembersReq struct {
	OrganizationUUID string `json:"-"`
	CurrentUser      string `json:"-"`
	Search           string `json:"-"`
	Role             string `json:"-"`
	Per              int    `json:"-"`
	Page             int    `json:"-"`
}

type CreateOrgReq struct {
	// Org unique identifier
	Name string `json:"name" example:"org_name_1" binding:"lt=30"`
	// Display name
	Nickname    string  `json:"nickname" example:"org_display_name" binding:"lt=30"`
	Description string  `json:"description" example:"org description" binding:"lt=500"`
	Username    string  `json:"-"`
	Homepage    string  `json:"homepage,omitempty" example:"https://www.example.com" binding:"lt=100"`
	Logo        string  `json:"logo,omitempty" example:"https://www.example.com/logo.png"`
	Verified    bool    `json:"verified" example:"false"`
	OrgType     string  `json:"org_type" example:"company or school etc"`
	TagIDs      []int64 `json:"tag_ids,omitempty"`
}

// CreateOrgReq implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*CreateOrgReq)(nil)

func (c *CreateOrgReq) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name:     "name",
			Value:    func() string { return c.Name },
			Scenario: "nickname_detection",
		},
		{
			Name:     "nickname",
			Value:    func() string { return c.Nickname },
			Scenario: "nickname_detection",
		},
		{
			Name:     "description",
			Value:    func() string { return c.Description },
			Scenario: "comment_detection",
		},
		{
			Name:     "homepage",
			Value:    func() string { return c.Homepage },
			Scenario: "chat_detection",
		},
	}
}

type EditOrgReq struct {
	// Display name
	Nickname    *string `json:"nickname" example:"org display name"`
	Description *string `json:"description" example:"org description"`
	// TODO:rename json field name to 'name", need to negotiate with Portal engineer
	// Org unique identifier
	Name        string  `json:"-"`
	Homepage    *string `json:"homepage,omitempty" example:"https://www.example.com"`
	Logo        *string `json:"logo,omitempty" example:"https://www.example.com/logo.png"`
	Verified    *bool   `json:"verified" example:"false"`
	OrgType     *string `json:"org_type" example:"company or school etc"`
	CurrentUser string  `json:"-"`
	TagIDs      []int64 `json:"tag_ids,omitempty"`
}

// EditOrgReq implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*EditOrgReq)(nil)

func (e *EditOrgReq) GetSensitiveFields() []SensitiveField {
	var fields []SensitiveField
	if e.Nickname != nil {
		fields = append(fields, SensitiveField{
			Name: "nickname",
			Value: func() string {
				return *e.Nickname
			},
			Scenario: "nickname_detection",
		})
	}
	if e.Description != nil {
		fields = append(fields, SensitiveField{
			Name: "description",
			Value: func() string {
				return *e.Description
			},
			Scenario: "comment_detection",
		})
	}
	if e.Homepage != nil {
		fields = append(fields, SensitiveField{
			Name: "homepage",
			Value: func() string {
				return *e.Homepage
			},
			Scenario: "chat_detection",
		})
	}
	return fields
}

type ListUserOrgsReq struct {
	Username     string `json:"-"`
	Search       string `form:"search" json:"search,omitempty"`
	OrgType      string `form:"org_type" json:"org_type,omitempty"`
	VerifyStatus string `form:"verify_status" json:"verify_status,omitempty"`
	Role         string `form:"role" json:"role,omitempty"`
	Tag          string `form:"tag" json:"tag,omitempty"`
	Per          int    `form:"per" json:"per,omitempty"`
	Page         int    `form:"page" json:"page,omitempty"`
}

type DeleteOrgReq struct {
	Name        string `json:"-"`
	CurrentUser string `json:"-"`
}

type OrgDatasetsReq struct {
	// org name of dataset
	Namespace   string `json:"namespace"`
	CurrentUser string `json:"-"`
	PageOpts
}

type (
	OrgModelsReq      = OrgDatasetsReq
	OrgCodesReq       = OrgDatasetsReq
	OrgSpacesReq      = OrgDatasetsReq
	OrgCollectionsReq = OrgDatasetsReq
	OrgPromptsReq     = OrgDatasetsReq
	OrgMCPsReq        = OrgDatasetsReq
	OrgFinetunesReq   = OrgDatasetsReq
	OrgEvaluationsReq = OrgDatasetsReq
	OrgNotebooksReq   = OrgDatasetsReq
)

// OrgRunDeploysReq is used for listing organization run deploys (e.g. inference).
type OrgRunDeploysReq struct {
	Namespace   string         `json:"-"`
	CurrentUser string         `json:"-"`
	RepoType    RepositoryType `json:"-"`
	DeployType  int            `json:"-"`
	PageOpts
}

type Organization struct {
	// unique name of the organization
	Name        string `json:"path"`
	Nickname    string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
	Logo        string `json:"logo,omitempty"`
	OrgType     string `json:"org_type,omitempty"`
	Verified    bool   `json:"verified"`
	// IsRoot reports whether this is a top-level organization.
	IsRoot bool `json:"is_root"`
	// IsUnit reports whether this organization belongs to the hierarchy model.
	IsUnit       bool       `json:"is_unit"`
	UserID       int64      `json:"user_id,omitempty"`
	VerifyStatus string     `json:"verify_status,omitempty"`
	UUID         uuid.UUID  `json:"uuid,omitempty"`
	Namespace    *Namespace `json:"namespace,omitempty"`
	Role         string     `json:"role,omitempty"`
	Tags         []RepoTag  `json:"tags,omitempty"`
}

type Member struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	UUID        string `json:"uuid"`
	Avatar      string `json:"avatar,omitempty"`
	Role        string `json:"role,omitempty"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

type OrgVerifyReq struct {
	Name               string `json:"path" binding:"required"`
	CompanyName        string `json:"company_name" binding:"required"`
	UnifiedCreditCode  string `json:"unified_credit_code" binding:"required"`
	ContactName        string `json:"contact_name" binding:"required"`
	ContactEmail       string `json:"contact_email" binding:"required"`
	BusinessLicenseImg string `json:"business_license_img" binding:"required"`
	Username           string `json:"username"`
	Status             string `json:"status"`
	UserUUID           string `json:"user_uuid"`
}

type OrgVerifyStatusReq struct {
	Status VerifyStatus `json:"status" binding:"required"` // approved,  rejected
	Reason string       `json:"reason"`
}
