package sensitive

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	green20220302 "github.com/alibabacloud-go/green-20220302/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/common/types"
)

// mockS3Client is a test double for the s3Client interface.
type mockS3Client struct {
	mock.Mock
}

func (m *mockS3Client) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func (m *mockS3Client) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	return m.Called(ctx, bucketName, objectName, opts).Error(0)
}

// mockGreen2022 is a minimal test double for Green2022Client.
type mockGreen2022 struct {
	mock.Mock
}

func (m *mockGreen2022) GetRegionId() string {
	return m.Called().String(0)
}

func (m *mockGreen2022) TextModeration(request *green20220302.TextModerationRequest) (*green20220302.TextModerationResponse, error) {
	args := m.Called(request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*green20220302.TextModerationResponse), args.Error(1)
}

func (m *mockGreen2022) ImageModeration(request *green20220302.ImageModerationRequest) (*green20220302.ImageModerationResponse, error) {
	args := m.Called(request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*green20220302.ImageModerationResponse), args.Error(1)
}

func (m *mockGreen2022) TextModerationPlusWithOptions(request *green20220302.TextModerationPlusRequest, options *util.RuntimeOptions) (*green20220302.TextModerationPlusResponse, error) {
	args := m.Called(request, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*green20220302.TextModerationPlusResponse), args.Error(1)
}

func TestPassImageStreamCheck_NilS3Client(t *testing.T) {
	checker := NewAliyunChecker(nil, &mockGreen2022{})
	_, err := checker.PassImageStreamCheck(context.Background(), types.ScenarioImageBaseLineCheck, strings.NewReader("image-data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPassImageStreamCheck_UploadFailed(t *testing.T) {
	g2c := &mockGreen2022{}
	mc := &mockS3Client{}
	mc.On("PutObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything, int64(-1), mock.Anything).
		Return(minio.UploadInfo{}, errors.New("upload failed")).Once()
	mc.On("RemoveObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything).
		Return(nil).Once()

	checker := NewAliyunCheckerWithS3(nil, g2c, mc, "test-bucket")
	_, err := checker.PassImageStreamCheck(context.Background(), types.ScenarioImageBaseLineCheck, strings.NewReader("image-data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload")
}

func TestPassImageStreamCheck_PassCheck(t *testing.T) {
	g2c := &mockGreen2022{}
	mc := &mockS3Client{}
	mc.On("PutObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything, int64(-1), mock.MatchedBy(func(opts minio.PutObjectOptions) bool {
		// Verify Expires is set to ~30 minutes from now
		return !opts.Expires.IsZero() && opts.Expires.After(time.Now().Add(29*time.Minute)) && opts.Expires.Before(time.Now().Add(31*time.Minute))
	})).Return(minio.UploadInfo{}, nil).Once()
	mc.On("RemoveObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything).
		Return(nil).Once()
	g2c.On("GetRegionId").Return("cn-beijing").Once()
	g2c.On("ImageModeration", mock.Anything).Return(&green20220302.ImageModerationResponse{
		StatusCode: tea.Int32(200),
		Body: &green20220302.ImageModerationResponseBody{
			Code:      tea.Int32(200),
			RequestId: tea.String("req-1"),
			Data: &green20220302.ImageModerationResponseBodyData{
				Result: []*green20220302.ImageModerationResponseBodyDataResult{},
			},
		},
	}, nil).Once()

	checker := NewAliyunCheckerWithS3(nil, g2c, mc, "test-bucket")
	result, err := checker.PassImageStreamCheck(context.Background(), types.ScenarioImageBaseLineCheck, strings.NewReader("image-data"))
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsSensitive)
	mc.AssertExpectations(t)
}

func TestPassImageStreamCheck_SensitiveDetected(t *testing.T) {
	g2c := &mockGreen2022{}
	mc := &mockS3Client{}
	mc.On("PutObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything, int64(-1), mock.Anything).
		Return(minio.UploadInfo{}, nil).Once()
	mc.On("RemoveObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything).
		Return(nil).Once()
	g2c.On("GetRegionId").Return("cn-beijing").Once()
	g2c.On("ImageModeration", mock.Anything).Return(&green20220302.ImageModerationResponse{
		StatusCode: tea.Int32(200),
		Body: &green20220302.ImageModerationResponseBody{
			Code:      tea.Int32(200),
			RequestId: tea.String("req-2"),
			Data: &green20220302.ImageModerationResponseBodyData{
				Result: []*green20220302.ImageModerationResponseBodyDataResult{
					{
						Label:      tea.String("porn"),
						Confidence: tea.Float32(95),
					},
				},
			},
		},
	}, nil).Once()

	checker := NewAliyunCheckerWithS3(nil, g2c, mc, "test-bucket")
	result, err := checker.PassImageStreamCheck(context.Background(), types.ScenarioImageBaseLineCheck, strings.NewReader("image-data"))
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsSensitive)
	assert.Contains(t, result.Reason, "porn")
}

func TestPassImageStreamCheck_ModerationError(t *testing.T) {
	g2c := &mockGreen2022{}
	mc := &mockS3Client{}
	mc.On("PutObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything, int64(-1), mock.Anything).
		Return(minio.UploadInfo{}, nil).Once()
	mc.On("RemoveObject", mock.Anything, "test-bucket", mock.AnythingOfType("string"), mock.Anything).
		Return(nil).Once()
	g2c.On("GetRegionId").Return("cn-beijing").Once()
	g2c.On("ImageModeration", mock.Anything).Return(nil, errors.New("moderation api error")).Once()

	checker := NewAliyunCheckerWithS3(nil, g2c, mc, "test-bucket")
	_, err := checker.PassImageStreamCheck(context.Background(), types.ScenarioImageBaseLineCheck, strings.NewReader("image-data"))
	assert.Error(t, err)
}
