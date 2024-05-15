package mirrorserver

import "context"

type MirrorServer interface {
	CreateMirrorRepo(ctx context.Context, req CreateMirrorRepoReq) (int64, error)
	GetMirrorTaskInfo(ctx context.Context, taskId int64) (*MirrorTaskInfo, error)
	CreatePushMirror(ctx context.Context, req CreatePushMirrorReq) error
	MirrorSync(ctx context.Context, req MirrorSyncReq) error
}
