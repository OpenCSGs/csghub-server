package parquet

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/XSAM/otelsql"
	_ "github.com/marcboeker/go-duckdb"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type Reader interface {
	RowCount(ctx context.Context, objNames []string, req types.QueryReq, lfs bool) (count int, err error)
	TopN(ctx context.Context, objName string, count int) (columns []string, columnsType []string, rows [][]interface{}, err error)
	FetchRows(ctx context.Context, objNames []string, req types.QueryReq, lfs bool) (columns []string, columnsType []string, rows [][]interface{}, err error)
}

type duckdbReader struct {
	db     *sql.DB
	bucket string
}

func NewS3Reader(ctx context.Context, cfg *config.Config) (Reader, error) {
	urlStyle := "vhost"
	if cfg.S3.BucketLookup == "path" {
		urlStyle = "path"
	}
	s3SetupSql := fmt.Sprintf(`
	INSTALL httpfs;
	LOAD httpfs;
	INSTALL json;
	LOAD json;
	SET s3_region = '%s';
	SET s3_endpoint = '%s';
	SET s3_access_key_id = '%s';
	SET s3_secret_access_key = '%s';
	SET s3_use_ssl = %t;
	SET s3_url_style = '%s';
	`, cfg.S3.Region, cfg.S3.Endpoint, cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret, cfg.S3.EnableSSL, urlStyle)
	db, err := otelsql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to duckdb, cause:%w", err)
	}
	err = otelsql.RegisterDBStatsMetrics(db)
	if err != nil {
		return nil, fmt.Errorf("failed to report db stats metrics, cause:%w", err)
	}
	_, err = db.ExecContext(ctx, s3SetupSql)
	if err != nil {
		return nil, fmt.Errorf("failed to setup s3 for duckdb, cause:%w", err)
	}
	slog.Debug("setup duckdb succeeded")
	return &duckdbReader{db: db, bucket: cfg.S3.Bucket}, nil
}

func (r *duckdbReader) RowCount(ctx context.Context, objNames []string, req types.QueryReq, lfs bool) (int, error) {
	multiFiles := r.genSelectMultiObjStr(objNames, lfs)
	whereStr := ""
	if len(req.Where) > 0 {
		whereStr = fmt.Sprintf("WHERE %s", req.Where)
	}
	selectCount := fmt.Sprintf("SELECT count(*) FROM read_parquet(%s, union_by_name = true) %s;", multiFiles, whereStr)
	row := r.db.QueryRowContext(ctx, selectCount)
	if row.Err() != nil {
		return 0, fmt.Errorf("failed to get row count: %w", row.Err())
	}
	var count int
	err := row.Scan(&count)
	return count, err
}

func (r *duckdbReader) TopN(ctx context.Context, objName string, count int) ([]string, []string, [][]interface{}, error) {
	topN := fmt.Sprintf("SELECT * FROM read_parquet('s3://%s/%s') limit %d;", r.bucket, objName, count)
	slog.Debug("query topN", slog.String("query", topN))
	rows, err := r.db.QueryContext(ctx, topN)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to execute query,cause:%w", err)
	}
	defer rows.Close()
	return r.getColumnsAndValues(rows)
}

func (r *duckdbReader) FetchRows(ctx context.Context, objNames []string, req types.QueryReq, lfs bool) ([]string, []string, [][]interface{}, error) {
	multiFiles := r.genSelectMultiObjStr(objNames, lfs)
	offset := (req.PageIndex - 1) * req.PageSize
	whereStr := ""
	if len(req.Where) > 0 {
		whereStr = fmt.Sprintf("WHERE %s", req.Where)
	}
	orderbyStr := ""
	if len(strings.Trim(req.Orderby, " ")) > 0 {
		orderbyStr = "order by " + req.Orderby
	}
	querySql := fmt.Sprintf("SELECT * FROM read_parquet(%s, union_by_name = true) %s %s limit %d offset %d;", multiFiles, whereStr, orderbyStr, req.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, querySql)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to execute fetch rows, cause:%w", err)
	}
	defer rows.Close()
	return r.getColumnsAndValues(rows)
}

func (r *duckdbReader) getColumnsAndValues(rows *sql.Rows) ([]string, []string, [][]interface{}, error) {
	colsType, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get duckdb query columns type, cause:%w", err)
	}

	var (
		columns     []string
		columnsType []string
	)
	for _, columnType := range colsType {
		columns = append(columns, columnType.Name())
		columnsType = append(columnsType, columnType.ScanType().Name())
	}
	values := make([][]interface{}, 0)

	for rows.Next() {
		fields := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range fields {
			pointers[i] = &fields[i]
		}
		// Scan values into the slice
		if err := rows.Scan(pointers...); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to scan rows, cause:%w", err)
			// log.Fatal(err)
		}
		values = append(values, fields)
	}
	return columns, columnsType, values, nil
}

func (r *duckdbReader) genSelectMultiObjStr(objNames []string, lfs bool) string {
	var parquetFiles []string
	for _, objName := range objNames {
		if lfs {
			parquetFiles = append(parquetFiles, fmt.Sprintf("'s3://%s/%s'", r.bucket, objName))
		} else {
			parquetFiles = append(parquetFiles, fmt.Sprintf("'%s'", objName))
		}
	}
	fileStr := ""
	if len(objNames) > 1 {
		fileStr = "[" + strings.Join(parquetFiles, ",") + "]"
	} else {
		fileStr = parquetFiles[0]
	}

	return fileStr
}
