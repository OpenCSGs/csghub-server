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
