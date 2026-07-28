package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcom "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewTestTagHandler(
	tagComp component.TagComponent,
) (*TagsHandler, error) {
	return &TagsHandler{
		tag: tagComp,
	}, nil
}

func TestTagHandler_AllTags(t *testing.T) {
	t.Run("no builtin", func(t *testing.T) {
		var tags []*types.RepoTag
		tags = append(tags, &types.RepoTag{Name: "test1"})

		values := url.Values{}
		values.Add("category", "task")
		values.Add("scope", "model")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes:     []types.TagScope{types.TagScope("model")},
			Categories: []string{"task"},
			BuiltIn:    nil,
		}, 0, 0).Return(tags, 1, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R

		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)

		require.Equal(t, "", resp.Code)
		require.Equal(t, "OK", resp.Msg)
		require.NotNil(t, resp.Data)
		require.Equal(t, 1, resp.Total)
	})

	t.Run("with builtin", func(t *testing.T) {
		var tags []*types.RepoTag
		tags = append(tags, &types.RepoTag{Name: "test1"})

		values := url.Values{}
		values.Add("category", "task")
		values.Add("scope", "model")
		values.Add("built_in", "true")
		values.Add("per", "10")
		values.Add("page", "2")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		builtin := true
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes:     []types.TagScope{types.TagScope("model")},
			Categories: []string{"task"},
			BuiltIn:    &builtin,
		}, 10, 2).Return(tags, 15, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R

		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)

		require.Equal(t, "", resp.Code)
		require.Equal(t, "OK", resp.Msg)
		require.NotNil(t, resp.Data)
		require.Equal(t, 15, resp.Total)
	})

	t.Run("invalid per", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/tags?per=101", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)
		require.Equal(t, http.StatusBadRequest, hr.Code)
	})

	t.Run("invalid page", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/tags?page=0", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)
		require.Equal(t, http.StatusBadRequest, hr.Code)
	})

	t.Run("non-numeric per", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/tags?per=abc", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)
		require.Equal(t, http.StatusBadRequest, hr.Code)
	})

	t.Run("server error", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/tags?per=10&page=1", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), mock.Anything, 10, 1).Return(nil, 0, fmt.Errorf("db error"))

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)
		require.Equal(t, http.StatusInternalServerError, hr.Code)
	})

	t.Run("empty result", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/tags?search=none", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), mock.Anything, 50, 1).Return(nil, 0, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)
		require.Equal(t, http.StatusOK, hr.Code)

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 0, resp.Total)
	})
}

// makeTags builds a slice of n []*types.RepoTag for test fixtures.
func makeTags(n int) []*types.RepoTag {
	tags := make([]*types.RepoTag, n)
	for i := 0; i < n; i++ {
		tags[i] = &types.RepoTag{Name: fmt.Sprintf("tag%d", i)}
	}
	return tags
}

// requireTagsLen unmarshals the response envelope from hr, re-marshals the
// Data field and unmarshals it into []types.RepoTag, then asserts that the
// number of returned tags equals wantLen.
func requireTagsLen(t *testing.T, hr *httptest.ResponseRecorder, wantLen int) {
	t.Helper()
	var resp httpbase.R
	err := json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	dataBytes, err := json.Marshal(resp.Data)
	require.Nil(t, err)

	var tags []types.RepoTag
	err = json.Unmarshal(dataBytes, &tags)
	require.Nil(t, err)

	require.Len(t, tags, wantLen)
}

