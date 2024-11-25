package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/jarcoal/httpmock"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	gsmock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	urpc_mock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	component_mock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func NewTestPromptComponent(stores *tests.MockStores, rpcUser rpc.UserSvcClient, gitServer gitserver.GitServer) *promptComponentImpl {
	cfg := &config.Config{}
	cfg.APIServer.PublicDomain = "https://foo.com"
	cfg.APIServer.SSHDomain = "ssh://test@127.0.0.1"
	return &promptComponentImpl{
		userStore:         stores.User,
		userLikeStore:     stores.UserLikes,
		promptConvStore:   stores.PromptConversation,
		promptPrefixStore: stores.PromptPrefix,
		llmConfigStore:    stores.LLMConfig,
		promptStore:       stores.Prompt,
		namespaceStore:    stores.Namespace,
		userSvcClient:     rpcUser,
		gitServer:         gitServer,
		repoStore:         stores.Repo,
		llmClient:         llm.NewClient(),
		config:            cfg,
	}
}

func TestPromptComponent_CheckPermission(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	rpcUser := urpc_mock.NewMockUserSvcClient(t)
	pc := NewTestPromptComponent(stores, rpcUser, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	req := types.PromptReq{
		Path:        "p",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	}

	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, errors.New("")).Once()
	_, err := pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "namespace does not exist")

	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{}, errors.New("")).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "user does not exist")

	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.UserNamespace,
		Path:          "zzz",
	}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo",
		Username: "zzz",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)

	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.UserNamespace,
		Path:          "uuu",
	}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo",
		Username: "vvv",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "user do not have permission")

	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.UserNamespace,
		Path:          "uuu",
	}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo",
		Username: "uuu",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)

	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "super-admin",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)

	// no write role
	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "person",
	}, nil).Once()
	mockedRepoComponent.EXPECT().CheckCurrentUserPermission(ctx, "foo", "ns", membership.RoleWrite).Return(false, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "user do not have permission")

	// has write role
	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "person",
	}, nil).Once()
	mockedRepoComponent.EXPECT().CheckCurrentUserPermission(ctx, "foo", "ns", membership.RoleWrite).Return(true, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)
}

func TestPromptComponent_CreatePrompt(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	rpcUser := urpc_mock.NewMockUserSvcClient(t)
	pc := NewTestPromptComponent(stores, rpcUser, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	for _, exist := range []bool{true, false} {
		t.Run(fmt.Sprintf("file exist: %v", exist), func(t *testing.T) {

			stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
				NamespaceType: database.OrgNamespace,
			}, nil).Once()
			stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
				RoleMask: "foo-admin",
				Email:    "foo@bar.com",
			}, nil).Once()
			if exist {
				gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", nil).Once()

				_, err := pc.CreatePrompt(ctx, types.PromptReq{
					Namespace:   "ns",
					Name:        "n",
					CurrentUser: "foo",
					Path:        "p",
				}, &CreatePromptReq{
					Prompt: Prompt{Title: "TEST", Content: "test"},
				})
				require.NotNil(t, err)
				return
			} else {
				gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", errors.New("")).Once()
			}

			mockedRepoComponent.EXPECT().CreateFile(ctx, &types.CreateFileReq{
				Namespace:   "ns",
				Name:        "n",
				Branch:      types.MainBranch,
				FilePath:    "TEST.jsonl",
				Content:     "eyJ0aXRsZSI6IlRFU1QiLCJjb250ZW50IjoidGVzdCIsImxhbmd1YWdlIjoiIiwidGFncyI6bnVsbCwidHlwZSI6IiIsInNvdXJjZSI6IiIsImF1dGhvciI6IiIsInRpbWUiOiIiLCJjb3B5cmlnaHQiOiIiLCJmZWVkYmFjayI6bnVsbH0=",
				RepoType:    types.PromptRepo,
				CurrentUser: "foo",
				Username:    "foo",
				Email:       "foo@bar.com",
				Message:     fmt.Sprintf("create prompt %s", "TEST.jsonl"),
			}).Return(&types.CreateFileResp{}, nil).Once()

			_, err := pc.CreatePrompt(ctx, types.PromptReq{
				Namespace:   "ns",
				Name:        "n",
				CurrentUser: "foo",
				Path:        "p",
			}, &CreatePromptReq{
				Prompt: Prompt{Title: "TEST", Content: "test"},
			})
			require.Nil(t, err)

		})
	}

}

