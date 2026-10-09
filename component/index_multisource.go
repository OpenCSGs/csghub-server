//go:build !saas

package component

import "opencsg.com/csghub-server/common/types"

func indexMultiSource(hfPath, msPath, csgPath string) types.MultiSource {
	return types.MultiSource{
		HFPath:  hfPath,
		MSPath:  msPath,
		CSGPath: csgPath,
	}
}
