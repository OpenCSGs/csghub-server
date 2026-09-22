//go:build !ee && !saas

package config

// IsHierarchicalOrganization reports the effective organization mode in CE.
// CE always uses single-level organizations regardless of configuration.
func (cfg *Config) IsHierarchicalOrganization() bool {
	return false
}
