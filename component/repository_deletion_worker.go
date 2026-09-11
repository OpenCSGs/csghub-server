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
	git       repositoryDeletionGit
	resources RepositoryDeletionResourceCleaner
	rebac     rebac.Authorizer
	finalizer database.RepositoryDeletionFinalizer
	mirrors   repositoryDeletionMirrorTaskFinder
	canceler  repositoryDeletionMirrorCanceler
}

func NewRepositoryDeletionWorker(
	git repositoryDeletionGit,
	resources RepositoryDeletionResourceCleaner,
	authorizer rebac.Authorizer,
	finalizer database.RepositoryDeletionFinalizer,
	mirrors repositoryDeletionMirrorTaskFinder,
	canceler repositoryDeletionMirrorCanceler,
) *RepositoryDeletionWorker {
	return &RepositoryDeletionWorker{
		git: git, resources: resources, rebac: authorizer, finalizer: finalizer,
		mirrors: mirrors, canceler: canceler,
	}
}

func (w *RepositoryDeletionWorker) Work(ctx context.Context, job *river.Job[workhub.RepositoryDeletionArgs]) error {
	if job == nil || job.Args.RepositoryID <= 0 {
		return fmt.Errorf("repository deletion job requires a positive repository ID")
	}
	if w.resources == nil || w.git == nil || w.rebac == nil || w.finalizer == nil || w.mirrors == nil || w.canceler == nil {
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
	relationship, err := repositoryDeletionOwnerRelationship(args)
	if err != nil {
		return err
	}
	decision, err := w.rebac.Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return fmt.Errorf("check repository ReBAC relationship: %w", err)
	}
	if decision.Allowed {
		if err := w.rebac.Delete(ctx, []rebac.Relationship{relationship}); err != nil {
			return fmt.Errorf("delete repository ReBAC relationship: %w", err)
		}
	}
	if err := w.finalizer.FinalizeRepositoryDeletion(ctx, args.RepositoryID); err != nil {
		return fmt.Errorf("finalize repository database deletion: %w", err)
	}
	return nil
}

func (w *RepositoryDeletionWorker) Timeout(*river.Job[workhub.RepositoryDeletionArgs]) time.Duration {
	return workhub.RepositoryDeletionJobTimeout
}

func repositoryDeletionOwnerRelationship(args workhub.RepositoryDeletionArgs) (rebac.Relationship, error) {
	if args.OwnerUUID == "" {
		return rebac.Relationship{}, fmt.Errorf("repository deletion job requires an owner UUID")
	}
	var subject rebac.Subject
	var relation rebac.Relation
	switch args.OwnerType {
	case database.UserNamespace:
		subject = rebac.UserSubject(args.OwnerUUID)
		relation = rebac.RelationOwner
	case database.OrgNamespace:
		subject = rebac.NewSubject(rebac.ObjectTypeOrganization, args.OwnerUUID)
		relation = rebac.RelationOrganization
	default:
		return rebac.Relationship{}, fmt.Errorf("unsupported repository owner type %q", args.OwnerType)
	}
	return rebac.Relationship{Subject: subject, Relation: relation, Object: rebac.RepositoryObject(args.RepositoryID)}, nil
}
