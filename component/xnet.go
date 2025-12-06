package component

import (
	"context"
	"net/url"
	"time"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type XnetComponent interface {
	XnetToken(ctx context.Context, req *types.XnetTokenReq) (*types.XnetTokenResp, error)
	PresignedGetObject(ctx context.Context, objectKey string, expireTime time.Duration, params url.Values) (*url.URL, error)
}

type XnetComponentImpl struct {
	repoStore      database.RepoStore
	xnetClient     rpc.XnetSvcClient
	userStore      database.UserStore
	namespaceStore database.NamespaceStore
	repoComp       RepoComponent
}
