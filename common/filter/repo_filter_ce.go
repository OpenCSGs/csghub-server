//go:build !ee && !saas

package filter

func NewRepoFilter() RepoFilter {
	return nil
}
