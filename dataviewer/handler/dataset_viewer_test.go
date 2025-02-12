package handler

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/dataviewer/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/dataviewer/common"
)

type DatasetViewerTester struct {
	*testutil.GinTester
	handler *DatasetViewerHandler
	mocks   struct {
		datasetViewer *mockcomponent.MockDatasetViewerComponent
	}
}

func NewDatasetViewerTester(t *testing.T) *DatasetViewerTester {
	tester := &DatasetViewerTester{GinTester: testutil.NewGinTester()}
	tester.mocks.datasetViewer = mockcomponent.NewMockDatasetViewerComponent(t)

	tester.handler = &DatasetViewerHandler{
		viewer: tester.mocks.datasetViewer,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *DatasetViewerTester) WithHandleFunc(fn func(h *DatasetViewerHandler) gin.HandlerFunc) *DatasetViewerTester {
	t.Handler(fn(t.handler))
	return t
}

func TestDatasetViewerHandler_View(t *testing.T) {
	tester := NewDatasetViewerTester(t).WithHandleFunc(func(h *DatasetViewerHandler) gin.HandlerFunc {
		return h.View
	})

	tester.mocks.datasetViewer.EXPECT().ViewParquetFile(
		tester.Gctx().Request.Context(),
		&common.ViewParquetFileReq{
			Namespace:   "u",
			RepoName:    "r",
			Path:        "foo",
			Per:         12,
			Page:        6,
			CurrentUser: "u",
		},
	).Return(&common.ViewParquetFileResp{Total: 123}, nil)
	tester.AddPagination(6, 12).WithParam("file_path", "foo").WithUser().Execute()
	tester.ResponseEq(t, 200, tester.OKText, &common.ViewParquetFileResp{Total: 123})

}

func TestDatasetViewerHandler_Catalog(t *testing.T) {
	tester := NewDatasetViewerTester(t).WithHandleFunc(func(h *DatasetViewerHandler) gin.HandlerFunc {
		return h.Catalog
	})

	tester.mocks.datasetViewer.EXPECT().GetCatalog(
		tester.Gctx().Request.Context(),
		&common.ViewParquetFileReq{
			Namespace:   "u",
			RepoName:    "r",
			CurrentUser: "u",
		},
	).Return(&common.CataLogRespone{Status: 6}, nil)
	tester.WithUser().Execute()
	tester.ResponseEq(t, 200, tester.OKText, &common.CataLogRespone{Status: 6})

}

func TestDatasetViewerHandler_Rows(t *testing.T) {

	t.Run("limit offset rows", func(t *testing.T) {
		tester := NewDatasetViewerTester(t).WithHandleFunc(func(h *DatasetViewerHandler) gin.HandlerFunc {
			return h.Rows
		})

		tester.mocks.datasetViewer.EXPECT().LimitOffsetRows(
			tester.Gctx().Request.Context(),
			&common.ViewParquetFileReq{
				Namespace:   "u",
				RepoName:    "r",
				CurrentUser: "u",
				Per:         12,
				Page:        6,
			}, types.DataViewerReq{
				Config: "a",
				Split:  "b",
			},
		).Return(&common.ViewParquetFileResp{Total: 12}, nil)
		tester.AddPagination(6, 12).WithQuery("config", "a").WithQuery("split", "b")
		tester.WithUser().Execute()
		tester.ResponseEq(t, 200, tester.OKText, &common.ViewParquetFileResp{Total: 12})
	})

	for _, param := range []string{"search", "where", "orderby"} {

		t.Run(fmt.Sprintf("%s rows", param), func(t *testing.T) {
			tester := NewDatasetViewerTester(t).WithHandleFunc(func(h *DatasetViewerHandler) gin.HandlerFunc {
				return h.Rows
			})

			tp := types.DataViewerReq{
				Config: "a",
				Split:  "b",
			}
			switch param {
			case "search":
				tp.Search = "c"
			case "where":
				tp.Where = "c"
			case "orderby":
				tp.Orderby = "c"
			}

			tester.mocks.datasetViewer.EXPECT().Rows(
				tester.Gctx().Request.Context(),
				&common.ViewParquetFileReq{
					Namespace:   "u",
					RepoName:    "r",
					CurrentUser: "u",
					Per:         12,
					Page:        6,
				}, tp,
			).Return(&common.ViewParquetFileResp{Total: 12}, nil)
			tester.AddPagination(6, 12).WithQuery("config", "a").WithQuery("split", "b")
			tester.WithQuery(param, "c").WithUser().Execute()
			tester.ResponseEq(t, 200, tester.OKText, &common.ViewParquetFileResp{Total: 12})
		})
	}

}
