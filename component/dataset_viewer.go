package component

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/parquet"
	"opencsg.com/starhub-server/common/config"
)

const ossLfsBucketName = "opencsg-test"

type ViewParquetFileReq struct {
	Namespace string `json:"namespace"`
	RepoName  string `json:"name"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
	RowCount  int    `json:"row_count"`
}
type ViewParquetFileResp struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}
type DatasetViewerComponent struct {
	gs      gitserver.GitServer
	preader parquet.Reader
}

func NewDatasetViewerComponent(cfg *config.Config) (*DatasetViewerComponent, error) {
	gs, err := gitserver.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server,cause:%w", err)
	}
	r, err := parquet.NewS3Reader(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet reader,cause:%w", err)
	}
	return &DatasetViewerComponent{
		gs:      gs,
		preader: r,
	}, nil
}

func (c *DatasetViewerComponent) ViewParquetFile(ctx context.Context, req *ViewParquetFileReq) (*ViewParquetFileResp, error) {
	bucketName := ossLfsBucketName
	objName, err := c.getParquetObject(req)
	if err != nil {
		slog.Error("Failed to view parquet file", slog.Any("error", err))
		return nil, err
	}
	rowCount := req.RowCount
	if rowCount < 1 {
		rowCount = 20
	} else if rowCount > 100 {
		rowCount = 100
	}
	columns, rows, err := c.preader.TopN(bucketName, objName, rowCount)
	if err != nil {
		slog.Error("Failed to view parquet file", slog.Any("error", err))
		return nil, err
	}
	resp := &ViewParquetFileResp{
		Columns: columns,
		Rows:    rows,
	}
	return resp, nil
}

func (c *DatasetViewerComponent) getParquetObject(req *ViewParquetFileReq) (string, error) {
	f, err := c.gs.GetDatasetFileContents(req.Namespace, req.RepoName, req.Branch, req.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get file contents,cause:%v", err)
	}

	return "lfs/" + f.LfsRelativePath, nil
}
