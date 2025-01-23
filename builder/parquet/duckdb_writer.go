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
)

type Writer interface {
	ConvertToParquet(ctx context.Context, method string, objNames []string, thread int, path string) error
}

type duckdbWriter struct {
	db     *sql.DB
	bucket string
}

func NewS3Writer(ctx context.Context, cfg *config.Config) (Writer, error) {
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
	return &duckdbWriter{db: db, bucket: cfg.S3.Bucket}, nil
}

func (r *duckdbWriter) ConvertToParquet(ctx context.Context, method string, objNames []string, thread int, path string) error {
	fileStr := ""
	if len(objNames) > 1 {
		fileStr = "[" + strings.Join(objNames, ",") + "]"
	} else {
		fileStr = objNames[0]
	}
	exportCMD := fmt.Sprintf(`
	SET preserve_insertion_order=false;
	SET threads = %d; 
	COPY (SELECT * FROM %s(%s)) TO '%s' (
		FORMAT 'PARQUET', 
		CODEC 'Snappy', 
		PER_THREAD_OUTPUT TRUE, 
		FILENAME_PATTERN '{i}' 
	);
	`, thread, method, fileStr, path)
	slog.Debug("export parquet sql", slog.String("sql", exportCMD))
	_, err := r.db.ExecContext(ctx, exportCMD)
	if err != nil {
		return fmt.Errorf("failed to export parquet: %w", err)
	}
	return nil
}
