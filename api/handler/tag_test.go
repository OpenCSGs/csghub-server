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
		}, 50, 1).Return(tags, 1, nil)

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
