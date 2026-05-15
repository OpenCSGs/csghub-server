package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	mockgitserver "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockllm "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/llm"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestIndustryTagComponent_IdentifyIndustryTags(t *testing.T) {
	ctx := context.Background()
	tagStore := mockdatabase.NewMockTagStore(t)
	promptStore := mockdatabase.NewMockPromptPrefixStore(t)
	llmConfigStore := mockdatabase.NewMockLLMConfigStore(t)
	llmClient := mockllm.NewMockLLMSvcClient(t)
	builtIn := true

	tagStore.EXPECT().AllTags(ctx, &types.TagFilter{
		Scopes:     []types.TagScope{types.DatasetTagScope},
		Categories: []string{"industry"},
		BuiltIn:    &builtIn,
	}).Return([]*database.Tag{
		{Name: "finance", ID: 1},
		{Name: "healthcare", ID: 2},
	}, nil)
	promptStore.EXPECT().Get(ctx, IndustryRepoPromptPrefixKind).Return(&database.PromptPrefix{
		ZH: "prompt",
	}, nil)
	llmConfigStore.EXPECT().GetModelForSummaryReadme(ctx).Return(&database.LLMConfig{
		ModelName:   "mock",
		ApiEndpoint: "http://llm",
		AuthHeader:  "{}",
		Upstreams: []database.Upstream{
			{URL: "http://llm", Enabled: true, AuthHeader: "{}"},
		},
	}, nil)
	llmClient.EXPECT().Chat(ctx, "http://llm", "", map[string]string{}, types.LLMReqBody{
		Model: "mock",
		Messages: []types.LLMMessage{
			{Role: SystemRole, Content: "prompt"},
			{Role: UserRole, Content: "{\"candidates\":[\"finance\",\"healthcare\"],\"description\":\"financial dataset\",\"readme\":\"dataset about banks\"}"},
		},
		Stream:      false,
		Temperature: 0.0,
	}).Return(`{"tag_names":["finance","unknown","finance"],"reason":"matched finance"}`, nil)

	c := &industryTagComponentImpl{
		tagStore:          tagStore,
		promptPrefixStore: promptStore,
		llmConfigStore:    llmConfigStore,
		llmClient:         llmClient,
	}

	result, err := c.IdentifyIndustryTags(ctx, types.IdentifyIndustryTagsReq{
		Namespace:   "ns",
		Name:        "repo",
		RepoType:    types.DatasetRepo,
		Description: "financial dataset",
		Readme:      "dataset about banks",
	})
	require.NoError(t, err)
	require.Equal(t, []int64{1}, result.TagIDs)
	require.Equal(t, []string{"finance"}, result.TagNames)
	require.Equal(t, "llm_candidates", result.MatchedBy)
	require.Equal(t, "matched finance", result.Reason)
}

func TestIndustryTagComponent_RefreshRepoAutoIndustryTags(t *testing.T) {
	ctx := context.Background()
	repoStore := mockdatabase.NewMockRepoStore(t)
	tagStore := mockdatabase.NewMockTagStore(t)
	promptStore := mockdatabase.NewMockPromptPrefixStore(t)
	llmConfigStore := mockdatabase.NewMockLLMConfigStore(t)
	llmClient := mockllm.NewMockLLMSvcClient(t)
	builtIn := true

	repoStore.EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "repo").Return(&database.Repository{
		ID:            1,
		DefaultBranch: "main",
	}, nil)
	tagStore.EXPECT().AllTags(ctx, &types.TagFilter{
		Scopes:     []types.TagScope{types.DatasetTagScope},
		Categories: []string{"industry"},
		BuiltIn:    &builtIn,
	}).Return([]*database.Tag{
		{Name: "finance", ID: 2},
	}, nil)
	promptStore.EXPECT().Get(ctx, IndustryRepoPromptPrefixKind).Return(&database.PromptPrefix{
		ZH: "prompt",
	}, nil)
	llmConfigStore.EXPECT().GetModelForSummaryReadme(ctx).Return(&database.LLMConfig{
		ModelName:   "mock",
		ApiEndpoint: "http://llm",
		AuthHeader:  "{}",
		Upstreams: []database.Upstream{
			{URL: "http://llm", Enabled: true, AuthHeader: "{}"},
		},
	}, nil)
	llmClient.EXPECT().Chat(ctx, "http://llm", "", map[string]string{}, types.LLMReqBody{
		Model: "mock",
		Messages: []types.LLMMessage{
			{Role: SystemRole, Content: "prompt"},
			{Role: UserRole, Content: "{\"candidates\":[\"finance\"],\"description\":\"financial dataset\",\"readme\":\"dataset about banks\"}"},
		},
		Stream:      false,
		Temperature: 0.0,
	}).Return(`["finance"]`, nil)
	tagStore.EXPECT().ReplaceRepoTagsByCategoryAndSource(ctx, int64(1), "industry", types.TagSourceAuto, []int64{2}).Return(nil)

	c := &industryTagComponentImpl{
		repoStore:         repoStore,
		tagStore:          tagStore,
		promptPrefixStore: promptStore,
		llmConfigStore:    llmConfigStore,
		gitServer:         mockgitserver.NewMockGitServer(t),
		llmClient:         llmClient,
	}

	err := c.RefreshRepoAutoIndustryTags(ctx, types.IdentifyIndustryTagsReq{
		Namespace:   "ns",
		Name:        "repo",
		RepoType:    types.DatasetRepo,
		Description: "financial dataset",
		Readme:      "dataset about banks",
	})
	require.NoError(t, err)
}

