package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

type BroadcastTester struct {
	*GinTester
	handler *BroadcastHandler
	mocks   struct {
		ec *mockcomponent.MockBroadcastComponent
	}
}

func NewBroadcastTester(t *testing.T) *BroadcastTester {
	tester := &BroadcastTester{GinTester: NewGinTester()}
	tester.mocks.ec = mockcomponent.NewMockBroadcastComponent(t)

	tester.handler = &BroadcastHandler{
		ec: tester.mocks.ec,
	}
	tester.WithParam("id", "1")
	return tester
}

func (t *BroadcastTester) WithHandleFunc(fn func(h *BroadcastHandler) gin.HandlerFunc) *BroadcastTester {
	t.ginHandler = fn(t.handler)
	return t
}

func TestBroadcastHandler_Index(t *testing.T) {
	tester := NewBroadcastTester(t).WithHandleFunc(func(h *BroadcastHandler) gin.HandlerFunc {
		return h.Index
	})

	tester.mocks.ec.EXPECT().AllBroadcasts(tester.ctx).Return([]types.Broadcast{
		{Content: "test", BcType: "banner", Theme: "light", Status: "inactive"},
	}, nil)

	tester.Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"data": []types.Broadcast{{Content: "test", BcType: "banner", Theme: "light", Status: "inactive"}},
		"msg":  "OK",
	})
}

func TestBroadcastHandler_Create(t *testing.T) {
	tester := NewBroadcastTester(t).WithHandleFunc(func(h *BroadcastHandler) gin.HandlerFunc {
		return h.Create
	})

	broadcast := types.Broadcast{
		Content: "test", BcType: "banner", Theme: "light", Status: "inactive",
	}

	tester.mocks.ec.EXPECT().NewBroadcast(tester.ctx, broadcast).Return(nil)

	tester.WithBody(t, &types.Broadcast{
		Content: "test", BcType: "banner", Theme: "light", Status: "inactive",
	}).Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"msg": "OK",
	})
}

func TestBroadcastHandler_Update(t *testing.T) {
	tester := NewBroadcastTester(t).WithHandleFunc(func(h *BroadcastHandler) gin.HandlerFunc {
		return h.Update
	})

	broadcast := types.Broadcast{
		ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "inactive",
	}

	tester.mocks.ec.EXPECT().ActiveBroadcast(tester.ctx).Return(&broadcast, nil)
	tester.mocks.ec.EXPECT().UpdateBroadcast(tester.ctx, broadcast).Return(&broadcast, nil)

	tester.WithBody(t, &types.Broadcast{
		Content: "test", BcType: "banner", Theme: "light", Status: "inactive",
	}).Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"data": types.Broadcast{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "inactive"},
		"msg":  "OK",
	})
}

func TestBroadcastHandler_Show(t *testing.T) {
	tester := NewBroadcastTester(t).WithHandleFunc(func(h *BroadcastHandler) gin.HandlerFunc {
		return h.Show
	})

	broadcast := types.Broadcast{
		ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "inactive",
	}

	tester.mocks.ec.EXPECT().GetBroadcast(tester.ctx, broadcast.ID).Return(&broadcast, nil)

	tester.Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"data": types.Broadcast{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "inactive"},
		"msg":  "OK",
	})
}