func TestPromptComponent_UpdatePrompt(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	rpcUser := urpc_mock.NewMockUserSvcClient(t)
	pc := NewTestPromptComponent(stores, rpcUser, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	for _, exist := range []bool{true, false} {
		t.Run(fmt.Sprintf("file exist: %v", exist), func(t *testing.T) {

			stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
				NamespaceType: database.OrgNamespace,
			}, nil).Once()
			stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
				RoleMask: "foo-admin",
				Email:    "foo@bar.com",
			}, nil).Once()
			if !exist {
				gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", errors.New("")).Once()

				_, err := pc.UpdatePrompt(ctx, types.PromptReq{
					Namespace:   "ns",
					Name:        "n",
					CurrentUser: "foo",
					Path:        "TEST.jsonl",
				}, &UpdatePromptReq{
					Prompt: Prompt{Title: "TEST.jsonl", Content: "test"},
				})
				require.NotNil(t, err)
				return
			} else {
				gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", nil).Once()
			}

			mockedRepoComponent.EXPECT().UpdateFile(ctx, &types.UpdateFileReq{
				Namespace:   "ns",
				Name:        "n",
				Branch:      types.MainBranch,
				FilePath:    "TEST.jsonl",
				Content:     "eyJ0aXRsZSI6IlRFU1QiLCJjb250ZW50IjoidGVzdCIsImxhbmd1YWdlIjoiIiwidGFncyI6bnVsbCwidHlwZSI6IiIsInNvdXJjZSI6IiIsImF1dGhvciI6IiIsInRpbWUiOiIiLCJjb3B5cmlnaHQiOiIiLCJmZWVkYmFjayI6bnVsbH0=",
				RepoType:    types.PromptRepo,
				CurrentUser: "foo",
				Username:    "foo",
				Email:       "foo@bar.com",
				Message:     fmt.Sprintf("update prompt %s", "TEST.jsonl"),
			}).Return(&types.UpdateFileResp{}, nil).Once()

			_, err := pc.UpdatePrompt(ctx, types.PromptReq{
				Namespace:   "ns",
				Name:        "n",
				CurrentUser: "foo",
				Path:        "TEST.jsonl",
			}, &UpdatePromptReq{
				Prompt: Prompt{Title: "TEST", Content: "test"},
			})
			require.Nil(t, err)

		})
	}

}

func TestPromptComponent_DeletePrompt(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	rpcUser := urpc_mock.NewMockUserSvcClient(t)
	pc := NewTestPromptComponent(stores, rpcUser, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo-admin",
		Email:    "foo@bar.com",
	}, nil).Once()

	mockedRepoComponent.EXPECT().DeleteFile(ctx, &types.DeleteFileReq{
		Namespace:   "ns",
		Name:        "n",
		Branch:      types.MainBranch,
		FilePath:    "TEST.jsonl",
		Content:     "",
		RepoType:    types.PromptRepo,
		CurrentUser: "foo",
		Username:    "foo",
		Email:       "foo@bar.com",
		Message:     fmt.Sprintf("delete prompt %s", "TEST.jsonl"),
	}).Return(&types.DeleteFileResp{}, nil).Once()

	err := pc.DeletePrompt(ctx, types.PromptReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
		Path:        "TEST.jsonl",
	})
	require.Nil(t, err)

}

