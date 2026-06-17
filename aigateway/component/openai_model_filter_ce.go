//go:build !ee && !saas

package component

func modelListDefaultFilters() []modelFilter {
	return nil
}
