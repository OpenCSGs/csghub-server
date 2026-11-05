package types

import "strings"

type LicenseProduct string

const (
	LicenseProductCSGHub  LicenseProduct = "CSGHub"
	LicenseProductCSGLite LicenseProduct = "CSGLite"
)

func ParseLicenseProduct(skuType string) (LicenseProduct, bool) {
	for _, product := range []LicenseProduct{LicenseProductCSGLite} {
		if skuType == strings.ToLower(string(product)) {
			return product, true
		}
	}
	return "", false
}

type CustomerLicenseDownloadReq struct {
	UserUUID string
	Product  LicenseProduct
}

type LicenseDownloadResp struct {
	Data    LicenseDownloadData `json:"data"`
	License string              `json:"license"`
}

type LicenseDownloadData struct {
	ID       int64  `json:"id"`
	Key      string `json:"key"`
	UserUUID string `json:"user_uuid"`
	DataBody
}
