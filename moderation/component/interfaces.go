package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type RepoComponent interface {
	UpdateRepoSensitiveCheckStatus(ctx context.Context, repoType types.RepositoryType, namespace string, name string, status types.SensitiveCheckStatus) error
	CheckRepoFiles(ctx context.Context, repoType types.RepositoryType, namespace string, name string, options CheckOption) error
	CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error)
}

type RepoFileComponent interface {
	GenRepoFileRecords(ctx context.Context, repoType types.RepositoryType, namespace, name string) error
	GenRepoFileRecordsBatch(ctx context.Context, repoType types.RepositoryType, lastRepoID int64, concurrency int) error
	DetectRepoSensitiveCheckStatus(ctx context.Context, repoType types.RepositoryType, namespace, name string) error
}

type SensitiveWordSetComponent interface {
	Index(ctx context.Context, search string) ([]types.SensitiveWordSet, error)
	Get(ctx context.Context, id int64) (*types.SensitiveWordSet, error)
	Create(ctx context.Context, input types.CreateSensitiveWordSetReq) error
	Update(ctx context.Context, input types.UpdateSensitiveWordSetReq) error
}