func TestPromptComponent_ListPrompt(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	rpcUser := urpc_mock.NewMockUserSvcClient(t)
	pc := NewTestPromptComponent(stores, rpcUser, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	repo := &database.Repository{}
	stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	mockedRepoComponent.EXPECT().AllowReadAccessRepo(ctx, repo, "foo").Return(true, nil).Once()

	gitServer.EXPECT().GetRepoFileTree(context.Background(), gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return([]*types.File{
		{Name: "foo.jsonl", Path: "foo.jsonl"},
		{Name: "bar.jsonl", Path: "bar.jsonl"},
		{Name: "main.go", Path: "main.go"},
		{Name: "large.jsonl", Path: "large.jsonl", Size: 12345678987654},
	}, nil)

	for _, name := range []string{"foo.jsonl", "bar.jsonl"} {
		gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
			Namespace: "ns",
			Name:      "n",
			Ref:       "main",
			Path:      name,
			RepoType:  types.PromptRepo,
		}).Return(&types.File{
			Content: "LS0tCiB0aXRsZTogImFpIg==",
		}, nil).Once()
	}

	outputs, err := pc.ListPrompt(ctx, types.PromptReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
		Path:        "p",
	})
	require.Nil(t, err)
	require.Equal(t, 2, len(outputs))
	paths := []string{}
	for _, o := range outputs {
		require.Equal(t, "ai", o.Title)
		paths = append(paths, o.FilePath)
	}
	require.ElementsMatch(t, []string{"foo.jsonl", "bar.jsonl"}, paths)

}

func TestPromptComponent_GetPrompt(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	rpcUser := urpc_mock.NewMockUserSvcClient(t)
	pc := NewTestPromptComponent(stores, rpcUser, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	repo := &database.Repository{}
	stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	mockedRepoComponent.EXPECT().GetUserRepoPermission(ctx, "foo", repo).Return(&types.UserRepoPermission{
		CanRead: true,
	}, nil).Once()

	gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "foo.jsonl",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCiB0aXRsZTogImFpIg==",
	}, nil).Once()

	output, err := pc.GetPrompt(ctx, types.PromptReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
		Path:        "foo.jsonl",
	})
	require.Nil(t, err)
	require.Equal(t, "ai", output.Title)
	require.Equal(t, "foo.jsonl", output.FilePath)
}

func TestPromptComponent_NewConversation(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
	stores.PromptConversationMock().EXPECT().CreateConversation(ctx, database.PromptConversation{
		UserID:         123,
		ConversationID: "zzz",
		Title:          "test",
	}).Return(nil).Once()

	cv, err := pc.NewConversation(ctx, types.ConversationTitleReq{
		CurrentUser: "foo",
		ConversationTitle: types.ConversationTitle{
			Uuid:  "zzz",
			Title: "test",
		},
	})
	require.Nil(t, err)
	require.Equal(t, 123, int(cv.UserID))

}

func TestPromptComponent_ListConversationByUserID(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
	mockedResults := []database.PromptConversation{
		{Title: "foo"},
		{Title: "bar"},
	}
	stores.PromptConversationMock().EXPECT().FindConversationsByUserID(ctx, int64(123)).Return(mockedResults, nil).Once()

	results, err := pc.ListConversationsByUserID(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, mockedResults, results)

}

func TestPromptComponent_GetConversation(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
	mocked := &database.PromptConversation{}
	stores.PromptConversationMock().EXPECT().GetConversationByID(ctx, int64(123), "uuid", true).Return(mocked, nil).Once()

	cv, err := pc.GetConversation(ctx, types.ConversationReq{
		CurrentUser: "foo",
		Conversation: types.Conversation{
			Uuid: "uuid",
		},
	})
	require.Nil(t, err)
	require.Equal(t, mocked, cv)

}

