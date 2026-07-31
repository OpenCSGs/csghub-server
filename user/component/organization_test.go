package component

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockusermodule "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
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
	mockNamespaceStore.EXPECT().ExistsByUUID(mock.Anything, mock.Anything).Return(false, nil).Once()

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	mockMemberComponent := mockusermodule.NewMockMemberComponent(t)
	mockMemberComponent.EXPECT().InitRoles(mock.Anything, mock.Anything).Return(nil).Once()
	mockMemberComponent.EXPECT().SetAdmin(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(false, nil).Once()
	mockSSO.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil).Once()

	mockTagStore := mockdb.NewMockTagStore(t)

	// GetOrganizationTags is called to return tags in the response
	mockOrgStore.EXPECT().GetOrganizationTags(mock.Anything, mock.Anything).Return([]database.Tag{}, nil).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		orgStore:  mockOrgStore,
		tagStore:  mockTagStore,
		msc:       mockMemberComponent,
		sso:       mockSSO,
	}
	org, err := c.Create(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, req.Name, org.Name)
	require.Equal(t, req.Nickname, org.Nickname)
	require.Equal(t, req.Homepage, org.Homepage)
	require.Equal(t, req.Logo, org.Logo)
	require.Equal(t, req.OrgType, org.OrgType)
	require.Equal(t, req.Verified, org.Verified)
	require.NotEqual(t, uuid.Nil, org.UUID)
}

func TestOrganizationComponent_Create_NamespaceExists(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:     "org1",
		Username: "user1",
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(true, nil).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
	}
	org, err := c.Create(context.Background(), req)
	require.Nil(t, org)
	require.ErrorIs(t, err, errorx.ErrNamespaceAlreadyExists)
}

func TestOrganizationComponent_Create_SSOUserExists(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:     "org1",
		Username: "user1",
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()

	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(true, nil).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		sso:       mockSSO,
	}
	org, err := c.Create(context.Background(), req)
	require.Nil(t, org)
	require.ErrorIs(t, err, errorx.ErrNamespaceAlreadyExists)
}

func TestOrganizationComponent_Create_InvalidTagIDs(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:     "org1",
		Username: "user1",
		TagIDs:   []int64{999},
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()

	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(false, nil).Once()

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExistInScope(mock.Anything, []int64{999}, types.OrganizationTagScope, string(types.IndustryCategory)).Return(database.ErrTagIDsNotFoundInScope).Once()

	// No mocks for Git, SSO CreateUser, or DB Create — tag validation fails first
	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		sso:       mockSSO,
		tagStore:  mockTagStore,
	}
	org, err := c.Create(context.Background(), req)
	require.Nil(t, org)
	require.ErrorIs(t, err, errorx.ErrTagIDsNotExist)
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
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})
	dbOrgs = append(dbOrgs, database.Organization{
		ID:       2,
		Name:     "org2",
		Nickname: "org_nickname",
		Homepage: "org-homepage.com",
		Logo:     "org-logo.png",
		OrgType:  "school",
		Verified: false,
		Namespace: &database.Namespace{
			Path:          "org2",
			NamespaceType: database.OrgNamespace,
		},
	})
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().Search(mock.Anything, "", 10, 0, "", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1, 2}).Return(map[int64][]database.Tag{}, nil).Once()

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	expectedOrgs, total, err := c.Index(context.Background(), "", 10, 0, "", "")

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
			if expectedOrgs[i].Namespace == nil {
				return false
			}
			if expectedOrgs[i].Namespace.Path != dbOrgs[i].Namespace.Path {
				return false
			}
		}
		return true
	})
}

