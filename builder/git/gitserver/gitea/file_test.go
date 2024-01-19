package gitea

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"testing"
	"time"

	_ "github.com/marcboeker/go-duckdb"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestGetFileContents(t *testing.T) {
	// cfg := &config.Config{}
	// cfg.GitServer.Type = "gitea"

	// cfg.GitServer.Host = "https://hub-stg.opencsg.com/admin"
	// cfg.GitServer.URL = "https://hub-stg.opencsg.com/admin"
	// cfg.GitServer.SecretKey = "6250835ebf583192861f96ac01847c8dfdbd7187"

	// // cfg.GitServer.Host = "http://localhost:3000/"
	// // cfg.GitServer.URL = "http://localhost:3000/"
	// // cfg.GitServer.SecretKey = "cbcfa2497b51a6f75adcff8421a6b8da808fe505"
	// cfg.GitServer.Username = "root"
	// cfg.GitServer.Password = "password123"

	// c, err := NewClient(cfg)
	// if err != nil {
	// 	fmt.Println(err)
	// 	t.FailNow()
	// }

	// start := time.Now()
	// // c.GetModelFileTree("wayne", "phi-2", "", "")
	// fl, _ := c.GetModelFileTree("leida", "testrepo", "", "")
	// du := time.Since(start)
	// fmt.Println("du in ms: ", du.Milliseconds())
	// buf, _ := json.Marshal(fl)
	// fmt.Println(string(buf))

	// return
	// f, err := c.GetDatasetFileContents("aaa_bb_nn", "test_data_2", "main", "awesome-chatgpt-prompts.parquet")
	// if err != nil {
	// 	t.Error(err)
	// }
	// t.Log(f)
	// // t.Fail()

	// client, err := oss.New("oss-cn-beijing.aliyuncs.com",
	// 	"LTAI5tDpoLDFzA5umg2ezTb8", "mMDY4uma2NLpxmrpPaAo1QdwpaTzv7")
	// // "LTAI5tQdL9KBM2WU1DDASunB", "28xARXi1RaRpxe4B8lTeWGX3crfsTC")
	// if err != nil {
	// 	slog.Error("Failed to connect", slog.Any("error", err))
	// 	return
	// }
	s3Client, err := minio.New("oss-cn-beijing.aliyuncs.com", &minio.Options{
		Creds:        credentials.NewStaticV4("LTAI5tDpoLDFzA5umg2ezTb8", "mMDY4uma2NLpxmrpPaAo1QdwpaTzv7", ""),
		Secure:       true,
		BucketLookup: minio.BucketLookupAuto,
		// Region:       config.S3.Region,
	})

	// bucketName := "opencsg-test"
	bucketName := "opencsg-gitea-lfs"
	objName := "lfs/81/72/417116ff18bb0d1ecc2166a658ebabf79a6c9cf02d8cad411fa764548663"
	// bucket, err := client.Bucket(bucketName)
	// dowloadUrl, err := bucket.SignURL(objName, oss.HTTPGet, 600)

	// Set request parameters for content-disposition.
	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "attachment;filename=test.json")

	downloadUrl, err := s3Client.PresignedGetObject(context.Background(), bucketName, objName, time.Minute*10, reqParams)
	// fmt.Println(dowloadUrl, err)

	if err != nil {
		slog.Error("Failed to get bucket", slog.String("bucket", bucketName))
		t.FailNow()
	}

	// opt := oss.ContentDisposition(`attachment; filename="test.json"`)
	// downloadUrl, err := bucket.SignURL(objName, oss.HTTPGet, 300, opt)

	// if err != nil {
	// 	slog.Error("Failed to get object metadata", slog.String("object", objName))
	// 	t.FailNow()
	// }
	fmt.Println(downloadUrl)
	// fmt.Printf("=== %+v", *downloadUrl)
	return

}
