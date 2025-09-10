package handler

import (
	"fmt"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type CodeTester struct {
	*testutil.GinTester
	handler *CodeHandler
	mocks   struct {
		code      *mockcomponent.MockCodeComponent
		sensitive *mockcomponent.MockSensitiveComponent
	}
}

func NewCodeTester(t *testing.T) *CodeTester {
	tester := &CodeTester{GinTester: testutil.NewGinTester()}
	tester.mocks.code = mockcomponent.NewMockCodeComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)
	tester.handler = &CodeHandler{code: tester.mocks.code, sensitive: tester.mocks.sensitive}
	tester.WithParam("name", "r")
	tester.WithParam("namespace", "u")
	return tester

}

func (ct *CodeTester) WithHandleFunc(fn func(cp *CodeHandler) gin.HandlerFunc) *CodeTester {
	ct.Handler(fn(ct.handler))
	return ct
}

func TestCodeHandler_Create(t *testing.T) {
	tester := NewCodeTester(t).WithHandleFunc(func(cp *CodeHandler) gin.HandlerFunc {
		return cp.Create
	})
	tester.WithUser()

	req := &types.CreateCodeReq{CreateRepoReq: types.CreateRepoReq{Name: "c"}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	reqn := *req
	reqn.Username = "u"
	tester.mocks.code.EXPECT().Create(tester.Ctx(), &reqn).Return(&types.Code{Name: "c"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEqSimple(t, 200, httpbase.R{Msg: "OK", Data: &types.Code{Name: "c"}})

}

func TestCodeHandler_Index(t *testing.T) {

	cases := []struct {
		sort   string
		source string
		error  bool
	}{
		{"most_download", "local", false},
		{"foo", "local", true},
		{"most_download", "bar", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			tester := NewCodeTester(t).WithHandleFunc(func(cp *CodeHandler) gin.HandlerFunc {
				return cp.Index
			})

			if !c.error {
				tester.mocks.code.EXPECT().Index(tester.Ctx(), &types.RepoFilter{
					Search: "foo",
					Sort:   c.sort,
					Source: c.source,
				}, 10, 1, true).Return([]*types.Code{
					{Name: "cc"},
				}, 100, nil)
			}

			tester.AddPagination(1, 10).WithQuery("search", "foo").
				WithQuery("sort", c.sort).
				WithQuery("source", c.source).
				WithQuery("need_op_weight", "true").Execute()

			if c.error {
				require.Equal(t, 400, tester.Response().Code)
			} else {
				tester.ResponseEqSimple(t, 200, gin.H{
					"data":  []types.Code{{Name: "cc"}},
					"total": 100,
					"msg":   "OK",
				})
			}
		})
	}
}

func TestCodeHandler_Update(t *testing.T) {
	tester := NewCodeTester(t).WithHandleFunc(func(cp *CodeHandler) gin.HandlerFunc {
		return cp.Update
	})
	tester.WithUser()

	req := &types.UpdateCodeReq{UpdateRepoReq: types.UpdateRepoReq{Nickname: tea.String("nc")}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	reqn := *req
	reqn.Username = "u"
	reqn.Name = "r"
	reqn.Namespace = "u"
	tester.mocks.code.EXPECT().Update(tester.Ctx(), &reqn).Return(&types.Code{Name: "c"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Code{Name: "c"})

}

func TestCodeHandler_Delete(t *testing.T) {
	tester := NewCodeTester(t).WithHandleFunc(func(cp *CodeHandler) gin.HandlerFunc {
		return cp.Delete
	})
	tester.WithUser()

	tester.mocks.code.EXPECT().Delete(tester.Ctx(), "u", "r", "u").Return(nil)
	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestCodeHandler_Show(t *testing.T) {
	tester := NewCodeTester(t).WithHandleFunc(func(cp *CodeHandler) gin.HandlerFunc {
		return cp.Show
	})

	tester.mocks.code.EXPECT().Show(tester.Ctx(), "u", "r", "u", false, false).Return(&types.Code{Name: "c"}, nil)
	tester.WithUser().Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.Code{Name: "c"})
}

func TestCodeHandler_Relations(t *testing.T) {
	tester := NewCodeTester(t).WithHandleFunc(func(cp *CodeHandler) gin.HandlerFunc {
		return cp.Relations
	})

	tester.mocks.code.EXPECT().Relations(tester.Ctx(), "u", "r", "u").Return(&types.Relations{}, nil)
	tester.WithUser().Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.Relations{})
}
