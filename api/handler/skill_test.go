package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type fakeSkillPublisher struct {
	resp *types.PublishSkillVersionResp
	err  error
	req  *types.PublishSkillVersionReq
}

func (f *fakeSkillPublisher) Publish(ctx context.Context, req *types.PublishSkillVersionReq) (*types.PublishSkillVersionResp, error) {
	f.req = req
	return f.resp, f.err
}

type SkillTester struct {
	*testutil.GinTester
	handler *SkillHandler
	mocks   struct {
		skill     *mockcomponent.MockSkillComponent
		sensitive *mockcomponent.MockSensitiveComponent
		repo      *mockcomponent.MockRepoComponent
	}
}

func NewSkillTester(t *testing.T) *SkillTester {
	tester := &SkillTester{GinTester: testutil.NewGinTester()}
	tester.mocks.skill = mockcomponent.NewMockSkillComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)
	tester.handler = &SkillHandler{
		skill:     tester.mocks.skill,
		sensitive: tester.mocks.sensitive,
		repo:      tester.mocks.repo,
	}
	tester.WithParam("name", "r")
	tester.WithParam("namespace", "u")
	return tester

}

func (st *SkillTester) WithHandleFunc(fn func(cp *SkillHandler) gin.HandlerFunc) *SkillTester {
	st.Handler(fn(st.handler))
	return st
}

func TestSkillHandler_Create(t *testing.T) {
	t.Run("create with empty namespace", func(t *testing.T) {
		tester := NewSkillTester(t).WithHandleFunc(func(h *SkillHandler) gin.HandlerFunc {
			return h.Create
		})
		tester.WithUser()

		req := &types.CreateSkillReq{CreateRepoReq: types.CreateRepoReq{Username: "u"}}
		expect_req := &types.CreateSkillReq{CreateRepoReq: types.CreateRepoReq{Username: "u", Namespace: "u"}}
		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), expect_req).Return(true, nil)
		tester.mocks.skill.EXPECT().Create(tester.Ctx(), expect_req).Return(&types.Skill{Name: "s"}, nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, &types.Skill{Name: "s"})
	})
	t.Run("create for self", func(t *testing.T) {
		tester := NewSkillTester(t).WithHandleFunc(func(h *SkillHandler) gin.HandlerFunc {
			return h.Create
		})
		tester.WithUser()

		req := &types.CreateSkillReq{CreateRepoReq: types.CreateRepoReq{Username: "u", Namespace: "u"}}
		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
		tester.mocks.skill.EXPECT().Create(tester.Ctx(), req).Return(&types.Skill{Name: "s"}, nil)
		tester.WithBody(t, req).Execute()

		tester.ResponseEq(t, 200, tester.OKText, &types.Skill{Name: "s"})
	})
}

