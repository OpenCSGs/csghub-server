//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *promptComponentImpl) NewConversation(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error) {
	return nil, nil
}

func (c *promptComponentImpl) ListConversationsByUserID(ctx context.Context, currentUser string) ([]database.PromptConversation, error) {
	return nil, nil
}

func (c *promptComponentImpl) GetConversation(ctx context.Context, req types.ConversationReq) (*database.PromptConversation, error) {
	return nil, nil
}

func (c *promptComponentImpl) SubmitMessage(ctx context.Context, req types.ConversationReq) (<-chan string, error) {
	return nil, nil
}

func (c *promptComponentImpl) SaveGeneratedText(ctx context.Context, req types.Conversation) (*database.PromptConversationMessage, error) {
	return nil, nil
}

func (c *promptComponentImpl) RemoveConversation(ctx context.Context, req types.ConversationReq) error {

	return nil
}

func (c *promptComponentImpl) UpdateConversation(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error) {
	return nil, nil
}

func (c *promptComponentImpl) LikeConversationMessage(ctx context.Context, req types.ConversationMessageReq) error {
	return nil
}

func (c *promptComponentImpl) HateConversationMessage(ctx context.Context, req types.ConversationMessageReq) error {
	return nil
}

func (c *promptComponentImpl) SummarizeConversationTitle(ctx context.Context, req types.ConversationTitleReq) (*database.PromptConversation, error) {
	return nil, nil
}

func (c *promptComponentImpl) addOpWeightToPrompts(ctx context.Context, repoIDs []int64, resPrompts []*types.PromptRes) {
}
