package component

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
)

type repositoryDeletionGit interface {
	DeleteRepo(ctx context.Context, relativePath string) error
}

type repositoryDeletionMirrorTaskFinder interface {
	FindCurrentMirrorTaskID(ctx context.Context, repositoryID int64) (int64, error)
}

type repositoryDeletionMirrorCanceler interface {
	CancelMirror(ctx context.Context, taskID int64) error
}

// RepositoryDeletionResourceCleaner removes slow storage resources (including
// LFS objects and generated repository packages) before database finalization.
type RepositoryDeletionResourceCleaner interface {
	DeleteRepositoryResources(ctx context.Context, args workhub.RepositoryDeletionArgs) error
}

// RepositoryDeletionWorker executes the retryable, asynchronous half of a
// repository deletion. Database finalization deliberately runs last.
type RepositoryDeletionWorker struct {
	river.WorkerDefaults[workhub.RepositoryDeletionArgs]
	git            repositoryDeletionGit
	resources      RepositoryDeletionResourceCleaner
	rebac          rebac.Authorizer
	authorizations database.RepositoryAuthorizationStore
	finalizer      database.RepositoryDeletionFinalizer
	mirrors        repositoryDeletionMirrorTaskFinder
	canceler       repositoryDeletionMirrorCanceler
}

func NewRepositoryDeletionWorker(
	git repositoryDeletionGit,
	resources RepositoryDeletionResourceCleaner,
	authorizer rebac.Authorizer,
	authorizations database.RepositoryAuthorizationStore,
	finalizer database.RepositoryDeletionFinalizer,
	mirrors repositoryDeletionMirrorTaskFinder,
	canceler repositoryDeletionMirrorCanceler,
) *RepositoryDeletionWorker {
	return &RepositoryDeletionWorker{
		git: git, resources: resources, rebac: authorizer, authorizations: authorizations, finalizer: finalizer,
		mirrors: mirrors, canceler: canceler,
	}
}

func (w *RepositoryDeletionWorker) Work(ctx context.Context, job *river.Job[workhub.RepositoryDeletionArgs]) error {
	if job == nil || job.Args.RepositoryID <= 0 {
		return fmt.Errorf("repository deletion job requires a positive repository ID")
	}
	if w.resources == nil || w.git == nil || w.rebac == nil || w.authorizations == nil || w.finalizer == nil || w.mirrors == nil || w.canceler == nil {
		return fmt.Errorf("repository deletion worker dependencies are required")
	}
	args := job.Args
	taskID, err := w.mirrors.FindCurrentMirrorTaskID(ctx, args.RepositoryID)
	if err != nil {
		return fmt.Errorf("find repository mirror task: %w", err)
	}
	if taskID > 0 {
		if err := w.canceler.CancelMirror(ctx, taskID); err != nil {
			return fmt.Errorf("cancel repository mirror task %d: %w", taskID, err)
		}
	}
	if err := w.resources.DeleteRepositoryResources(ctx, args); err != nil {
		return fmt.Errorf("delete repository resources: %w", err)
	}
	if err := w.git.DeleteRepo(ctx, args.GitalyPath); err != nil && status.Code(err) != codes.NotFound {
		return fmt.Errorf("delete Git repository: %w", err)
	}
	relationships, err := repositoryDeletionOwnerRelationships(args)
	if err != nil {
		return err
	}
	if err := w.rebac.Delete(ctx, relationships); err != nil {
		return fmt.Errorf("delete repository ReBAC relationships: %w", err)
	}
	if err := cleanRepositoryAuthorizations(ctx, w.authorizations, w.rebac, args.RepositoryID); err != nil {
		return fmt.Errorf("clean repository authorizations: %w", err)
	}
	if err := w.finalizer.FinalizeRepositoryDeletion(ctx, args.RepositoryID); err != nil {
		return fmt.Errorf("finalize repository database deletion: %w", err)
	}
	return nil
}

func (w *RepositoryDeletionWorker) Timeout(*river.Job[workhub.RepositoryDeletionArgs]) time.Duration {
	return workhub.RepositoryDeletionJobTimeout
}

// repositoryDeletionOwnerRelationships returns all namespace owner tuples that can exist for a repository.
func repositoryDeletionOwnerRelationships(args workhub.RepositoryDeletionArgs) ([]rebac.Relationship, error) {
	if args.OwnerUUID == "" {
		return nil, fmt.Errorf("repository deletion job requires an owner UUID")
	}
	var subject rebac.Subject
	relations := []rebac.Relation{rebac.RelationOwner}
	switch args.OwnerType {
	case database.UserNamespace:
		subject = rebac.UserSubject(args.OwnerUUID)
	case database.OrgNamespace:
		subject = rebac.NewSubject(rebac.ObjectTypeOrganization, args.OwnerUUID)
		relations = []rebac.Relation{rebac.RelationOrganization, rebac.RelationOrganizationDirect}
	default:
		return nil, fmt.Errorf("unsupported repository owner type %q", args.OwnerType)
	}
	relationships := make([]rebac.Relationship, 0, len(relations))
	for _, relation := range relations {
		relationships = append(relationships, rebac.Relationship{
			Subject: subject, Relation: relation, Object: rebac.RepositoryObject(args.RepositoryID),
		})
	}
	return relationships, nil
}
