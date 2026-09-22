//go:build ee || saas

package component

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/config"
)

// newMockRewriteComponent creates a test component with a mock presigner injected.
func newMockRewriteComponent(bucket, endpoint string, mockPresigner presignURLer) *evaluationComponentImpl {
	cfg := &config.Config{}
	cfg.Argo.S3PublicBucket = bucket
	cfg.S3.Bucket = bucket
	cfg.S3.Endpoint = endpoint
	return &evaluationComponentImpl{
		config:    cfg,
		presigner: mockPresigner,
	}
}

func TestRewriteURLViaGateway_MockVirtualHostStyle(t *testing.T) {
	mockPresigner := mockcomponent.NewMockStorageGatewayComponent(t)
	mockPresigner.On("PresignURL",
		mock.Anything,
		"opencsg-public-resource",
		"evaluation/result.json",
		"GET",
		mock.AnythingOfType("time.Duration"),
	).Return("http://gateway/api/v1/storage/opencsg-public-resource/evaluation/result.json?X-Amz-Signature=abc", nil)

	c := newMockRewriteComponent("opencsg-public-resource", "oss-cn-beijing.aliyuncs.com", mockPresigner)
	original := "https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/evaluation/result.json"

	result := c.rewriteURLViaGateway(context.Background(), original)

	require.Equal(t, "http://gateway/api/v1/storage/opencsg-public-resource/evaluation/result.json?X-Amz-Signature=abc", result)
	mockPresigner.AssertCalled(t, "PresignURL",
		mock.Anything,
		"opencsg-public-resource",
		"evaluation/result.json",
		"GET",
		7*24*time.Hour,
	)
}

func TestRewriteURLViaGateway_MockPathStyle(t *testing.T) {
	mockPresigner := mockcomponent.NewMockStorageGatewayComponent(t)
	mockPresigner.On("PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/result.json",
		"GET",
		mock.AnythingOfType("time.Duration"),
	).Return("http://gateway/api/v1/storage/mybucket/evaluation/result.json?sig=xyz", nil)

	c := newMockRewriteComponent("mybucket", "minio.local:9000", mockPresigner)
	original := "http://minio.local:9000/mybucket/evaluation/result.json"

	result := c.rewriteURLViaGateway(context.Background(), original)

	require.Equal(t, "http://gateway/api/v1/storage/mybucket/evaluation/result.json?sig=xyz", result)
	mockPresigner.AssertCalled(t, "PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/result.json",
		"GET",
		7*24*time.Hour,
	)
}

func TestRewriteURLViaGateway_MockGatewayUploadStyle(t *testing.T) {
	mockPresigner := mockcomponent.NewMockStorageGatewayComponent(t)
	mockPresigner.On("PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/result.json",
		"GET",
		mock.AnythingOfType("time.Duration"),
	).Return("http://gateway/api/v1/storage/mybucket/evaluation/result.json?sig=def", nil)

	c := newMockRewriteComponent("mybucket", "minio.local:9000", mockPresigner)
	original := "http://csghub-server:8080/api/v1/storage/mybucket/evaluation/result.json"

	result := c.rewriteURLViaGateway(context.Background(), original)

	require.Equal(t, "http://gateway/api/v1/storage/mybucket/evaluation/result.json?sig=def", result)
	mockPresigner.AssertCalled(t, "PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/result.json",
		"GET",
		7*24*time.Hour,
	)
}

func TestRewriteURLViaGateway_MockPresignFails(t *testing.T) {
	mockPresigner := mockcomponent.NewMockStorageGatewayComponent(t)
	mockPresigner.On("PresignURL",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return("", fmt.Errorf("presign error"))

	c := newMockRewriteComponent("mybucket", "minio.local:9000", mockPresigner)
	original := "http://minio.local:9000/mybucket/evaluation/result.json"

	result := c.rewriteURLViaGateway(context.Background(), original)

	require.Equal(t, "", result)
}

func TestRewriteURLViaGateway_MockNilPresigner(t *testing.T) {
	c := newMockRewriteComponent("mybucket", "minio.local:9000", nil)
	original := "http://minio.local:9000/mybucket/evaluation/result.json"

	result := c.rewriteURLViaGateway(context.Background(), original)

	require.Equal(t, "", result)
}

func TestRewriteURLViaGateway_MockKeyWithNestedPath(t *testing.T) {
	mockPresigner := mockcomponent.NewMockStorageGatewayComponent(t)
	mockPresigner.On("PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/sub/deep/result.json",
		"GET",
		mock.AnythingOfType("time.Duration"),
	).Return("http://gateway/api/v1/storage/mybucket/evaluation/sub/deep/result.json?sig=nested", nil)

	c := newMockRewriteComponent("mybucket", "minio.local:9000", mockPresigner)
	original := "http://minio.local:9000/mybucket/evaluation/sub/deep/result.json"

	result := c.rewriteURLViaGateway(context.Background(), original)

	require.Equal(t, "http://gateway/api/v1/storage/mybucket/evaluation/sub/deep/result.json?sig=nested", result)
	mockPresigner.AssertCalled(t, "PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/sub/deep/result.json",
		"GET",
		7*24*time.Hour,
	)
}

func TestRewriteURLViaGateway_MockQueryStringStripped(t *testing.T) {
	mockPresigner := mockcomponent.NewMockStorageGatewayComponent(t)
	mockPresigner.On("PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/result.json",
		"GET",
		mock.AnythingOfType("time.Duration"),
	).Return("http://gateway/api/v1/storage/mybucket/evaluation/result.json?sig=q", nil)

	c := newMockRewriteComponent("mybucket", "minio.local:9000", mockPresigner)
	original := "http://minio.local:9000/mybucket/evaluation/result.json?versionId=abc"

	result := c.rewriteURLViaGateway(context.Background(), original)

	require.Equal(t, "http://gateway/api/v1/storage/mybucket/evaluation/result.json?sig=q", result)
	// Key should NOT contain the query string
	mockPresigner.AssertCalled(t, "PresignURL",
		mock.Anything,
		"mybucket",
		"evaluation/result.json",
		"GET",
		7*24*time.Hour,
	)
}