// TestTagHandler_AllTags_HomepagePagination covers the boundary cases around
// the homepage "fetch all" behavior. A homepage request is identified by having
// a scope filter set. When "per" is not sent on such a request, per and page
// are both 0 so AllTagsWithPagination returns all tags without pagination.
// When "per" is explicitly sent, or when no scope is set, standard pagination
// applies.
//
// Each subtest verifies both the total count and the actual number of tags
// returned in resp.Data to ensure the per/page values are passed through
// correctly to the component layer.
func TestTagHandler_AllTags_HomepagePagination(t *testing.T) {
	t.Run("scope set, no per -> fetch all (per=0,page=0)", func(t *testing.T) {
		// 23 tags total, no pagination applied -> all 23 returned
		tags := makeTags(23)

		values := url.Values{}
		values.Add("scope", "model")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes: []types.TagScope{types.TagScope("model")},
		}, 0, 0).Return(tags, 23, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 23, resp.Total)
		requireTagsLen(t, hr, 23)
	})

	t.Run("scope set, with per -> standard pagination, full page", func(t *testing.T) {
		// per=10, page=2, total=23 -> page 2 returns a full page of 10 tags
		tags := makeTags(10)

		values := url.Values{}
		values.Add("scope", "model")
		values.Add("per", "10")
		values.Add("page", "2")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes: []types.TagScope{types.TagScope("model")},
		}, 10, 2).Return(tags, 23, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 23, resp.Total)
		requireTagsLen(t, hr, 10)
	})

	t.Run("scope set, with per -> last page returns remainder", func(t *testing.T) {
		// per=10, page=3, total=23 -> page 3 returns only 3 tags (23-20=3)
		tags := makeTags(3)

		values := url.Values{}
		values.Add("scope", "model")
		values.Add("per", "10")
		values.Add("page", "3")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes: []types.TagScope{types.TagScope("model")},
		}, 10, 3).Return(tags, 23, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 23, resp.Total)
		requireTagsLen(t, hr, 3)
	})

	t.Run("scope set, with per -> first page of single page", func(t *testing.T) {
		// per=10, page=1, total=5 -> only 5 tags, less than per
		tags := makeTags(5)

		values := url.Values{}
		values.Add("scope", "model")
		values.Add("per", "10")
		values.Add("page", "1")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes: []types.TagScope{types.TagScope("model")},
		}, 10, 1).Return(tags, 5, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 5, resp.Total)
		requireTagsLen(t, hr, 5)
	})

	t.Run("no scope, no per -> standard pagination (default 50, page 1)", func(t *testing.T) {
		// default per=50, page=1, total=30 -> all 30 fit in one page
		tags := makeTags(30)

		req := httptest.NewRequest("get", "/api/v1/tags", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{}, 50, 1).Return(tags, 30, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 30, resp.Total)
		requireTagsLen(t, hr, 30)
	})

	t.Run("no scope, with per -> standard pagination", func(t *testing.T) {
		// per=10, page=2, total=15 -> page 2 returns 5 tags (15-10=5)
		tags := makeTags(5)

		values := url.Values{}
		values.Add("per", "10")
		values.Add("page", "2")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{}, 10, 2).Return(tags, 15, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 15, resp.Total)
		requireTagsLen(t, hr, 5)
	})

	t.Run("built_in set, no scope, no per -> standard pagination", func(t *testing.T) {
		// default per=50, page=1, total=8 -> all 8 returned
		tags := makeTags(8)

		values := url.Values{}
		values.Add("built_in", "true")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		builtin := true
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			BuiltIn: &builtin,
		}, 50, 1).Return(tags, 8, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 8, resp.Total)
		requireTagsLen(t, hr, 8)
	})

	t.Run("scope and built_in set, no per -> fetch all (per=0,page=0)", func(t *testing.T) {
		// 12 tags total, no pagination -> all 12 returned
		tags := makeTags(12)

		values := url.Values{}
		values.Add("scope", "dataset")
		values.Add("built_in", "false")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		builtin := false
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes:  []types.TagScope{types.TagScope("dataset")},
			BuiltIn: &builtin,
		}, 0, 0).Return(tags, 12, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 12, resp.Total)
		requireTagsLen(t, hr, 12)
	})

	t.Run("scope set, per exceeds max -> bad request", func(t *testing.T) {
		values := url.Values{}
		values.Add("scope", "model")
		values.Add("per", "101")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)
		require.Equal(t, http.StatusBadRequest, hr.Code)
	})

	t.Run("multiple scopes set, no per -> fetch all (per=0,page=0)", func(t *testing.T) {
		// 7 tags total, no pagination -> all 7 returned
		tags := makeTags(7)

		values := url.Values{}
		values.Add("scope", "model")
		values.Add("scope", "dataset")
		req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), &types.TagFilter{
			Scopes: []types.TagScope{types.TagScope("model"), types.TagScope("dataset")},
		}, 0, 0).Return(tags, 7, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 7, resp.Total)
		requireTagsLen(t, hr, 7)
	})

	t.Run("empty result -> 0 tags in data", func(t *testing.T) {
		// per=10, page=1, total=0 -> no tags returned
		req := httptest.NewRequest("get", "/api/v1/tags?per=10&page=1", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		tagComp := mockcom.NewMockTagComponent(t)
		tagComp.EXPECT().AllTagsWithPagination(ginContext.Request.Context(), mock.Anything, 10, 1).Return(nil, 0, nil)

		tagHandler, err := NewTestTagHandler(tagComp)
		require.Nil(t, err)

		tagHandler.AllTags(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err = json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, 0, resp.Total)
		requireTagsLen(t, hr, 0)
	})
}