// TestSkillHandler_CreateMirrorAuthInvalid verifies invalid mirror credentials return a bad request.
func TestSkillHandler_CreateMirrorAuthInvalid(t *testing.T) {
	tester := NewSkillTester(t).WithHandleFunc(func(h *SkillHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.WithUser()

	req := &types.CreateSkillReq{CreateRepoReq: types.CreateRepoReq{Username: "u", Namespace: "u"}}
	authErr := errorx.MirrorSourceRepoAuthInvalid(fmt.Errorf("credentials are incomplete"), errorx.Ctx())
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.skill.EXPECT().Create(tester.Ctx(), req).Return(nil, authErr)
	tester.WithBody(t, req).Execute()

	tester.ResponseEqSimple(t, http.StatusBadRequest, httpbase.R{
		Code: "MIRROR-ERR-5",
		Msg:  "MIRROR-ERR-5: credentials are incomplete",
	})
}

// TestSkillHandler_CreateMirrorSourceBadRequest verifies malformed mirror source URLs return a bad request.
func TestSkillHandler_CreateMirrorSourceBadRequest(t *testing.T) {
	tester := NewSkillTester(t).WithHandleFunc(func(h *SkillHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.WithUser()

	req := &types.CreateSkillReq{CreateRepoReq: types.CreateRepoReq{Username: "u", Namespace: "u"}}
	badRequestErr := errorx.BadRequest(fmt.Errorf("invalid source git clone url"), errorx.Ctx())
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.skill.EXPECT().Create(tester.Ctx(), req).Return(nil, badRequestErr)
	tester.WithBody(t, req).Execute()

	tester.ResponseEqSimple(t, http.StatusBadRequest, httpbase.R{
		Code: "REQ-ERR-0",
		Msg:  "REQ-ERR-0: invalid source git clone url",
	})
}

func TestSkillHandler_Index(t *testing.T) {

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

			tester := NewSkillTester(t).WithHandleFunc(func(cp *SkillHandler) gin.HandlerFunc {
				return cp.Index
			})

			if !c.error {
				tester.mocks.skill.EXPECT().Index(tester.Ctx(), &types.RepoFilter{
					Search: "foo",
					Sort:   c.sort,
					Source: c.source,
				}, 10, 1, true, false).Return([]*types.Skill{
					{Name: "ss"},
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
					"data":  []types.Skill{{Name: "ss"}},
					"total": 100,
					"msg":   "OK",
				})
			}
		})
	}
}

func TestSkillHandler_Update(t *testing.T) {
	tester := NewSkillTester(t).WithHandleFunc(func(cp *SkillHandler) gin.HandlerFunc {
		return cp.Update
	})
	tester.WithUser()

	req := &types.UpdateSkillReq{UpdateRepoReq: types.UpdateRepoReq{Nickname: tea.String("ns")}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	reqn := *req
	reqn.Username = "u"
	reqn.Name = "r"
	reqn.Namespace = "u"
	tester.mocks.skill.EXPECT().Update(tester.Ctx(), &reqn).Return(&types.Skill{Name: "s"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Skill{Name: "s"})

}

func TestSkillHandler_Delete(t *testing.T) {
	tester := NewSkillTester(t).WithHandleFunc(func(cp *SkillHandler) gin.HandlerFunc {
		return cp.Delete
	})
	tester.WithUser()

	tester.mocks.skill.EXPECT().Delete(tester.Ctx(), "u", "r", "u").Return(nil)
	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestSkillHandler_Show(t *testing.T) {
	tester := NewSkillTester(t).WithHandleFunc(func(cp *SkillHandler) gin.HandlerFunc {
		return cp.Show
	})

	tester.mocks.skill.EXPECT().Show(tester.Ctx(), "u", "r", "u", false, false).Return(&types.Skill{Name: "s"}, nil)
	tester.WithUser().Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.Skill{Name: "s"})
}

func TestSkillHandler_Publish(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		publisher := &fakeSkillPublisher{
			resp: &types.PublishSkillVersionResp{
				Ok:        true,
				SkillID:   "1",
				VersionID: "2",
				Version:   "v1.0.0",
				Commit:    "abc123",
			},
		}
		tester := NewSkillTester(t).WithHandleFunc(func(h *SkillHandler) gin.HandlerFunc {
			h.publisher = publisher
			return h.Publish
		})
		tester.WithUser().WithBody(t, &types.PublishSkillVersionReq{
			Version:   "v1.0.0",
			Changelog: "Initial release",
		}).Execute()

		require.Equal(t, "u", publisher.req.Namespace)
		require.Equal(t, "r", publisher.req.Name)
		require.Equal(t, "u", publisher.req.Username)
		require.Equal(t, "v1.0.0", publisher.req.Version)
		tester.ResponseEq(t, 200, tester.OKText, publisher.resp)
	})

	t.Run("missing version", func(t *testing.T) {
		publisher := &fakeSkillPublisher{}
		tester := NewSkillTester(t).WithHandleFunc(func(h *SkillHandler) gin.HandlerFunc {
			h.publisher = publisher
			return h.Publish
		})
		tester.WithUser().WithBody(t, &types.PublishSkillVersionReq{
			Changelog: "Initial release",
		}).Execute()

		require.Nil(t, publisher.req)
		require.Equal(t, http.StatusBadRequest, tester.Response().Code)
	})
}

func TestSkillHandler_Relations(t *testing.T) {
	tester := NewSkillTester(t).WithHandleFunc(func(cp *SkillHandler) gin.HandlerFunc {
		return cp.Relations
	})

	tester.mocks.skill.EXPECT().Relations(tester.Ctx(), "u", "r", "u").Return(&types.Relations{}, nil)
	tester.WithUser().Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.Relations{})
}

func TestSkillHandler_GetUploadUrl(t *testing.T) {
	// Setup test configuration
	cfg := &config.Config{}
	cfg.S3.Bucket = "test-bucket"

	// Create mock components
	mockSkillComponent := mockcomponent.NewMockSkillComponent(t)
	mockSensitiveComponent := mockcomponent.NewMockSensitiveComponent(t)
	mockRepoComponent := mockcomponent.NewMockRepoComponent(t)

	// Expected upload URL, UUID, and form data
	expectedURL := "http://example.com/upload"
	expectedUUID := "test-uuid"
	expectedFormData := map[string]string{
		"key":             "skills/packages/test-uuid",
		"policy":          "test-policy",
		"x-amz-signature": "test-signature",
	}

	// Set up mock expectations
	mockSkillComponent.EXPECT().GetUploadUrl(mock.Anything).Return(expectedURL, expectedUUID, expectedFormData, nil)

	// Create skill handler with mock dependencies
	handler := &SkillHandler{
		skill:     mockSkillComponent,
		sensitive: mockSensitiveComponent,
		repo:      mockRepoComponent,
		config:    cfg,
	}

	// Create a test HTTP request
	req, err := http.NewRequest("POST", "/skills/upload_url", nil)
	require.Nil(t, err)
	// Set the current user header
	req.Header.Set("X-User", "test-user")

	// Create a test HTTP response recorder
	w := httptest.NewRecorder()

	// Create a gin context
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	// Set current user in context
	ctx.Set("currentUser", "test-user")

	// Call the GetUploadUrl method
	handler.GetUploadUrl(ctx)

	// Check the response
	require.Equal(t, http.StatusOK, w.Code)

	// Check the response body
	var response struct {
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.Nil(t, err)
	require.Equal(t, expectedURL, response.Data["url"])
	require.Equal(t, expectedUUID, response.Data["uuid"])

	// Convert formData to map[string]string for comparison
	formData, ok := response.Data["formData"].(map[string]interface{})
	require.True(t, ok)
	expectedFormDataMap := make(map[string]interface{})
	for k, v := range expectedFormData {
		expectedFormDataMap[k] = v
	}
	require.Equal(t, expectedFormDataMap, formData)
}