func TestPromptComponent_SubmitMessage(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {

			stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
			stores.PromptConversationMock().EXPECT().GetConversationByID(ctx, int64(123), "uuid", false).Return(&database.PromptConversation{}, nil).Once()

			content := "go"
			if lang == "zh" {
				content = "围棋"
			}
			stores.PromptConversationMock().EXPECT().SaveConversationMessage(
				ctx, database.PromptConversationMessage{
					ConversationID: "uuid",
					Role:           UserRole,
					Content:        content,
				},
			).Return(&database.PromptConversationMessage{}, nil)
			stores.LLMConfigMock().EXPECT().GetOptimization(ctx).Return(&database.LLMConfig{
				ApiEndpoint: "https://llm.com",
				AuthHeader:  `{"token": "foobar"}`,
			}, nil).Once()
			stores.PromptPrefixMock().EXPECT().Get(ctx).Return(&database.PromptPrefix{
				ZH: "use Chinese",
				EN: "use English",
			}, nil).Once()
			httpmock.Activate()
			t.Cleanup(httpmock.DeactivateAndReset)

			httpmock.RegisterResponder("POST", "https://llm.com",
				func(req *http.Request) (*http.Response, error) {
					article := make(map[string]interface{})
					if err := json.NewDecoder(req.Body).Decode(&article); err != nil {
						return httpmock.NewStringResponse(400, ""), nil
					}
					prefix := cast.ToStringMap(cast.ToSlice(article["messages"])[0])["content"]
					d := ""
					switch prefix {
					case "use English":
						d = `[{"id": 1, "name": "My Great Article"}]`
					case "use Chinese":
						d = `[{"id": 1, "name": "好好好"}]`
					default:
						d = "wrong"
					}
					return httpmock.NewStringResponse(
						200, d,
					), nil
				})

			ch, err := pc.SubmitMessage(ctx, types.ConversationReq{
				CurrentUser: "foo",
				Conversation: types.Conversation{
					Uuid:    "uuid",
					Message: content,
				},
			})
			require.Nil(t, err)
			all := ""
			for i := range ch {
				all += i
			}
			if lang == "en" {
				require.Equal(t, "[{\"id\": 1, \"name\": \"My Great Article\"}]", all)
			} else {
				require.Equal(t, "[{\"id\": 1, \"name\": \"好好好\"}]", all)
			}
		})
	}
}

func TestPromptComponent_SaveGeneratedText(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	mocked := &database.PromptConversationMessage{}
	stores.PromptConversationMock().EXPECT().SaveConversationMessage(ctx, database.PromptConversationMessage{
		ConversationID: "uuid",
		Role:           AssistantRole,
		Content:        "m",
	}).Return(mocked, nil).Once()

	m, err := pc.SaveGeneratedText(ctx, types.Conversation{
		Uuid:        "uuid",
		Message:     "m",
		Temperature: tea.Float64(0.8),
	})
	require.Nil(t, err)
	require.Equal(t, mocked, m)
}

func TestPromptComponent_RemoveConversation(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
	stores.PromptConversationMock().EXPECT().DeleteConversationsByID(ctx, int64(123), "uuid").Return(nil).Once()

	err := pc.RemoveConversation(ctx, types.ConversationReq{
		CurrentUser: "foo",
		Conversation: types.Conversation{
			Uuid: "uuid",
		},
	})
	require.Nil(t, err)
}

func TestPromptComponent_UpdateConversation(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
	stores.PromptConversationMock().EXPECT().UpdateConversation(ctx, database.PromptConversation{
		UserID:         123,
		ConversationID: "uuid",
		Title:          "title",
	}).Return(nil).Once()
	mocked := &database.PromptConversation{}
	stores.PromptConversationMock().EXPECT().GetConversationByID(ctx, int64(123), "uuid", false).Return(mocked, nil)

	cv, err := pc.UpdateConversation(ctx, types.ConversationTitleReq{
		CurrentUser: "foo",
		ConversationTitle: types.ConversationTitle{
			Uuid:  "uuid",
			Title: "title",
		},
	})
	require.Nil(t, err)
	require.Equal(t, mocked, cv)
}

func TestPromptComponent_LikeConversationMessage(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
	stores.PromptConversationMock().EXPECT().GetConversationByID(ctx, int64(123), "uuid", false).Return(&database.PromptConversation{}, nil).Once()
	stores.PromptConversationMock().EXPECT().LikeMessageByID(ctx, int64(123)).Return(nil).Once()

	err := pc.LikeConversationMessage(ctx, types.ConversationMessageReq{
		Uuid:        "uuid",
		Id:          123,
		CurrentUser: "foo",
	})
	require.Nil(t, err)
}

func TestPromptComponent_HateConversationMessage(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{ID: 123}, nil).Once()
	stores.PromptConversationMock().EXPECT().GetConversationByID(ctx, int64(123), "uuid", false).Return(&database.PromptConversation{}, nil).Once()
	stores.PromptConversationMock().EXPECT().HateMessageByID(ctx, int64(123)).Return(nil).Once()

	err := pc.HateConversationMessage(ctx, types.ConversationMessageReq{
		Uuid:        "uuid",
		Id:          123,
		CurrentUser: "foo",
	})
	require.Nil(t, err)
}