func TestTagHandler_CreateTag(t *testing.T) {
	data := types.CreateTag{
		Name:     "testtag",
		Scope:    "testscope",
		Category: "testcategory",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("post", "/api/v1/tags", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().CreateTag(ginContext.Request.Context(), mock.Anything).Return(&database.Tag{ID: 1, Name: "testtag"}, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.CreateTag(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_GetTagByID(t *testing.T) {
	req := httptest.NewRequest("get", "/api/v1/tags/1", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().GetTagByID(ginContext.Request.Context(), int64(1)).Return(&database.Tag{ID: 1, Name: "test1"}, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.GetTagByID(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_UpdateTag(t *testing.T) {
	data := types.UpdateTag{
		Name:     "testtag",
		Scope:    "testscope",
		Category: "testcategory",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("put", "/api/v1/tags/1", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().UpdateTag(ginContext.Request.Context(), int64(1), mock.Anything).Return(&database.Tag{ID: 1, Name: "testtag"}, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.UpdateTag(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_DeleteTag(t *testing.T) {
	req := httptest.NewRequest("delete", "/api/v1/tags/1", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().DeleteTag(ginContext.Request.Context(), int64(1)).Return(nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.DeleteTag(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "", resp.Msg)
	require.Nil(t, resp.Data)
}

func TestTagHandler_AllCategories(t *testing.T) {
	var categories []types.RepoTagCategory
	categories = append(categories, types.RepoTagCategory{ID: 1, Name: "test1", Scope: types.TagScope("scope")})

	req := httptest.NewRequest("get", "/tags/categories", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().AllCategories(ginContext.Request.Context()).Return(categories, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.AllCategories(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "OK", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_CreateCategory(t *testing.T) {
	data := types.CreateCategory{
		Name:  "testcate",
		Scope: "testscope",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("post", "/tags/categories", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().CreateCategory(ginContext.Request.Context(), data).Return(
		&database.TagCategory{ID: 1, Name: "testcate", Scope: types.TagScope("testscope")},
		nil,
	)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.CreateCategory(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_UpdateCategory(t *testing.T) {
	data := types.UpdateCategory{
		Name:  "testcate",
		Scope: "testscope",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("put", "/tags/categories/1", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().UpdateCategory(ginContext.Request.Context(), data, int64(1)).Return(
		&database.TagCategory{ID: 1, Name: "testcate", Scope: types.TagScope("testscope")},
		nil,
	)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.UpdateCategory(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_DeleteCategory(t *testing.T) {
	req := httptest.NewRequest("delete", "/tags/categories/1", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().DeleteCategory(ginContext.Request.Context(), int64(1)).Return(nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.DeleteCategory(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, "", resp.Code)
	require.Equal(t, "", resp.Msg)
}
