package tests

import (
	"testing"

	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
)

type MockStores struct {
	User               database.UserStore
	UserLikes          database.UserLikesStore
	Repo               database.RepoStore
	Model              database.ModelStore
	SpaceResource      database.SpaceResourceStore
	Tag                database.TagStore
	Dataset            database.DatasetStore
	PromptConversation database.PromptConversationStore
	PromptPrefix       database.PromptPrefixStore
	LLMConfig          database.LLMConfigStore
	Prompt             database.PromptStore
	Namespace          database.NamespaceStore
}

func NewMockStores(t *testing.T) *MockStores {
	return &MockStores{
		User:               mockdb.NewMockUserStore(t),
		UserLikes:          mockdb.NewMockUserLikesStore(t),
		Repo:               mockdb.NewMockRepoStore(t),
		Model:              mockdb.NewMockModelStore(t),
		SpaceResource:      mockdb.NewMockSpaceResourceStore(t),
		Tag:                mockdb.NewMockTagStore(t),
		Dataset:            mockdb.NewMockDatasetStore(t),
		PromptConversation: mockdb.NewMockPromptConversationStore(t),
		PromptPrefix:       mockdb.NewMockPromptPrefixStore(t),
		LLMConfig:          mockdb.NewMockLLMConfigStore(t),
		Prompt:             mockdb.NewMockPromptStore(t),
		Namespace:          mockdb.NewMockNamespaceStore(t),
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
