package errorx

import "errors"

// errOrganizationPrefix is the shared error-code prefix for organization domain errors.
const errOrganizationPrefix = "ORG-ERR"

const (
	// organizationMemberNotFound is returned when a user is not a member of the organization.
	organizationMemberNotFound = iota
	// organizationNotFound is returned when the requested organization does not exist.
	organizationNotFound
	// organizationManageForbidden is returned when the user cannot manage the organization.
	organizationManageForbidden
	// organizationAccessForbidden is returned when the user cannot access the organization.
	organizationAccessForbidden
)

var (
	// ErrOrganizationMemberNotFound indicates that the requested user has no membership in the organization.
	//
	// Description: The requested user does not have a membership in the specified organization.
	//
	// Description_ZH: 请求的用户在指定组织中不存在成员关系。
	//
	// en-US: Organization member not found
	//
	// zh-CN: 组织成员不存在
	//
	// zh-HK: 組織成員不存在
	ErrOrganizationMemberNotFound error = CustomError{prefix: errOrganizationPrefix, code: organizationMemberNotFound}

	// ErrOrganizationNotFound indicates that the requested organization does not exist.
	//
	// Description: The requested organization does not exist.
	//
	// Description_ZH: 请求的组织不存在。
	//
	// en-US: Organization not found
	//
	// zh-CN: 组织不存在
	//
	// zh-HK: 組織不存在
	ErrOrganizationNotFound error = CustomError{prefix: errOrganizationPrefix, code: organizationNotFound}

	// ErrOrganizationManageForbidden indicates that the user cannot manage the requested organization.
	//
	// Description: The user does not have permission to manage the requested organization.
	//
	// Description_ZH: 用户没有管理该组织的权限。
	//
	// en-US: You do not have permission to manage this organization
	//
	// zh-CN: 无权管理该组织
	//
	// zh-HK: 無權管理該組織
	ErrOrganizationManageForbidden error = CustomError{prefix: errOrganizationPrefix, code: organizationManageForbidden}

	// ErrOrganizationAccessForbidden indicates that the user cannot access the requested organization.
	//
	// Description: The user does not have permission to access the requested organization.
	//
	// Description_ZH: 用户没有访问该组织的权限。
	//
	// en-US: You do not have permission to access this organization
	//
	// zh-CN: 无权访问该组织
	//
	// zh-HK: 無權訪問該組織
	ErrOrganizationAccessForbidden error = CustomError{prefix: errOrganizationPrefix, code: organizationAccessForbidden}
)

// OrganizationMemberNotFound creates an organization member error with the target identifiers.
func OrganizationMemberNotFound(organizationUUID string, userID int64) error {
	return CustomError{
		prefix: errOrganizationPrefix,
		code:   organizationMemberNotFound,
		err:    errors.New("organization member does not exist"),
		context: map[string]interface{}{
			"organization_uuid": organizationUUID,
			"user_id":           userID,
		},
	}
}

// OrganizationNotFound creates an organization-not-found error with the target UUID.
func OrganizationNotFound(organizationUUID string) error {
	return CustomError{
		prefix: errOrganizationPrefix,
		code:   organizationNotFound,
		err:    errors.New("organization does not exist"),
		context: map[string]interface{}{
			"organization_uuid": organizationUUID,
		},
	}
}

// OrganizationManageForbidden creates an organization-management error with the target UUID.
func OrganizationManageForbidden(organizationUUID string) error {
	return CustomError{
		prefix: errOrganizationPrefix,
		code:   organizationManageForbidden,
		err:    errors.New("no permission to manage organization"),
		context: map[string]interface{}{
			"organization_uuid": organizationUUID,
		},
	}
}

// OrganizationAccessForbidden creates an organization-access error with the target UUID.
func OrganizationAccessForbidden(organizationUUID string) error {
	return CustomError{
		prefix: errOrganizationPrefix,
		code:   organizationAccessForbidden,
		err:    errors.New("no permission to access organization"),
		context: map[string]interface{}{
			"organization_uuid": organizationUUID,
		},
	}
}
