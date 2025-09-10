package geo

import "opencsg.com/csghub-server/common/config"

var ipLocator IPLocator
var cityToCdnDomain map[string]string
var lbsServiceKey string

func Config(config *config.Config) {
	cityToCdnDomain = config.CityToCdnDomain
	lbsServiceKey = config.LBSServiceKey
}

// SetIPLocator changes the default ip locator
func SetIPLocator(locator IPLocator) {
	ipLocator = locator
}
