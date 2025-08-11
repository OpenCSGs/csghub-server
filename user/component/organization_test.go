package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockusermodule "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestOrganizationComponent_Create(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:        "org1",
		Nickname:    "org_nickname",
		Description: "org_description",
		Username:    "user1",
		Homepage:    "org-homepage.com",
		Logo:        "org-logo.png",
		Verified:    false,
		OrgType:     "school",
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()

	mockGitServer := mockgit.NewMockGitServer(t)
	mockGitServer.EXPECT().CreateOrganization(mock.Anything, mock.Anything).Return(&database.Organization{
		ID:       1,
		Name:     req.Name,
		Nickname: req.Nickname,
	}, nil).Once()

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	mockMemberComponent := mockusermodule.NewMockMemberComponent(t)
	mockMemberComponent.EXPECT().InitRoles(mock.Anything, mock.Anything).Return(nil).Once()
	mockMemberComponent.EXPECT().SetAdmin(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	expected := &types.Organization{
		Name:     req.Name,
		Nickname: req.Nickname,
		Homepage: req.Homepage,
		Logo:     req.Logo,
		OrgType:  req.OrgType,
		Verified: req.Verified,
	}

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		gs:        mockGitServer,
		orgStore:  mockOrgStore,
		msc:       mockMemberComponent,
	}
	org, err := c.Create(context.Background(), req)
	require.NoError(t, err)
	require.EqualValues(t, expected, org)
}

func TestOrganizationComponent_Index(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:       1,
		Name:     "org1",
		Nickname: "org_nickname",
		Homepage: "org-homepage.com",
		Logo:     "org-logo.png",
		OrgType:  "school",
		Verified: false,
	})
	dbOrgs = append(dbOrgs, database.Organization{
		ID:       2,
		Name:     "org2",
		Nickname: "org_nickname",
		Homepage: "org-homepage.com",
		Logo:     "org-logo.png",
		OrgType:  "school",
		Verified: false,
	})
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().GetUserOwnOrgs(mock.Anything, "user1").Return(dbOrgs, len(dbOrgs), nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	expectedOrgs, total, err := c.Index(context.Background(), "user1", "", 10, 0)

	require.NoError(t, err)
	require.Len(t, expectedOrgs, 2)
	require.Equal(t, 2, total)
	require.Condition(t, func() bool {

		for i := 0; i < len(expectedOrgs); i++ {
			if expectedOrgs[i].Name != dbOrgs[i].Name {
				return false
			}
			if expectedOrgs[i].Nickname != dbOrgs[i].Nickname {
				return false
			}
			if expectedOrgs[i].Homepage != dbOrgs[i].Homepage {
				return false
			}
			if expectedOrgs[i].Logo != dbOrgs[i].Logo {
				return false
			}
			if expectedOrgs[i].OrgType != dbOrgs[i].OrgType {
				return false
			}
			if expectedOrgs[i].Verified != dbOrgs[i].Verified {
				return false
			}
		}
		return true
	})
}

func TestOrganizationComponent_Update(t *testing.T) {
	org := database.Organization{
		ID:       1,
		UserID:   1,
		Name:     "org1",
		Nickname: "org_nickname",
		Homepage: "org-homepage.com",
		Logo:     "org-logo.png",
		OrgType:  "school",
		Verified: false,
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindByPath(mock.Anything, "org1").Return(org, nil)
	mockOrgStore.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	mockUserStore := mockdb.NewMockUserStore(t)
	user1 := database.User{
		Username: "user1",
		RoleMask: "",
		ID:       2,
	}
	operator := database.User{
		Username: "op",
		ID:       1,
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, user1.Username).Return(user1, nil)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(operator, nil)

	mems := mockdb.NewMockMemberStore(t)
	member := &database.Member{
		ID:             1,
		OrganizationID: 1,
		UserID:         2,
		Role:           "admin",
		User: &database.User{
			ID: 2, Username: "user1", NickName: "nick1", Avatar: "avatar1", UUID: "uuid1",
			LastLoginAt: "2020-01-01T00:00:00Z",
		},
	}

	opMember := &database.Member{
		ID:             2,
		OrganizationID: 1,
		UserID:         1,
		Role:           "admin",
		User: &database.User{
			ID: 1, Username: "op", NickName: "nick1", Avatar: "avatar1", UUID: "uuid1",
			LastLoginAt: "2020-01-01T00:00:00Z",
		},
	}
	mems.EXPECT().Find(mock.Anything, org.ID, int64(1)).Return(opMember, nil)
	mems.EXPECT().Find(mock.Anything, org.ID, int64(2)).Return(member, nil)

	mc := &memberComponentImpl{
		memberStore: mems,
		userStore:   mockUserStore,
		orgStore:    mockOrgStore,
	}

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
		msc:       mc,
	}
	returnOrg, err := c.Update(context.Background(), &types.EditOrgReq{
		Name:        "org1",
		NewOwner:    &user1.Username,
		CurrentUser: operator.Username,
	})

	require.NoError(t, err)
	require.Equal(t, "org1", returnOrg.Name)
}