func TestOrganizationComponent_ListUserOrgs_Admin(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:    1,
		Name:  "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(1), "", 10, 1, "", "", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "admin1").Return(database.User{
		ID:       1,
		Username: "admin1",
		RoleMask: "admin",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "admin1", Per: 10, Page: 1,
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_RegularUser(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:    1,
		Name:  "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "", 10, 1, "", "", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Per: 10, Page: 1,
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_RegularUserWithFilters(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:       1,
		Name:     "org1",
		OrgType:  "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "search", 5, 2, "school", "approved", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Search: "search", Per: 5, Page: 2, OrgType: "school", VerifyStatus: "approved",
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_EmptyUsername(t *testing.T) {
	c := &organizationComponentImpl{}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{})

	require.Error(t, err)
	require.Nil(t, orgs)
	require.Equal(t, 0, total)
	require.Contains(t, err.Error(), "username is required")
}

func TestOrganizationComponent_ListUserOrgs_OwnerRole(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:    1,
		Name:  "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "", 10, 1, "", "", "owner").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Per: 10, Page: 1, Role: "owner",
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_WriteRole(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:    1,
		Name:  "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "", 10, 1, "", "", "write").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Per: 10, Page: 1, Role: "write",
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_AdminRole(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:    1,
		Name:  "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "", 10, 1, "", "", "admin").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Per: 10, Page: 1, Role: "admin",
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_toOrgList(t *testing.T) {
	dbOrgs := []database.Organization{
		{
			ID:           1,
			Name:         "org1",
			Nickname:     "nick1",
			Description:  "desc1",
			Homepage:     "https://org1.com",
			Logo:         "logo1.png",
			OrgType:      "school",
			Verified:     true,
			VerifyStatus: "approved",
			UUID:         uuid.New(),
			Namespace: &database.Namespace{
				Path:          "org1",
				NamespaceType: database.OrgNamespace,
			},
		},
		{
			ID:    2,
			Name:  "org2",
			OrgType: "company",
		},
	}

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1, 2}).Return(map[int64][]database.Tag{}, nil).Once()

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	orgs, err := c.toOrgList(context.Background(), dbOrgs)
	require.NoError(t, err)

	require.Len(t, orgs, 2)
	require.Equal(t, "org1", orgs[0].Name)
	require.Equal(t, "nick1", orgs[0].Nickname)
	require.Equal(t, "desc1", orgs[0].Description)
	require.Equal(t, "https://org1.com", orgs[0].Homepage)
	require.Equal(t, "logo1.png", orgs[0].Logo)
	require.Equal(t, "school", orgs[0].OrgType)
	require.True(t, orgs[0].Verified)
	require.Equal(t, "approved", orgs[0].VerifyStatus)
	require.NotNil(t, orgs[0].Namespace)
	require.Equal(t, "org1", orgs[0].Namespace.Path)

	require.Equal(t, "org2", orgs[1].Name)
	require.Nil(t, orgs[1].Namespace)
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

	newDesc := "org1 description"

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

	mockGitServer := mockgit.NewMockGitServer(t)


	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
		msc:       mc,
		gs:        mockGitServer,
	}

	returnOrg, err := c.Update(context.Background(), &types.EditOrgReq{
		Name:        "org1",
		NewOwner:    &user1.Username,
		CurrentUser: operator.Username,
		Description: &newDesc,
	})

	require.NoError(t, err)
	require.Equal(t, "org1", returnOrg.Name)
	require.Equal(t, newDesc, returnOrg.Description)
}

func TestOrganizationComponent_Get(t *testing.T) {
	dbOrg := database.Organization{
		ID:       1,
		Nickname: "org1",
		Name:     "org_path",
		Homepage: "https://org1.com",
		Logo:     "https://org1.com/logo.png",
		OrgType:  "company",
		Verified: true,
		Namespace: &database.Namespace{
			ID:            1,
			Path:          "org_path",
			NamespaceType: database.OrgNamespace,
			UUID:          "ns-uuid-1",
		},
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindByPath(mock.Anything, "org_path").Return(dbOrg, nil)
	mockOrgStore.EXPECT().GetOrganizationTags(mock.Anything, dbOrg.ID).Return([]database.Tag{}, nil)

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	org, err := c.Get(context.Background(), "org_path")
	require.NoError(t, err)
	require.Equal(t, "org_path", org.Name)
	require.Equal(t, "org1", org.Nickname)
	require.Equal(t, "https://org1.com", org.Homepage)
	require.NotNil(t, org.Namespace)
	require.Equal(t, "org_path", org.Namespace.Path)
}

func TestOrganizationComponent_GetByUUID(t *testing.T) {
	dbOrg := &database.Organization{
		ID:       1,
		Nickname: "org1",
		Name:     "org_path",
		Homepage: "https://org1.com",
		Logo:     "https://org1.com/logo.png",
		OrgType:  "company",
		Verified: true,
		Namespace: &database.Namespace{
			ID:            1,
			Path:          "org_path",
			NamespaceType: database.OrgNamespace,
			UUID:          "ns-uuid-1",
		},
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindByUUID(mock.Anything, "org-uuid-123").Return(dbOrg, nil)
	mockOrgStore.EXPECT().GetOrganizationTags(mock.Anything, dbOrg.ID).Return([]database.Tag{}, nil)

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	org, err := c.GetByUUID(context.Background(), "org-uuid-123")
	require.NoError(t, err)
	require.Equal(t, "org_path", org.Name)
	require.Equal(t, "org1", org.Nickname)
	require.Equal(t, "https://org1.com", org.Homepage)
	require.NotNil(t, org.Namespace)
	require.Equal(t, "org_path", org.Namespace.Path)
}
