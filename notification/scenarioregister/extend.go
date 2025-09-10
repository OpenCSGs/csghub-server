//go:build !ee && !saas

package scenarioregister

import (
	"opencsg.com/csghub-server/notification/scenariomgr"
)

func extend(_ *scenariomgr.DataProvider) {
}
