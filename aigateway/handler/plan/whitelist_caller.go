package plan

import (
	"opencsg.com/csghub-server/api/httpbase"
	commontypes "opencsg.com/csghub-server/common/types"

	"github.com/gin-gonic/gin"
)

// ModelCallerFromGin derives the caller identity from the authenticated
// request context. The auth flavor decides how the identity is shaped:
//
//   - UserOrgApiKey: the api key resolves to a namespace uuid — whitelist
//     grants match by direct equality against that uuid.
//   - JWT / personal access tokens / OAuth: a personal identity — whitelist
//     grants match the user's own namespace uuid plus the namespace uuids of
//     the organizations the user belongs to (expanded inline by the SQL).
//   - SystemApiKey: the trusted platform token sees every model.
//   - Anything without a user uuid is anonymous and only sees public models.
//
// Every flavor carries the upstream channel priority the gateway user-tags
// middleware resolved from the user's routing tags (nil for the platform key
// and anonymous callers, i.e. unrestricted); upstream channel routing reads
// it.
func ModelCallerFromGin(c *gin.Context) *commontypes.GatewayWhitelistCaller {
	switch httpbase.GetAuthType(c) {
	case httpbase.AuthTypeSystemApiKey:
		return &commontypes.GatewayWhitelistCaller{Bypass: true}
	case httpbase.AuthTypeUserOrgApiKey:
		nsUUID := httpbase.GetCurrentNamespaceUUID(c)
		if nsUUID == "" {
			nsUUID = httpbase.GetCurrentUserUUID(c)
		}
		return &commontypes.GatewayWhitelistCaller{
			NsUUID:        nsUUID,
			NamespaceUUID: nsUUID,
			UserPriority:  httpbase.GetCurrentUserPriority(c),
		}
	default:
		nsUUID := httpbase.GetCurrentNamespaceUUID(c)
		if nsUUID == "" {
			nsUUID = httpbase.GetCurrentUserUUID(c)
		}
		return &commontypes.GatewayWhitelistCaller{
			NsUUID:       nsUUID,
			UserUUID:     httpbase.GetCurrentUserUUID(c),
			UserPriority: httpbase.GetCurrentUserPriority(c),
		}
	}
}