func TestPromptComponent_SetRelationModels(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	pc := NewTestPromptComponent(stores, nil, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:    123,
		Email: "foo@bar.com",
	}, nil).Once()
	repo := &database.Repository{}
	stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	mockedRepoComponent.EXPECT().GetUserRepoPermission(ctx, "foo", repo).Return(&types.UserRepoPermission{
		CanWrite: true,
	}, nil).Once()
	gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "README.md",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCiB0aXRsZTogImFpIg==",
	}, nil).Once()
	gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Branch:    types.MainBranch,
		Message:   "update model relation tags",
		FilePath:  REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
		Namespace: "ns",
		Name:      "n",
		Username:  "foo",
		Email:     "foo@bar.com",
		Content:   "LS0tCm1vZGVsczoKICAgIC0gbWEKICAgIC0gbWIKICAgIC0gbWMKdGl0bGU6IGFpCgotLS0=",
	}).Return(nil).Once()

	err := pc.SetRelationModels(ctx, types.RelationModels{
		Models:      []string{"ma", "mb", "mc"},
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	})
	require.Nil(t, err)

}

func TestPromptComponent_AddRelationModel(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	pc := NewTestPromptComponent(stores, nil, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:       123,
		Email:    "foo@bar.com",
		RoleMask: "foo-admin",
	}, nil).Once()
	repo := &database.Repository{}
	stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "README.md",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCiB0aXRsZTogImFpIg==",
	}, nil).Once()
	gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Branch:    types.MainBranch,
		Message:   "add relation model",
		FilePath:  REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
		Namespace: "ns",
		Name:      "n",
		Username:  "foo",
		Email:     "foo@bar.com",
		Content:   "LS0tCm1vZGVsczoKICAgIC0gbWEKdGl0bGU6IGFpCgotLS0=",
	}).Return(nil).Once()

	err := pc.AddRelationModel(ctx, types.RelationModel{
		Model:       "ma",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	})
	require.Nil(t, err)

}

func TestPromptComponent_DelRelationModel(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	pc := NewTestPromptComponent(stores, nil, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:       123,
		Email:    "foo@bar.com",
		RoleMask: "foo-admin",
	}, nil).Once()
	repo := &database.Repository{}
	stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "README.md",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCm1vZGVsczoKICAgIC0gbWEKdGl0bGU6IGFpCgotLS0=",
	}, nil).Once()
	gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Branch:    types.MainBranch,
		Message:   "delete relation model",
		FilePath:  REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
		Namespace: "ns",
		Name:      "n",
		Username:  "foo",
		Email:     "foo@bar.com",
		Content:   "LS0tCm1vZGVsczogW10KdGl0bGU6IGFpCgotLS0=",
	}).Return(nil).Once()

	err := pc.DelRelationModel(ctx, types.RelationModel{
		Model:       "ma",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	})
	require.Nil(t, err)

}

