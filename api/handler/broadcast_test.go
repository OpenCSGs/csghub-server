package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type BroadcastTester struct {
	*testutil.GinTester
	handler *BroadcastHandler
	mocks   struct {
		ec *mockcomponent.MockBroadcastComponent
	}
}

func NewBroadcastTester(t *testing.T) *BroadcastTester {
	tester := &BroadcastTester{GinTester: testutil.NewGinTester()}
	tester.mocks.ec = mockcomponent.NewMockBroadcastComponent(t)

	tester.handler = &BroadcastHandler{
		ec: tester.mocks.ec,
	}
	tester.WithParam("id", "1")
	return tester
}

func (t *BroadcastTester) WithHandleFunc(fn func(h *BroadcastHandler) gin.HandlerFunc) *BroadcastTester {
	t.Handler(fn(t.handler))
	return t
}

func TestBroadcastHandler_Index(t *testing.T) {
	tester := NewBroadcastTester(t).WithHandleFunc(func(h *BroadcastHandler) gin.HandlerFunc {
		return h.Index
	})

	tester.mocks.ec.EXPECT().AllBroadcasts(tester.Ctx()).Return([]types.Broadcast{
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

	tester.mocks.ec.EXPECT().NewBroadcast(tester.Ctx(), broadcast).Return(nil)

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

	tester.mocks.ec.EXPECT().ActiveBroadcast(tester.Ctx()).Return(&broadcast, nil)
	tester.mocks.ec.EXPECT().UpdateBroadcast(tester.Ctx(), broadcast).Return(&broadcast, nil)

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

	tester.mocks.ec.EXPECT().GetBroadcast(tester.Ctx(), broadcast.ID).Return(&broadcast, nil)

	tester.Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"data": types.Broadcast{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "inactive"},
		"msg":  "OK",
	})
}
