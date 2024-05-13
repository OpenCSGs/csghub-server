package mirrorserver

import "context"

type MirrorServer interface {
	CreateMirrorRepo(ctx context.Context, req CreateMirrorRepoReq) error
}
