package handler

import (
	"bytes"
	"encoding/json"
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
		tc: tagComp,
	}, nil
}

func TestTagHandler_AllTags(t *testing.T) {
	var tags []*database.Tag
	tags = append(tags, &database.Tag{ID: 1, Name: "test1"})

	values := url.Values{}
	values.Add("category", "testcate")
	values.Add("scope", "testscope")
	req := httptest.NewRequest("get", "/api/v1/tags?"+values.Encode(), nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().AllTagsByScopeAndCategory(ginContext, "testscope", "testcate").Return(tags, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.AllTags(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_CreateTag(t *testing.T) {
	username := "testuser"
	data := types.CreateTag{
		Name:     "testtag",
		Scope:    "testscope",
		Category: "testcategory",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("post", "/api/v1/tags", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Set("currentUser", username)
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().CreateTag(ginContext, username, mock.Anything).Return(&database.Tag{ID: 1, Name: "testtag"}, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.CreateTag(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_GetTagByID(t *testing.T) {
	username := "testuser"

	req := httptest.NewRequest("get", "/api/v1/tags/1", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Set("currentUser", username)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().GetTagByID(ginContext, username, int64(1)).Return(&database.Tag{ID: 1, Name: "test1"}, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.GetTagByID(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_UpdateTag(t *testing.T) {
	username := "testuser"
	data := types.UpdateTag{
		Name:     "testtag",
		Scope:    "testscope",
		Category: "testcategory",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("put", "/api/v1/tags/1", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Set("currentUser", username)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().UpdateTag(ginContext, username, int64(1), mock.Anything).Return(&database.Tag{ID: 1, Name: "testtag"}, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.UpdateTag(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_DeleteTag(t *testing.T) {
	username := "testuser"

	req := httptest.NewRequest("delete", "/api/v1/tags/1", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Set("currentUser", username)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().DeleteTag(ginContext, username, int64(1)).Return(nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.DeleteTag(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.Nil(t, resp.Data)
}

func TestTagHandler_AllCategories(t *testing.T) {
	var categories []database.TagCategory
	categories = append(categories, database.TagCategory{ID: 1, Name: "test1", Scope: database.TagScope("scope")})

	req := httptest.NewRequest("get", "/tags/categories", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().AllCategories(ginContext).Return(categories, nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.AllCategories(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_CreateCategory(t *testing.T) {
	username := "testuser"
	data := types.CreateCategory{
		Name:  "testcate",
		Scope: "testscope",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("post", "/tags/categories", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Set("currentUser", username)
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().CreateCategory(ginContext, username, data).Return(
		&database.TagCategory{ID: 1, Name: "testcate", Scope: database.TagScope("testscope")},
		nil,
	)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.CreateCategory(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_UpdateCategory(t *testing.T) {
	username := "testuser"
	data := types.UpdateCategory{
		Name:  "testcate",
		Scope: "testscope",
	}

	reqBody, _ := json.Marshal(data)

	req := httptest.NewRequest("put", "/tags/categories/1", bytes.NewBuffer(reqBody))

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Set("currentUser", username)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().UpdateCategory(ginContext, username, data, int64(1)).Return(
		&database.TagCategory{ID: 1, Name: "testcate", Scope: database.TagScope("testscope")},
		nil,
	)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.UpdateCategory(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
	require.NotNil(t, resp.Data)
}

func TestTagHandler_DeleteCategory(t *testing.T) {
	username := "testuser"

	req := httptest.NewRequest("delete", "/tags/categories/1", nil)

	hr := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(hr)
	ginContext.Set("currentUser", username)
	ginContext.AddParam("id", "1")
	ginContext.Request = req

	tagComp := mockcom.NewMockTagComponent(t)
	tagComp.EXPECT().DeleteCategory(ginContext, username, int64(1)).Return(nil)

	tagHandler, err := NewTestTagHandler(tagComp)
	require.Nil(t, err)

	tagHandler.DeleteCategory(ginContext)

	require.Equal(t, http.StatusOK, hr.Code)

	var resp httpbase.R

	err = json.Unmarshal(hr.Body.Bytes(), &resp)
	require.Nil(t, err)

	require.Equal(t, 0, resp.Code)
	require.Equal(t, "", resp.Msg)
}