func TestPromptComponent_CreatePromptRepo(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	pc := NewTestPromptComponent(stores, nil, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:       123,
		Email:    "foo@bar.com",
		RoleMask: "foo-admin",
	}, nil).Once()

	req := types.CreateRepoReq{
		Username:      "foo",
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "nc",
		Description:   "good",
		License:       "MIT",
		Readme:        "\n---\nlicense: MIT\n---\n\t",
		DefaultBranch: "main",
		RepoType:      types.PromptRepo,
	}
	stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil).Once()
	dbRepo := &database.Repository{}
	mockedRepoComponent.EXPECT().CreateRepo(ctx, req).Return(&gitserver.CreateRepoResp{}, dbRepo, nil)
	dbPrompt := database.Prompt{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}
	stores.PromptMock().EXPECT().Create(ctx, dbPrompt).Return(&database.Prompt{
		Repository: &database.Repository{
			Name: "r1",
			Tags: []database.Tag{
				{Name: "t1"},
				{Name: "t2"},
			},
		},
	}, nil)
	// create readme
	gitServer.EXPECT().CreateRepoFile(&types.CreateFileReq{
		Email:     "foo@bar.com",
		Message:   "initial commit",
		Branch:    "main",
		Content:   "Ci0tLQpsaWNlbnNlOiBNSVQKLS0tCgk=",
		NewBranch: "main",
		FilePath:  "README.md",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return(nil).Once()
	// create .gitattributes
	gitServer.EXPECT().CreateRepoFile(&types.CreateFileReq{
		Email:     "foo@bar.com",
		Message:   "initial commit",
		Branch:    "main",
		Content:   base64.StdEncoding.EncodeToString([]byte(datasetGitattributesContent)),
		NewBranch: "main",
		FilePath:  ".gitattributes",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return(nil).Once()

	res, err := pc.CreatePromptRepo(ctx, &types.CreatePromptRepoReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:    "foo",
			Namespace:   "ns",
			Name:        "n",
			Nickname:    "nc",
			Description: "good",
			License:     "MIT",
		},
	})
	require.Nil(t, err)
	expected := types.PromptRes{
		Name: "r1",
		Repository: types.Repository{
			HTTPCloneURL: "https://foo.com/s/.git",
			SSHCloneURL:  "test@127.0.0.1:s/.git",
		},
		User: types.User{Email: "foo@bar.com"},
		Tags: []types.RepoTag{
			{Name: "t1"},
			{Name: "t2"},
		},
	}
	require.Equal(t, expected, *res)

}

func TestPromptComponent_IndexPromptRepo(t *testing.T) {
	stores := tests.NewMockStores(t)
	gitServer := gsmock.NewMockGitServer(t)
	pc := NewTestPromptComponent(stores, nil, gitServer)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	filter := &types.RepoFilter{Username: "foo"}
	mockedRepoComponent.EXPECT().PublicToUser(ctx, types.PromptRepo, "foo", filter, 1, 1).Return([]*database.Repository{
		{ID: 1, Name: "rp1"}, {ID: 2, Name: "rp2"},
		{ID: 3, Name: "rp3", Tags: []database.Tag{{Name: "t1"}, {Name: "t2"}}},
	}, 30, nil).Once()
	stores.PromptMock().EXPECT().ByRepoIDs(ctx, []int64{1, 2, 3}).Return([]database.Prompt{
		{ID: 6, RepositoryID: 2, Repository: &database.Repository{}},
		{ID: 5, RepositoryID: 1, Repository: &database.Repository{}},
		{ID: 4, RepositoryID: 3, Repository: &database.Repository{}},
		{ID: 3, RepositoryID: 2, Repository: &database.Repository{}},
	}, nil).Once()

	results, total, err := pc.IndexPromptRepo(ctx, filter, 1, 1)
	require.Nil(t, err)
	require.Equal(t, 30, total)
	require.Equal(t, []types.PromptRes{
		{
			RepositoryID: 1,
			ID:           5, Name: "rp1", Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			}},
		{
			RepositoryID: 2,
			ID:           6, Name: "rp2", Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			}},
		{
			RepositoryID: 3,
			ID:           4, Name: "rp3", Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			}, Tags: []types.RepoTag{{Name: "t1"}, {Name: "t2"}}},
	}, results)

}

func TestPromptComponent_UpdatePromptRepo(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	req := &types.UpdatePromptRepoReq{
		UpdateRepoReq: types.UpdateRepoReq{RepoType: types.PromptRepo},
	}
	mockedRepo := &database.Repository{Name: "rp1", ID: 123}
	mockedRepoComponent.EXPECT().UpdateRepo(ctx, req.UpdateRepoReq).Return(mockedRepo, nil).Once()
	mockedPrompt := &database.Prompt{ID: 3}
	stores.PromptMock().EXPECT().ByRepoID(ctx, int64(123)).Return(mockedPrompt, nil).Once()
	stores.PromptMock().EXPECT().Update(ctx, *mockedPrompt).Return(nil).Once()

	res, err := pc.UpdatePromptRepo(ctx, req)
	require.Nil(t, err)
	require.Equal(t, types.PromptRes{
		RepositoryID: 123,
		ID:           3,
		Name:         "rp1",
	}, *res)

}

func TestPromptComponent_RemovetRepo(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	mockedPrompt := &database.Prompt{}
	stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "n").Return(mockedPrompt, nil).Once()
	mockedRepoComponent.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "foo",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return(nil, nil).Once()
	stores.PromptMock().EXPECT().Delete(ctx, *mockedPrompt).Return(nil).Once()

	err := pc.RemoveRepo(ctx, "ns", "n", "foo")
	require.Nil(t, err)

}

