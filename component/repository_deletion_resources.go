package component

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	commonutil "opencsg.com/csghub-server/common/utils/common"
)

type repositoryDeletionLFSStore interface {
	FindByRepoID(ctx context.Context, repoID int64) ([]database.LfsMetaObject, error)
	ExistsByOidInActiveRepoExclRepo(ctx context.Context, oid string, repoID int64) (bool, error)
}

type repositoryDeletionResources struct {
	config   *config.Config
	lfs      repositoryDeletionLFSStore
	s3       s3.Client
	packages RepositoryPackageSyncer
}

// NewRepositoryDeletionResourceCleaner builds the storage cleanup dependency
// used by RepositoryDeletionWorker.
func NewRepositoryDeletionResourceCleaner(
	cfg *config.Config,
	lfsStore database.LfsMetaObjectStore,
	s3Client s3.Client,
	packages RepositoryPackageSyncer,
) RepositoryDeletionResourceCleaner {
	return &repositoryDeletionResources{config: cfg, lfs: lfsStore, s3: s3Client, packages: packages}
}

func (r *repositoryDeletionResources) DeleteRepositoryResources(ctx context.Context, args workhub.RepositoryDeletionArgs) error {
	if r.packages != nil {
		if err := r.packages.RemoveRepoPackages(ctx, args.RepositoryType, args.RepositoryID); err != nil {
			return fmt.Errorf("delete repository packages: %w", err)
		}
	}
	if r.config == nil || r.lfs == nil || r.s3 == nil {
		return nil
	}
	metas, err := r.lfs.FindByRepoID(ctx, args.RepositoryID)
	if err != nil {
		return fmt.Errorf("find repository LFS objects: %w", err)
	}
	objects := make(chan minio.ObjectInfo, len(metas))
	for _, meta := range metas {
		if !args.Migrated {
			exists, err := r.lfs.ExistsByOidInActiveRepoExclRepo(ctx, meta.Oid, args.RepositoryID)
			if err != nil {
				return fmt.Errorf("check shared LFS object %s: %w", meta.Oid, err)
			}
			if exists {
				continue
			}
		}
		objects <- minio.ObjectInfo{Key: commonutil.BuildLfsPath(args.RepositoryID, meta.Oid, args.Migrated)}
	}
	close(objects)
	for removeErr := range r.s3.RemoveObjects(ctx, r.config.S3.Bucket, objects, minio.RemoveObjectsOptions{}) {
		if removeErr.Err != nil {
			return fmt.Errorf("delete LFS object %s: %w", removeErr.ObjectName, removeErr.Err)
		}
	}
	return nil
}
