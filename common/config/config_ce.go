//go:build !ee && !saas

package config

import "opencsg.com/csghub-server/common/types"

// SupportsMultipleAdminRoles reports whether this edition supports module administrator roles.
func SupportsMultipleAdminRoles() bool {
	return false
}

// PlatformAdminPermissions maps validated administrator requirements to the edition's grant policy.
// Callers retain the original requirements for validation and authorization context semantics.
func PlatformAdminPermissions(_ []types.AdminPermission) []types.AdminPermission {
	return []types.AdminPermission{types.AdminPermissionAll}
}

// IsHierarchicalOrganization reports the effective organization mode in CE.
// CE always uses single-level organizations regardless of configuration.
func (cfg *Config) IsHierarchicalOrganization() bool {
	return false
}