func TestPromptComponent_Show(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	mockedPrompt := &database.Prompt{
		Repository: &database.Repository{
			ID:   123,
			Tags: []database.Tag{{Name: "t1"}},
		},
	}
	stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "n").Return(mockedPrompt, nil).Once()
	mockedRepoComponent.EXPECT().GetUserRepoPermission(ctx, "foo", mockedPrompt.Repository).Return(&types.UserRepoPermission{
		CanRead: true,
	}, nil).Once()
	mockedRepoComponent.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil).Once()
	stores.UserLikesMock().EXPECT().IsExist(ctx, "foo", int64(123)).Return(true, nil).Once()

	res, err := pc.Show(ctx, "ns", "n", "foo")
	require.Nil(t, err)
	require.Equal(t, types.PromptRes{
		RepositoryID: 123,
		Repository: types.Repository{
			HTTPCloneURL: "https://foo.com/s/.git",
			SSHCloneURL:  "test@127.0.0.1:s/.git",
		},
		Tags:      []types.RepoTag{{Name: "t1"}},
		UserLikes: true,
		Namespace: &types.Namespace{},
	}, *res)

}

func TestPromptComponent_Relations(t *testing.T) {
	stores := tests.NewMockStores(t)
	pc := NewTestPromptComponent(stores, nil, nil)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	mockedPrompt := &database.Prompt{
		RepositoryID: 123,
		Repository: &database.Repository{
			ID:   123,
			Tags: []database.Tag{{Name: "t1"}},
		},
	}
	stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "n").Return(mockedPrompt, nil).Once()
	mockedRepoComponent.EXPECT().AllowReadAccessRepo(ctx, mockedPrompt.Repository, "foo").Return(true, nil).Once()
	mockedRepoComponent.EXPECT().RelatedRepos(ctx, int64(123), "foo").Return(
		map[types.RepositoryType][]*database.Repository{
			types.ModelRepo: {
				{ID: 1, Name: "r1"},
				{ID: 2, Name: "r2"},
			},
			types.PromptRepo: {
				{ID: 3, Name: "r3"},
				{ID: 4, Name: "r4"},
			},
		},
		nil).Once()

	r, err := pc.Relations(ctx, "ns", "n", "foo")
	require.Nil(t, err)
	require.Equal(t, types.Relations{
		Models: []*types.Model{
			{Name: "r1"},
			{Name: "r2"},
		},
	}, *r)

}

func TestPromptComponent_OrgPrompts(t *testing.T) {
	stores := tests.NewMockStores(t)
	rpcUser := urpc_mock.NewMockUserSvcClient(t)
	pc := NewTestPromptComponent(stores, rpcUser, nil)
	mockedRepoComponent := component_mock.NewMockRepoComponent(t)
	pc.repoComponent = mockedRepoComponent
	ctx := context.TODO()

	cases := []struct {
		role       membership.Role
		publicOnly bool
	}{
		{membership.RoleUnknown, true},
		{membership.RoleAdmin, false},
	}

	for _, c := range cases {
		t.Run(string(c.role), func(t *testing.T) {
			rpcUser.EXPECT().GetMemberRole(ctx, "ns", "foo").Return(c.role, nil).Once()
			stores.PromptMock().EXPECT().ByOrgPath(ctx, "ns", 1, 1, c.publicOnly).Return([]database.Prompt{
				{ID: 1, Repository: &database.Repository{Name: "r1"}},
				{ID: 2, Repository: &database.Repository{Name: "r2"}},
				{ID: 3, Repository: &database.Repository{Name: "r3"}},
			}, 100, nil)
			res, count, err := pc.OrgPrompts(ctx, &types.OrgDatasetsReq{
				Namespace:   "ns",
				CurrentUser: "foo",
				PageOpts: types.PageOpts{
					Page:     1,
					PageSize: 1,
				},
			})
			require.Nil(t, err)
			require.Equal(t, 100, count)
			require.Equal(t, []types.PromptRes{
				{ID: 1, Name: "r1"},
				{ID: 2, Name: "r2"},
				{ID: 3, Name: "r3"},
			}, res)
		})

	}

}
