package component

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestEnsureMirrorOrgNamespaceCoexistsWithHierarchyRoot verifies both creation orders
// against the database uniqueness constraint. The global database prevents parallel execution.
func TestEnsureMirrorOrgNamespaceCoexistsWithHierarchyRoot(t *testing.T) {
	for _, rootFirst := range []bool{true, false} {
		name := "mirror_before_root"
		if rootFirst {
			name = "root_before_mirror"
		}
		t.Run(name, func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			previousDB := database.GetDB()
			database.SetDB(db)
			defer database.SetDB(previousDB)
			ctx := context.Background()
			owner := &database.User{Username: "mirror-admin", UUID: uuid.NewString(), RoleMask: "admin"}
			_, err := db.Core.NewInsert().Model(owner).Exec(ctx)
			require.NoError(t, err)

			authorizer := mockrebac.NewMockAuthorizer(t)
			authorizer.EXPECT().Check(mock.Anything, mock.Anything).Return(rebac.Decision{Allowed: false}, nil)
			authorizer.EXPECT().Write(mock.Anything, mock.Anything).Return(nil)
			mirror := &mirrorComponentImpl{
				orgStore:       database.NewOrgStore(false, nil),
				namespaceStore: database.NewNamespaceStoreWithDB(db),
				userStore:      database.NewUserStoreWithDB(db),
				rebac:          authorizer,
			}
			unitStore := database.NewOrganizationUnitStoreWithDB(db)
			createRoot := func() {
				rootUUID := uuid.New()
				_, err := unitStore.CreateRoot(ctx, database.CreateRootOrganizationInput{
					Organization:  &database.Organization{Name: "enterprise-root", UUID: rootUUID, UserID: owner.ID},
					Namespace:     &database.Namespace{Path: "enterprise-root", UUID: rootUUID.String(), UserID: owner.ID},
					CreatorUserID: owner.ID,
				})
				require.NoError(t, err)
			}
			if rootFirst {
				createRoot()
			}
			for _, path := range []string{"mirror-qwen", "mirror-deepseek"} {
				require.NoError(t, mirror.ensureMirrorOrgNamespace(ctx, path, owner.Username))
				org, err := mirror.orgStore.FindByPath(ctx, path)
				require.NoError(t, err)
				require.True(t, org.IsRoot)
				require.False(t, org.IsHierarchical)
				require.NotNil(t, org.Namespace)
				require.Equal(t, path, org.Namespace.Path)
				member, err := database.NewMemberStoreWithDB(db).Find(ctx, org.ID, owner.ID)
				require.NoError(t, err)
				require.Equal(t, string(types.UserAdmin), member.Role)
			}
			if !rootFirst {
				exists, err := unitStore.HasActiveHierarchicalRoot(ctx)
				require.NoError(t, err)
				require.False(t, exists)
				createRoot()
			}
			root, err := unitStore.FindActiveHierarchicalRoot(ctx)
			require.NoError(t, err)
			require.Equal(t, "enterprise-root", root.Name)
			// Only the actual hierarchy root owns a unit and closure self-row.
			var units []database.OrganizationUnit
			require.NoError(t, db.Core.NewSelect().Model(&units).Scan(ctx))
			require.Len(t, units, 1)
			require.Equal(t, root.ID, units[0].OrganizationID)
			var closures []database.OrganizationUnitClosure
			require.NoError(t, db.Core.NewSelect().Model(&closures).Scan(ctx))
			require.Len(t, closures, 1)
			require.Equal(t, root.ID, closures[0].RootOrganizationID)
			require.Equal(t, units[0].ID, closures[0].AncestorUnitID)
			require.Equal(t, units[0].ID, closures[0].DescendantUnitID)
		})
	}
}