func TestIndustryTagComponent_IdentifyIndustryTags_ModelRepo(t *testing.T) {
	ctx := context.Background()
	tagStore := mockdatabase.NewMockTagStore(t)
	promptStore := mockdatabase.NewMockPromptPrefixStore(t)
	llmConfigStore := mockdatabase.NewMockLLMConfigStore(t)
	llmClient := mockllm.NewMockLLMSvcClient(t)
	builtIn := true

	tagStore.EXPECT().AllTags(ctx, &types.TagFilter{
		Scopes:     []types.TagScope{types.ModelTagScope},
		Categories: []string{"industry"},
		BuiltIn:    &builtIn,
	}).Return([]*database.Tag{
		{Name: "finance", ID: 1},
	}, nil)
	promptStore.EXPECT().Get(ctx, IndustryRepoPromptPrefixKind).Return(&database.PromptPrefix{
		ZH: "prompt",
	}, nil)
	llmConfigStore.EXPECT().GetModelForSummaryReadme(ctx).Return(&database.LLMConfig{
		ModelName:   "mock",
		ApiEndpoint: "http://llm",
		AuthHeader:  "{}",
		Upstreams: []database.Upstream{
			{URL: "http://llm", Enabled: true, AuthHeader: "{}"},
		},
	}, nil)
	llmClient.EXPECT().Chat(ctx, "http://llm", "", map[string]string{}, types.LLMReqBody{
		Model: "mock",
		Messages: []types.LLMMessage{
			{Role: SystemRole, Content: "prompt"},
			{Role: UserRole, Content: "{\"candidates\":[\"finance\"],\"description\":\"financial model\",\"readme\":\"model for banking risk\"}"},
		},
		Stream:      false,
		Temperature: 0.0,
	}).Return(`["finance"]`, nil)

	c := &industryTagComponentImpl{
		tagStore:          tagStore,
		promptPrefixStore: promptStore,
		llmConfigStore:    llmConfigStore,
		llmClient:         llmClient,
	}

	result, err := c.IdentifyIndustryTags(ctx, types.IdentifyIndustryTagsReq{
		Namespace:   "ns",
		Name:        "repo",
		RepoType:    types.ModelRepo,
		Description: "financial model",
		Readme:      "model for banking risk",
	})
	require.NoError(t, err)
	require.Equal(t, []int64{1}, result.TagIDs)
	require.Equal(t, []string{"finance"}, result.TagNames)
}

func TestIndustryTagComponent_IdentifyIndustryTags_ChineseDescriptionMatchesEnglishTag(t *testing.T) {
	ctx := context.Background()
	tagStore := mockdatabase.NewMockTagStore(t)
	promptStore := mockdatabase.NewMockPromptPrefixStore(t)
	llmConfigStore := mockdatabase.NewMockLLMConfigStore(t)
	llmClient := mockllm.NewMockLLMSvcClient(t)
	builtIn := true

	tagStore.EXPECT().AllTags(ctx, &types.TagFilter{
		Scopes:     []types.TagScope{types.DatasetTagScope},
		Categories: []string{"industry"},
		BuiltIn:    &builtIn,
	}).Return([]*database.Tag{
		{Name: "finance", ID: 1},
		{Name: "healthcare", ID: 2},
	}, nil)
	promptStore.EXPECT().Get(ctx, IndustryRepoPromptPrefixKind).Return(&database.PromptPrefix{
		ZH: "prompt",
	}, nil)
	llmConfigStore.EXPECT().GetModelForSummaryReadme(ctx).Return(&database.LLMConfig{
		ModelName:   "mock",
		ApiEndpoint: "http://llm",
		AuthHeader:  "{}",
		Upstreams: []database.Upstream{
			{URL: "http://llm", Enabled: true, AuthHeader: "{}"},
		},
	}, nil)
	llmClient.EXPECT().Chat(ctx, "http://llm", "", map[string]string{}, types.LLMReqBody{
		Model: "mock",
		Messages: []types.LLMMessage{
			{Role: SystemRole, Content: "prompt"},
			{Role: UserRole, Content: "{\"candidates\":[\"finance\",\"healthcare\"],\"description\":\"这是一个银行风控和信贷评估数据集\",\"readme\":\"包含贷款违约预测、银行客户评分等金融场景样本\"}"},
		},
		Stream:      false,
		Temperature: 0.0,
	}).Return(`{"tag_names":["finance"],"reason":"银行风控和信贷评估属于金融行业"}`, nil)

	c := &industryTagComponentImpl{
		tagStore:          tagStore,
		promptPrefixStore: promptStore,
		llmConfigStore:    llmConfigStore,
		llmClient:         llmClient,
	}

	result, err := c.IdentifyIndustryTags(ctx, types.IdentifyIndustryTagsReq{
		Namespace:   "ns",
		Name:        "repo",
		RepoType:    types.DatasetRepo,
		Description: "这是一个银行风控和信贷评估数据集",
		Readme:      "包含贷款违约预测、银行客户评分等金融场景样本",
	})
	require.NoError(t, err)
	require.Equal(t, []int64{1}, result.TagIDs)
	require.Equal(t, []string{"finance"}, result.TagNames)
	require.Equal(t, "银行风控和信贷评估属于金融行业", result.Reason)
}
