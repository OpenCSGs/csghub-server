package workhub

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// TestOrganizationDeletionArgsJSONIsStable verifies retained cleanup identities
// remain present in the durable River payload.
func TestOrganizationDeletionArgsJSONIsStable(t *testing.T) {
	payload, err := json.Marshal(OrganizationDeletionArgs{
		OrganizationIDs:   []int64{42},
		OrganizationUUIDs: []string{"organization-uuid"},
		DeletedHierarchyRelationships: []OrganizationHierarchyRelationshipArgs{{
			ParentOrganizationUUID: "parent-uuid", ChildOrganizationUUID: "organization-uuid",
		}},
		DeletedReBACRelationships: []OrganizationReBACCleanupArgs{{
			OrganizationUUID: "organization-uuid", NamespaceUUID: "namespace-uuid", UserUUIDs: []string{"user-uuid"},
		}},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"organization_ids":[42],
		"organization_uuids":["organization-uuid"],
		"deleted_hierarchy_relationships":[{"parent_organization_uuid":"parent-uuid","child_organization_uuid":"organization-uuid"}],
		"deleted_rebac_relationships":[{"organization_uuid":"organization-uuid","namespace_uuid":"namespace-uuid","user_uuids":["user-uuid"]}]
	}`, string(payload))
}

// TestOrganizationDeletionJobClientForwardsSnapshot verifies job routing and
// retries are configured at insertion time.
func TestOrganizationDeletionJobClientForwardsSnapshot(t *testing.T) {
	ctx := context.Background()
	jobClient := &fakeJobClient{}
	client := NewOrganizationDeletionJobClient(jobClient)
	tx := (*sql.Tx)(nil)

	jobID, err := client.InsertOrganizationDeletionJobTx(ctx, tx, database.OrganizationDeletionJobInput{
		OrganizationIDs:   []int64{42},
		OrganizationUUIDs: []string{"organization-uuid"},
		DeletedHierarchyRelationships: []types.OrganizationHierarchyRelationship{{
			ParentOrganizationUUID: "parent-uuid", ChildOrganizationUUID: "organization-uuid",
		}},
		DeletedReBACRelationships: []types.OrganizationReBACCleanup{{
			OrganizationUUID: "organization-uuid", NamespaceUUID: "namespace-uuid", UserUUIDs: []string{"user-uuid"},
		}},
	})

	require.NoError(t, err)
	require.Equal(t, int64(123), jobID)
	require.Same(t, tx, jobClient.tx)
	require.Equal(t, OrganizationDeletionQueue, jobClient.opts.Queue)
	require.Equal(t, OrganizationDeletionJobMaxAttempts, jobClient.opts.MaxAttempts)
	require.True(t, jobClient.opts.ScheduledAt.IsZero())
	args, ok := jobClient.args.(OrganizationDeletionArgs)
	require.True(t, ok)
	require.Equal(t, []int64{42}, args.OrganizationIDs)
	require.Equal(t, "parent-uuid", args.DeletedHierarchyRelationships[0].ParentOrganizationUUID)
	require.Equal(t, "namespace-uuid", args.DeletedReBACRelationships[0].NamespaceUUID)
}
