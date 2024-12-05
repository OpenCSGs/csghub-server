package tests

import (
	"github.com/stretchr/testify/mock"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
)

type MockStores struct {
	User                 database.UserStore
	UserLikes            database.UserLikesStore
	Repo                 database.RepoStore
	RepoRelation         database.RepoRelationsStore
	Model                database.ModelStore
	SpaceResource        database.SpaceResourceStore
	Tag                  database.TagStore
	Dataset              database.DatasetStore
	PromptConversation   database.PromptConversationStore
	PromptPrefix         database.PromptPrefixStore
	LLMConfig            database.LLMConfigStore
	Prompt               database.PromptStore
	Namespace            database.NamespaceStore
	LfsMetaObject        database.LfsMetaObjectStore
	Mirror               database.MirrorStore
	MirrorSource         database.MirrorSourceStore
	AccessToken          database.AccessTokenStore
	SyncVersion          database.SyncVersionStore
	SyncClientSetting    database.SyncClientSettingStore
	RuntimeFramework     database.RuntimeFrameworksStore
	DeployTask           database.DeployTaskStore
	ClusterInfo          database.ClusterInfoStore
	Code                 database.CodeStore
	Collection           database.CollectionStore
	Space                database.SpaceStore
	SpaceSdk             database.SpaceSdkStore
	Recom                database.RecomStore
	RepoRuntimeFramework database.RepositoriesRuntimeFrameworkStore
}

func NewMockStores(t interface {
	Cleanup(func())
	mock.TestingT
}) *MockStores {
	return &MockStores{
		User:                 mockdb.NewMockUserStore(t),
		UserLikes:            mockdb.NewMockUserLikesStore(t),
		Repo:                 mockdb.NewMockRepoStore(t),
		RepoRelation:         mockdb.NewMockRepoRelationsStore(t),
		Model:                mockdb.NewMockModelStore(t),
		SpaceResource:        mockdb.NewMockSpaceResourceStore(t),
		Tag:                  mockdb.NewMockTagStore(t),
		Dataset:              mockdb.NewMockDatasetStore(t),
		PromptConversation:   mockdb.NewMockPromptConversationStore(t),
		PromptPrefix:         mockdb.NewMockPromptPrefixStore(t),
		LLMConfig:            mockdb.NewMockLLMConfigStore(t),
		Prompt:               mockdb.NewMockPromptStore(t),
		Namespace:            mockdb.NewMockNamespaceStore(t),
		LfsMetaObject:        mockdb.NewMockLfsMetaObjectStore(t),
		Mirror:               mockdb.NewMockMirrorStore(t),
		MirrorSource:         mockdb.NewMockMirrorSourceStore(t),
		AccessToken:          mockdb.NewMockAccessTokenStore(t),
		SyncVersion:          mockdb.NewMockSyncVersionStore(t),
		SyncClientSetting:    mockdb.NewMockSyncClientSettingStore(t),
		RuntimeFramework:     mockdb.NewMockRuntimeFrameworksStore(t),
		DeployTask:           mockdb.NewMockDeployTaskStore(t),
		ClusterInfo:          mockdb.NewMockClusterInfoStore(t),
		Code:                 mockdb.NewMockCodeStore(t),
		Collection:           mockdb.NewMockCollectionStore(t),
		Space:                mockdb.NewMockSpaceStore(t),
		SpaceSdk:             mockdb.NewMockSpaceSdkStore(t),
		Recom:                mockdb.NewMockRecomStore(t),
		RepoRuntimeFramework: mockdb.NewMockRepositoriesRuntimeFrameworkStore(t),
	}
}

func (s *MockStores) UserMock() *mockdb.MockUserStore {
	return s.User.(*mockdb.MockUserStore)
}

func (s *MockStores) UserLikesMock() *mockdb.MockUserLikesStore {
	return s.UserLikes.(*mockdb.MockUserLikesStore)
}

func (s *MockStores) RepoMock() *mockdb.MockRepoStore {
	return s.Repo.(*mockdb.MockRepoStore)
}

func (s *MockStores) RepoRelationMock() *mockdb.MockRepoRelationsStore {
	return s.RepoRelation.(*mockdb.MockRepoRelationsStore)
}

