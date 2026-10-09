package workhub

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"opencsg.com/csghub-server/builder/store/database"
)

const (
	// OrganizationDeletionQueue is the dedicated queue and job kind for
	// organization SSO and ReBAC cleanup.
	OrganizationDeletionQueue = "organization_deletion"
	// OrganizationDeletionJobTimeout bounds one external cleanup attempt.
	OrganizationDeletionJobTimeout = 10 * time.Minute
	// OrganizationDeletionJobMaxAttempts keeps cleanup retryable through
	// extended SSO, ReBAC, or database outages.
	OrganizationDeletionJobMaxAttempts = 12
)

// OrganizationHierarchyRelationshipArgs is the serialized snapshot of one
// direct parent-child organization edge.
type OrganizationHierarchyRelationshipArgs struct {
	ParentOrganizationUUID string `json:"parent_organization_uuid"`
	ChildOrganizationUUID  string `json:"child_organization_uuid"`
}

// OrganizationReBACCleanupArgs is the serialized snapshot of direct member
// and namespace relationships for one organization.
type OrganizationReBACCleanupArgs struct {
	OrganizationUUID string   `json:"organization_uuid"`
	NamespaceUUID    string   `json:"namespace_uuid"`
	UserUUIDs        []string `json:"user_uuids"`
}

// OrganizationDeletionArgs contains the stable identities needed to retry all
// external cleanup without reading active organization records.
type OrganizationDeletionArgs struct {
	OrganizationIDs               []int64                                 `json:"organization_ids"`
	OrganizationUUIDs             []string                                `json:"organization_uuids"`
	DeletedHierarchyRelationships []OrganizationHierarchyRelationshipArgs `json:"deleted_hierarchy_relationships"`
	DeletedReBACRelationships     []OrganizationReBACCleanupArgs          `json:"deleted_rebac_relationships"`
}

// Kind returns the River kind for organization deletion jobs.
func (OrganizationDeletionArgs) Kind() string {
	return OrganizationDeletionQueue
}

// InsertOpts routes organization deletion jobs to their dedicated queue.
func (OrganizationDeletionArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: OrganizationDeletionQueue}
}

type organizationDeletionJobClient struct {
	jobClient JobClient
}

// NewOrganizationDeletionJobClient adapts a workhub job client to the database
// organization deletion job interface.
func NewOrganizationDeletionJobClient(jobClient JobClient) database.OrganizationDeletionJobClient {
	return organizationDeletionJobClient{jobClient: jobClient}
}

// InsertOrganizationDeletionJobTx inserts one durable cleanup job inside the
// transaction that soft-deletes the organization records.
func (c organizationDeletionJobClient) InsertOrganizationDeletionJobTx(ctx context.Context, tx *sql.Tx, input database.OrganizationDeletionJobInput) (int64, error) {
	if c.jobClient == nil {
		return 0, fmt.Errorf("workhub job client is required")
	}
	args := OrganizationDeletionArgs{
		OrganizationIDs:   input.OrganizationIDs,
		OrganizationUUIDs: input.OrganizationUUIDs,
	}
	for _, relationship := range input.DeletedHierarchyRelationships {
		args.DeletedHierarchyRelationships = append(args.DeletedHierarchyRelationships, OrganizationHierarchyRelationshipArgs{
			ParentOrganizationUUID: relationship.ParentOrganizationUUID,
			ChildOrganizationUUID:  relationship.ChildOrganizationUUID,
		})
	}
	for _, cleanup := range input.DeletedReBACRelationships {
		args.DeletedReBACRelationships = append(args.DeletedReBACRelationships, OrganizationReBACCleanupArgs{
			OrganizationUUID: cleanup.OrganizationUUID,
			NamespaceUUID:    cleanup.NamespaceUUID,
			UserUUIDs:        cleanup.UserUUIDs,
		})
	}
	slog.InfoContext(ctx, "enqueuing organization deletion job in tx",
		slog.String("queue", args.InsertOpts().Queue),
		slog.Any("organization_ids", args.OrganizationIDs),
		slog.Any("organization_uuids", args.OrganizationUUIDs),
		slog.Int("hierarchy_relationships_count", len(args.DeletedHierarchyRelationships)),
		slog.Int("rebac_cleanups_count", len(args.DeletedReBACRelationships)),
	)
	jobID, err := c.jobClient.InsertTx(ctx, tx, args, &InsertOpts{
		Queue:       args.InsertOpts().Queue,
		MaxAttempts: OrganizationDeletionJobMaxAttempts,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to enqueue organization deletion job in tx",
			slog.String("queue", args.InsertOpts().Queue),
			slog.Any("organization_uuids", args.OrganizationUUIDs),
			slog.Any("error", err),
		)
		return 0, err
	}
	slog.InfoContext(ctx, "enqueued organization deletion job in tx successfully",
		slog.Int64("job_id", jobID),
		slog.String("queue", args.InsertOpts().Queue),
		slog.Any("organization_uuids", args.OrganizationUUIDs),
	)
	return jobID, nil
}
