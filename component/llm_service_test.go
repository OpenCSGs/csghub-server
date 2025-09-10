package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestLLMServiceComponent_CreateLLMConfig(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	req := &types.CreateLLMConfigReq{
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		AuthHeader:  "Bearer token",
		Type:        666,
		Enabled:     true,
	}
	dbLLMConfig := &database.LLMConfig{
		ID:          123,
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		AuthHeader:  "Bearer token",
		Type:        666,
		Enabled:     true,
	}
	mc.mocks.stores.LLMConfigMock().EXPECT().Create(ctx, database.LLMConfig{
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		AuthHeader:  "Bearer token",
		Type:        666,
		Enabled:     true,
	}).Return(dbLLMConfig, nil)
	res, err := mc.CreateLLMConfig(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.ModelName, "new-model")
}

func TestLLMServiceComponent_CreatePromptPrefix(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	req := &types.CreatePromptPrefixReq{
		ZH:   "zh",
		EN:   "en",
		Kind: "kind",
	}
	dbPromptPrefix := &database.PromptPrefix{
		ID:   123,
		ZH:   "zh",
		EN:   "en",
		Kind: "kind",
	}
	mc.mocks.stores.PromptPrefixMock().EXPECT().Create(ctx, database.PromptPrefix{
		ZH:   "zh",
		EN:   "en",
		Kind: "kind",
	}).Return(dbPromptPrefix, nil)
	res, err := mc.CreatePromptPrefix(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.Kind, "kind")
}
func TestLLMServiceComponent_IndexLLMConfig(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	per := 1
	page := 1
	search := &types.SearchLLMConfig{
		Keyword: "",
	}
	dbLLMConfig := &database.LLMConfig{
		ID:          123,
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		AuthHeader:  "Bearer token",
		Type:        666,
		Enabled:     true,
	}
	mc.mocks.stores.LLMConfigMock().EXPECT().Index(ctx, per, page, search).Return([]*database.LLMConfig{dbLLMConfig}, 1, nil)
	res, total, err := mc.IndexLLMConfig(ctx, per, page, search)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res, []*database.LLMConfig{dbLLMConfig})
	require.Equal(t, total, 1)
}

func TestLLMServiceComponent_IndexPromptPrefix(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	per := 1
	page := 1
	dbPromptPrefix := &database.PromptPrefix{
		ID: 123,
		ZH: "zh",
	}
	search := &types.SearchPromptPrefix{}
	mc.mocks.stores.PromptPrefixMock().EXPECT().Index(ctx, per, page, search).Return([]*database.PromptPrefix{dbPromptPrefix}, 1, nil)
	res, total, err := mc.IndexPromptPrefix(ctx, per, page, search)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res, []*database.PromptPrefix{dbPromptPrefix})
	require.Equal(t, total, 1)
}

func TestLLMServiceComponent_UpdateLLMConfig(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	newName := "new-model"
	req := &types.UpdateLLMConfigReq{
		ID:        123,
		ModelName: &newName,
	}
	dbLLMConfig := &database.LLMConfig{
		ID:        123,
		ModelName: newName,
	}
	mc.mocks.stores.LLMConfigMock().EXPECT().GetByID(ctx, int64(123)).Return(dbLLMConfig, nil)
	mc.mocks.stores.LLMConfigMock().EXPECT().Update(ctx, database.LLMConfig{
		ID:        123,
		ModelName: newName,
	}).Return(dbLLMConfig, nil)
	res, err := mc.UpdateLLMConfig(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.ModelName, "new-model")
}

func TestLLMServiceComponent_UpdatePromptPrefix(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	newKind := "new-kind"
	req := &types.UpdatePromptPrefixReq{
		ID:   123,
		Kind: &newKind,
	}
	dbPromptPrefix := &database.PromptPrefix{
		ID:   123,
		Kind: newKind,
	}
	mc.mocks.stores.PromptPrefixMock().EXPECT().GetByID(ctx, int64(123)).Return(dbPromptPrefix, nil)
	mc.mocks.stores.PromptPrefixMock().EXPECT().Update(ctx, database.PromptPrefix{
		ID:   123,
		Kind: newKind,
	}).Return(dbPromptPrefix, nil)
	res, err := mc.UpdatePromptPrefix(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.Kind, "new-kind")
}

func TestLLMServiceComponent_DeleteLLMConfig(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	mc.mocks.stores.LLMConfigMock().EXPECT().Delete(ctx, int64(123)).Return(nil)
	err := mc.DeleteLLMConfig(ctx, int64(123))
	require.Nil(t, err)
}

func TestLLMServiceComponent_DeletePromptPrefix(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestLLMServiceComponent(ctx, t)
	mc.mocks.stores.PromptPrefixMock().EXPECT().Delete(ctx, int64(123)).Return(nil)
	err := mc.DeletePromptPrefix(ctx, int64(123))
	require.Nil(t, err)
}
