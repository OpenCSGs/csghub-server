package errorx

const errFederationSitePrefix = "FS-ERR"

const (
	codeFederationSiteNotFound = iota
	codeFederationSiteOwnerOrgNotFound
	codeFederationSiteSelfReference
)

var (
	// Description: The requested federation site was not found.
	//
	// Description_ZH: 未找到请求的联邦站点。
	//
	// en-US: Federation site not found
	//
	// zh-CN: 联邦站点未找到
	//
	// zh-HK: 聯邦站點未找到
	ErrFederationSiteNotFound error = CustomError{prefix: errFederationSitePrefix, code: codeFederationSiteNotFound}

	// Description: The specified owner organization (parent site) was not found.
	//
	// Description_ZH: 指定的所属组织（父站点）未找到。
	//
	// en-US: Owner organization not found
	//
	// zh-CN: 所属组织未找到
	//
	// zh-HK: 所屬組織未找到
	ErrFederationSiteOwnerOrgNotFound error = CustomError{prefix: errFederationSitePrefix, code: codeFederationSiteOwnerOrgNotFound}

	// Description: A federation site cannot reference itself as its owner organization.
	//
	// Description_ZH: 联邦站点不能将自己设为所属组织。
	//
	// en-US: Federation site cannot reference itself as owner organization
	//
	// zh-CN: 联邦站点不能将自身设为所属组织
	//
	// zh-HK: 聯邦站點不能將自身設為所屬組織
	ErrFederationSiteSelfReference error = CustomError{prefix: errFederationSitePrefix, code: codeFederationSiteSelfReference}
)
