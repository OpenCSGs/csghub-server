//go:build !saas

package component

import "opencsg.com/csghub-server/common/types"

func expectedIndexMultiSource(paths types.MultiSource) types.MultiSource {
	return paths
}