func (s *MockStores) ModelMock() *mockdb.MockModelStore {
	return s.Model.(*mockdb.MockModelStore)
}

func (s *MockStores) SpaceResourceMock() *mockdb.MockSpaceResourceStore {
	return s.SpaceResource.(*mockdb.MockSpaceResourceStore)
}

func (s *MockStores) TagMock() *mockdb.MockTagStore {
	return s.Tag.(*mockdb.MockTagStore)
}

func (s *MockStores) DatasetMock() *mockdb.MockDatasetStore {
	return s.Dataset.(*mockdb.MockDatasetStore)
}

func (s *MockStores) PromptConversationMock() *mockdb.MockPromptConversationStore {
	return s.PromptConversation.(*mockdb.MockPromptConversationStore)
}

func (s *MockStores) PromptPrefixMock() *mockdb.MockPromptPrefixStore {
	return s.PromptPrefix.(*mockdb.MockPromptPrefixStore)
}

func (s *MockStores) LLMConfigMock() *mockdb.MockLLMConfigStore {
	return s.LLMConfig.(*mockdb.MockLLMConfigStore)
}

func (s *MockStores) PromptMock() *mockdb.MockPromptStore {
	return s.Prompt.(*mockdb.MockPromptStore)
}

func (s *MockStores) NamespaceMock() *mockdb.MockNamespaceStore {
	return s.Namespace.(*mockdb.MockNamespaceStore)
}

func (s *MockStores) LfsMetaObjectMock() *mockdb.MockLfsMetaObjectStore {
	return s.LfsMetaObject.(*mockdb.MockLfsMetaObjectStore)
}

func (s *MockStores) MirrorMock() *mockdb.MockMirrorStore {
	return s.Mirror.(*mockdb.MockMirrorStore)
}

func (s *MockStores) MirrorSourceMock() *mockdb.MockMirrorSourceStore {
	return s.MirrorSource.(*mockdb.MockMirrorSourceStore)
}

func (s *MockStores) AccessTokenMock() *mockdb.MockAccessTokenStore {
	return s.AccessToken.(*mockdb.MockAccessTokenStore)
}

func (s *MockStores) SyncVersionMock() *mockdb.MockSyncVersionStore {
	return s.SyncVersion.(*mockdb.MockSyncVersionStore)
}

func (s *MockStores) SyncClientSettingMock() *mockdb.MockSyncClientSettingStore {
	return s.SyncClientSetting.(*mockdb.MockSyncClientSettingStore)
}

func (s *MockStores) RuntimeFrameworkMock() *mockdb.MockRuntimeFrameworksStore {
	return s.RuntimeFramework.(*mockdb.MockRuntimeFrameworksStore)
}

func (s *MockStores) DeployTaskMock() *mockdb.MockDeployTaskStore {
	return s.DeployTask.(*mockdb.MockDeployTaskStore)
}

func (s *MockStores) ClusterInfoMock() *mockdb.MockClusterInfoStore {
	return s.ClusterInfo.(*mockdb.MockClusterInfoStore)
}

func (s *MockStores) CodeMock() *mockdb.MockCodeStore {
	return s.Code.(*mockdb.MockCodeStore)
}

func (s *MockStores) CollectionMock() *mockdb.MockCollectionStore {
	return s.Collection.(*mockdb.MockCollectionStore)
}

func (s *MockStores) SpaceMock() *mockdb.MockSpaceStore {
	return s.Space.(*mockdb.MockSpaceStore)
}

func (s *MockStores) SpaceSdkMock() *mockdb.MockSpaceSdkStore {
	return s.SpaceSdk.(*mockdb.MockSpaceSdkStore)
}

func (s *MockStores) RecomMock() *mockdb.MockRecomStore {
	return s.Recom.(*mockdb.MockRecomStore)
}

func (s *MockStores) RepoRuntimeFrameworkMock() *mockdb.MockRepositoriesRuntimeFrameworkStore {
	return s.RepoRuntimeFramework.(*mockdb.MockRepositoriesRuntimeFrameworkStore)
}
