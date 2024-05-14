package mirrorserver

import "context"

type MirrorServer interface {
	CreateMirrorRepo(ctx context.Context, req CreateMirrorRepoReq) error
	CreatePushMirror(ctx context.Context, req CreatePushMirrorReq) error
	MirrorSync(ctx context.Context, req MirrorSyncReq) error
}
