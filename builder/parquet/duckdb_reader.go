package parquet

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"

	_ "github.com/marcboeker/go-duckdb"
	"opencsg.com/csghub-server/common/config"
)

type Reader interface {
	RowCount(objName string) (count int, err error)
	TopN(objName string, count int) (columns []string, rows [][]interface{}, err error)
}

type duckdbReader struct {
	db     *sql.DB
	bucket string
}

// NewS3Reader create a new reader to read from s3 compatible object storage service
func NewS3Reader(cfg *config.Config) (Reader, error) {
	s3SetupSql := fmt.Sprintf(`
	INSTALL httpfs;
	LOAD httpfs;
	SET s3_region = '%s';
	SET s3_endpoint = '%s';
	SET s3_url_style = 'vhost';
	SET s3_access_key_id = '%s';
	SET s3_secret_access_key = '%s';
	`, cfg.S3.Region, cfg.S3.Endpoint, cfg.S3.AccessKeyID, cfg.S3.AccessKeySecret)
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to duckdb, cause:%w", err)
	}
	slog.Debug("setup duckdb", slog.String("sql", s3SetupSql))
	_, err = db.Exec(s3SetupSql)
	if err != nil {
		return nil, fmt.Errorf("failed to setup s3 for duckdb, cause:%w", err)
	}
	slog.Info("setup duckdb succeeded")

	return &duckdbReader{db: db, bucket: cfg.S3.Bucket}, nil
}

// RowCount returns the total number of rows in a parquet file in S3 bucket.
func (r *duckdbReader) RowCount(objName string) (int, error) {
	selectCount := fmt.Sprintf("select count(*) from read_parquet('s3://%s/%s');", r.bucket, objName)
	fmt.Println(selectCount)
	row := r.db.QueryRow(selectCount)
	if row.Err() != nil {
		return 0, fmt.Errorf("failed to get row count: %w", row.Err())
	}
	var count int
	err := row.Scan(&count)
	return count, err
}

// TopN returns the top N rows of a parquet file in S3 bucket.
func (r *duckdbReader) TopN(objName string, count int) ([]string, [][]interface{}, error) {
	topN := fmt.Sprintf("select * from read_parquet('s3://%s/%s') limit %d;", r.bucket, objName, count)
	slog.Debug("query topN", slog.String("query", topN))
	rows, err := r.db.Query(topN)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute query,cause:%w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns,cause:%w", err)
	}
	slog.Debug("get parquet columns", slog.Any("columns", columns))
	values := make([][]interface{}, 0)

	for rows.Next() {
		fields := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range fields {
			pointers[i] = &fields[i]
		}

		// Scan values into the slice
		if err := rows.Scan(pointers...); err != nil {
			log.Fatal(err)
		}
		values = append(values, fields)
	}
	return columns, values, nil
}
