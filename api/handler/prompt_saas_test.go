package handler

import (
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type PromptOptimizeTester struct {
	*testutil.GinTester
	handler PromptOptimizeHandler
	mocks   struct {
		prompt    *mock_component.MockPromptComponent
		sensitive *mock_component.MockSensitiveComponent
		repo      *mock_component.MockRepoComponent
	}
}

func NewPromptOptimizeTester(t *testing.T) *PromptOptimizeTester {
	tester := &PromptOptimizeTester{GinTester: testutil.NewGinTester()}
	tester.mocks.prompt = mock_component.NewMockPromptComponent(t)
	tester.mocks.sensitive = mock_component.NewMockSensitiveComponent(t)
	tester.mocks.repo = mock_component.NewMockRepoComponent(t)
	tester.handler = &promptOptimizeHandlerImpl{
		prompt: tester.mocks.prompt, sensitive: tester.mocks.sensitive,
		repo: tester.mocks.repo,
	}
	tester.WithParam("name", "r")
	tester.WithParam("namespace", "u")
	return tester

}

func (t *PromptOptimizeTester) WithHandleFunc(fn func(h PromptOptimizeHandler) gin.HandlerFunc) *PromptOptimizeTester {
	t.Handler(fn(t.handler))
	return t

}

func TestPromptOptimizeHandler_NewConversation(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.NewConversation
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().NewConversation(tester.Ctx(), types.ConversationTitleReq{
		CurrentUser: "u",
		ConversationTitle: types.ConversationTitle{
			Uuid:  "uuid",
			Title: "title",
		},
	}).Return(&database.PromptConversation{ID: 5}, nil)
	tester.WithBody(t, &types.ConversationTitle{Uuid: "uuid", Title: "title"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.PromptConversation{ID: 5})

}

func TestPromptOptimizeHandler_ListConversation(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.ListConversation
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().ListConversationsByUserID(tester.Ctx(), "u").Return(
		[]database.PromptConversation{{ID: 5}}, nil,
	)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, []database.PromptConversation{{ID: 5}})

}

func TestPromptOptimizeHandler_GetConversation(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.GetConversation
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().GetConversation(tester.Ctx(), types.ConversationReq{
		CurrentUser:  "u",
		Conversation: types.Conversation{Uuid: "uuid"},
	}).Return(
		&database.PromptConversation{ID: 5}, nil,
	)
	tester.WithParam("id", "uuid").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.PromptConversation{ID: 5})

}

func TestPromptOptimizeHandler_SubmitMessage(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.SubmitMessage
	})
	tester.WithUser()

	ch := make(chan string)
	tester.mocks.prompt.EXPECT().SubmitMessage(tester.Ctx(), types.ConversationReq{
		CurrentUser: "u",
		Conversation: types.Conversation{
			Uuid:        "uuid",
			Message:     "msg",
			Temperature: tea.Float64(1.23),
		},
	}).Return(
		ch, nil,
	)

	tester.mocks.prompt.EXPECT().SaveGeneratedText(tester.Ctx(), types.Conversation{
		Uuid:    "uuid",
		Message: "foobar",
	}).Return(&database.PromptConversationMessage{ID: 2}, nil)

	go func() {
		ch <- `data:{"id": "d1", "choices": [{"delta": {"content": "foo"}}]}`
		ch <- `data:{"id": "d2", "choices": [{"delta": {"content": "bar"}}]}`
		close(ch)
	}()

	tester.WithParam("id", "uuid").WithBody(t, &types.Conversation{
		Message:     "msg",
		Temperature: tea.Float64(1.23),
		Uuid:        "foo",
	}).Execute()

	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:data\ndata:{\"id\": \"d1\", \"choices\": [{\"delta\": {\"content\": \"foo\"}}]}\n\nevent:data\ndata:{\"id\": \"d2\", \"choices\": [{\"delta\": {\"content\": \"bar\"}}]}\n\nevent:data\ndata:{\"msg_id\": 2}\n\n",
		tester.Response().Body.String(),
	)

}

func TestPromptOptimizeHandler_UpdateConversation(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.UpdateConversation
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().UpdateConversation(tester.Ctx(), types.ConversationTitleReq{
		CurrentUser: "u",
		ConversationTitle: types.ConversationTitle{
			Uuid:  "uuid",
			Title: "title",
		},
	}).Return(&database.PromptConversation{ID: 5}, nil)
	tester.WithParam("id", "uuid").WithBody(t, &types.ConversationTitle{Uuid: "uuid", Title: "title"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.PromptConversation{ID: 5})

}

func TestPromptOptimizeHandler_RemoveConversation(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.RemoveConversation
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().RemoveConversation(tester.Ctx(), types.ConversationReq{
		CurrentUser: "u",
		Conversation: types.Conversation{
			Uuid: "uuid",
		},
	}).Return(nil)
	tester.WithParam("id", "uuid").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestPromptOptimizeHandler_LikeMessage(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.LikeMessage
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().LikeConversationMessage(tester.Ctx(), types.ConversationMessageReq{
		Uuid:        "uuid",
		Id:          1,
		CurrentUser: "u",
	}).Return(nil)
	tester.WithParam("id", "uuid").WithParam("msgid", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestPromptOptimizeHandler_HateMessage(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.HateMessage
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().HateConversationMessage(tester.Ctx(), types.ConversationMessageReq{
		Uuid:        "uuid",
		Id:          1,
		CurrentUser: "u",
	}).Return(nil)
	tester.WithParam("id", "uuid").WithParam("msgid", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestPromptOptimizeHandler_SummarizeTitle(t *testing.T) {
	tester := NewPromptOptimizeTester(t).WithHandleFunc(func(h PromptOptimizeHandler) gin.HandlerFunc {
		return h.SummarizeTitle
	})
	tester.WithUser()

	tester.mocks.prompt.EXPECT().SummarizeConversationTitle(tester.Ctx(), types.ConversationTitleReq{
		CurrentUser: "u",
		ConversationTitle: types.ConversationTitle{
			Uuid: "uuid",
		},
	}).Return(&database.PromptConversation{ID: 5}, nil)

	tester.WithParam("id", "uuid").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.PromptConversation{ID: 5})
}
