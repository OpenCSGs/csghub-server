package sensitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	green20220302 "github.com/alibabacloud-go/green-20220302/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/green"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// copy from common/utils/common to avoid cycle import
func truncString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}

	s1 := []byte(s[:limit])
	s1[limit-1] = '.'
	s1[limit-2] = '.'
	s1[limit-3] = '.'
	return string(s1)
}

type GreenClient interface {
	TextScan(request *green.TextScanRequest) (response *TextScanResponse, err error)
}

type greenClientImpl struct {
	green *green.Client
}

func (c *greenClientImpl) TextScan(request *green.TextScanRequest) (response *TextScanResponse, err error) {
	textScanResponse, err := c.green.TextScan(request)
	if err != nil {
		slog.Error("Failed to call TextScan", slog.Any("error", err))
		return nil, err
	}
	data := textScanResponse.GetHttpContentBytes()
	resp := new(TextScanResponse)
	err = json.Unmarshal(data, resp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling scan response: %w", err)
	}
	return resp, nil
}

type Green2022Client interface {
	GetRegionId() string
	TextModeration(request *green20220302.TextModerationRequest) (_result *green20220302.TextModerationResponse, _err error)
	ImageModeration(request *green20220302.ImageModerationRequest) (_result *green20220302.ImageModerationResponse, _err error)
	TextModerationPlusWithOptions(request *green20220302.TextModerationPlusRequest, options *util.RuntimeOptions) (_result *green20220302.TextModerationPlusResponse, _err error)
	VoiceModeration(request *green20220302.VoiceModerationRequest) (_result *green20220302.VoiceModerationResponse, _err error)
	VoiceModerationResult(request *green20220302.VoiceModerationResultRequest) (_result *green20220302.VoiceModerationResultResponse, _err error)
	VideoModeration(request *green20220302.VideoModerationRequest) (_result *green20220302.VideoModerationResponse, _err error)
	VideoModerationResult(request *green20220302.VideoModerationResultRequest) (_result *green20220302.VideoModerationResultResponse, _err error)
}

type green2022ClientImpl struct {
	green *green20220302.Client
}

func (c *green2022ClientImpl) GetRegionId() string {
	return tea.StringValue(c.green.RegionId)
}

func (c *green2022ClientImpl) TextModeration(request *green20220302.TextModerationRequest) (_result *green20220302.TextModerationResponse, _err error) {
	return c.green.TextModeration(request)
}

func (c *green2022ClientImpl) ImageModeration(request *green20220302.ImageModerationRequest) (_result *green20220302.ImageModerationResponse, _err error) {
	return c.green.ImageModeration(request)
}

func (c *green2022ClientImpl) TextModerationPlusWithOptions(request *green20220302.TextModerationPlusRequest, options *util.RuntimeOptions) (_result *green20220302.TextModerationPlusResponse, _err error) {
	return c.green.TextModerationPlusWithOptions(request, options)
}

func (c *green2022ClientImpl) VoiceModeration(request *green20220302.VoiceModerationRequest) (_result *green20220302.VoiceModerationResponse, _err error) {
	return c.green.VoiceModeration(request)
}

func (c *green2022ClientImpl) VoiceModerationResult(request *green20220302.VoiceModerationResultRequest) (_result *green20220302.VoiceModerationResultResponse, _err error) {
	return c.green.VoiceModerationResult(request)
}

func (c *green2022ClientImpl) VideoModeration(request *green20220302.VideoModerationRequest) (_result *green20220302.VideoModerationResponse, _err error) {
	return c.green.VideoModeration(request)
}

func (c *green2022ClientImpl) VideoModerationResult(request *green20220302.VideoModerationResultRequest) (_result *green20220302.VideoModerationResultResponse, _err error) {
	return c.green.VideoModerationResult(request)
}

/*
AliyunGreenChecker implements SensitiveChecker by calling Aliyun green sdk
*/
type AliyunGreenChecker struct {
	//improved client
	green2022 Green2022Client
	//normal client
	green GreenClient
	//s3Client is a minio client pointing at Aliyun OSS, used for temporary
	//image uploads when checking images from private repos. nil if OSS is
	//not configured.
	s3Client s3Client
	//s3BucketName is the name of the Aliyun OSS bucket used for temp uploads.
	s3BucketName string
}

// s3Client is the subset of minio.Client methods used by AliyunGreenChecker
// for temporary image uploads. It allows mocking in tests.
type s3Client interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

func NewAliyunChecker(green GreenClient, green2022 Green2022Client) *AliyunGreenChecker {
	return &AliyunGreenChecker{
		green:     green,
		green2022: green2022,
	}
}

// NewAliyunCheckerWithS3 creates an AliyunGreenChecker with an OSS client for
// image stream checks. Used in tests; production code uses initS3Client.
func NewAliyunCheckerWithS3(green GreenClient, green2022 Green2022Client, s3Cli s3Client, bucketName string) *AliyunGreenChecker {
	return &AliyunGreenChecker{
		green:        green,
		green2022:    green2022,
		s3Client:     s3Cli,
		s3BucketName: bucketName,
	}
}

var _ SensitiveChecker = (*AliyunGreenChecker)(nil)

const smallTextSize = 500
const LargeTextSize = 9000

// NewAliyunGreenCheckerFromConfig creates a new AliyunGreenChecker
func NewAliyunGreenCheckerFromConfig(config *config.Config) *AliyunGreenChecker {
	accessKeyID := config.SensitiveCheck.AccessKeyID
	accessKeySecret := config.SensitiveCheck.AccessKeySecret
	region := config.SensitiveCheck.Region
	slog.Debug("Aliyun client init", slog.String("accessKeyID", accessKeyID),
		slog.String("region", region))

	aliyunConfig := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		RegionId:        tea.String(region),
		ConnectTimeout:  tea.Int(1000),
		ReadTimeout:     tea.Int(2000),
	}
	cip, err := green20220302.NewClient(aliyunConfig)
	if err != nil {
		log.Fatalf("NewAliyunGreenChecker client enhanced failed: %v", err)
	}

	c, err := green.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		log.Fatalf("NewAliyunGreenChecker client failed: %v", err)
	}

	return &AliyunGreenChecker{
		&green2022ClientImpl{green: cip},
		&greenClientImpl{green: c},
		nil,
		"",
	}
}

// initS3Client initializes a minio client pointing at Aliyun OSS using the
// SensitiveCheck credentials (same AK/SK as Aliyun Green). This ensures the
// temporary uploaded objects are in the same Aliyun OSS account, so Aliyun
// Green can read them directly via PassImageCheck (ossBucketName + ossObjectName).
//
// Called separately to allow the checker to function without OSS (URL-based
// checks still work). If OSSBucket is empty, s3Client stays nil.
func (c *AliyunGreenChecker) initS3Client(config *config.Config) {
	bucketName := config.SensitiveCheck.OSSBucket
	if bucketName == "" {
		slog.Warn("sensitive check OSS bucket not configured, image stream check will be unavailable for private repos")
		return
	}

	endpoint := config.SensitiveCheck.Endpoint
	accessKeyID := config.SensitiveCheck.AccessKeyID
	accessKeySecret := config.SensitiveCheck.AccessKeySecret
	region := config.SensitiveCheck.Region
	enableSSL := config.SensitiveCheck.EnableSSL

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKeyID, accessKeySecret, ""),
		Secure:       enableSSL,
		BucketLookup: minio.BucketLookupAuto,
		Region:       region,
	})
	if err != nil {
		slog.Error("failed to create OSS client for sensitive check", slog.Any("error", err))
		return
	}

	c.s3Client = client
	c.s3BucketName = bucketName
	slog.Info("sensitive check OSS client initialized", slog.String("bucket", bucketName), slog.String("endpoint", endpoint))
}

// passLargeTextCheck splits large text into smaller `largeTextSize` bytes chunks and check them in batch
func (c *AliyunGreenChecker) PassLargeTextCheck(ctx context.Context, text string) (*CheckResult, error) {
	if len(text) > 100*LargeTextSize {
		return nil, fmt.Errorf("text length can't be greater than 100*%d", LargeTextSize)
	}
	tasks := c.SplitTasks(text)
	content, _ := json.Marshal(
		map[string]interface{}{
			"scenes": [...]string{"antispam"},
			"tasks":  tasks,
		},
	)

	textScanRequest := green.CreateTextScanRequest()
	textScanRequest.SetContent(content)
	resp, err := c.green.TextScan(textScanRequest)
	if err != nil {
		slog.Error("Failed to call TextScan", slog.Any("error", err))
		return nil, err
	}
	for _, data := range resp.Data {
		for _, result := range data.Results {
			if result.Label != "politics" && result.Label != "political_content" {
				continue
			}

			if result.Rate < 0.8 {
				continue
			}

			if result.Suggestion == "block" {
				slog.Info("block content", slog.String("label", result.Label), slog.String("content", truncString(data.Content, 128)),
					slog.String("aliyun_taskId", data.TaskId),
					slog.String("aliyun_requestId", resp.RequestID))

				return &CheckResult{IsSensitive: true, Reason: fmt.Sprintf("label:%s,taskId:%s,requestId:%s", result.Label, data.TaskId, resp.RequestID)}, nil
			}
		}
	}

	return &CheckResult{IsSensitive: false}, nil
}

func (c *AliyunGreenChecker) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error) {
	if len(text) > smallTextSize {
		slog.Info("switch to large text check", slog.String("scenario", string(scenario)), slog.Int("size", len(text)))
		return c.PassLargeTextCheck(ctx, text)
	}
	task := map[string]string{"content": text}
	serviceParameters, _ := json.Marshal(task)
	textModerationRequest := &green20220302.TextModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	resp, err := c.green2022.TextModeration(textModerationRequest)
	if err != nil {
		slog.Error("fail to call aliyun TextModeration", slog.String("content", text), slog.Any("error", err))
		return nil, err
	}

	if *resp.StatusCode != http.StatusOK || *resp.Body.Code != 200 {
		slog.Error("aliyun TextModeration return code not 200", slog.String("content", text),
			slog.String("resp", resp.GoString()))
		return nil, errors.New(*resp.Body.Message)
	}

	if len(*resp.Body.Data.Labels) == 0 {
		return &CheckResult{IsSensitive: false}, nil
	}

	labelStr := *resp.Body.Data.Labels
	labels := strings.Split(labelStr, ",")
	for _, label := range labels {
		if label != "politics" && label != "political_content" {
			continue
		}

		slog.Info("sensitive content detected", slog.String("content", text),
			slog.String("label", label), slog.String("reason", *resp.Body.Data.Reason),
			slog.String("aliyun_request_id", *resp.Body.RequestId))
		return &CheckResult{IsSensitive: true, Reason: fmt.Sprintf("label:%s,reason:%s,requestId:%s", label, *resp.Body.Data.Reason, *resp.Body.RequestId)}, nil
	}

	return &CheckResult{IsSensitive: false}, nil
}

func (*AliyunGreenChecker) SplitTasks(text string) []map[string]string {
	var tasks []map[string]string
	var i int
	for i+LargeTextSize < len(text) {
		tasks = append(tasks, map[string]string{"content": text[i : i+LargeTextSize]})
		i += LargeTextSize
	}
	if i <= len(text) {
		tasks = append(tasks, map[string]string{"content": text[i:]})
	}
	return tasks
}

func (c *AliyunGreenChecker) PassLLMCheck(ctx context.Context, req *types.LLMCheckRequest) (*CheckResult, error) {
	// Build parameter map
	paramMap := map[string]interface{}{
		"content": req.Text,
	}
	// Add different ID field based on idType
	if req.SessionId != "" && req.AccountId != "" {
		return nil, fmt.Errorf("fail to call aliyun TextModerationPlusWithOptions, can't set sessionId and accountId both")
	}
	if req.SessionId != "" {
		paramMap["sessionId"] = req.SessionId
	}
	if req.AccountId != "" {
		if req.Text == "" {
			return &CheckResult{IsSensitive: false}, nil
		}
		paramMap["accountId"] = req.AccountId
	}

	serviceParameters, _ := json.Marshal(paramMap)

	request := &green20220302.TextModerationPlusRequest{
		Service:           tea.String(string(req.Scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}

	options := &util.RuntimeOptions{
		ReadTimeout:    tea.Int(500),
		ConnectTimeout: tea.Int(500),
	}
	resp, err := c.green2022.TextModerationPlusWithOptions(request, options)
	if err != nil {
		slog.Error("fail to call aliyun TextModerationPlusWithOptions", slog.String("content", req.Text), slog.Any("error", err))
		return nil, fmt.Errorf("fail to call aliyun TextModerationPlusWithOptions: %w", err)
	}

	if *resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aliyun TextModerationPlusWithOptions response not success, http status code: %d", *resp.StatusCode)
	}

	if *resp.Body.Code != http.StatusOK {
		return nil, fmt.Errorf("aliyun TextModerationPlusWithOptions text moderation not success, error message: %s", *resp.Body.Message)
	}

	results := resp.Body.Data.Result
	if *resp.Body.Data.RiskLevel == "low" ||
		*resp.Body.Data.RiskLevel == "medium" ||
		*resp.Body.Data.RiskLevel == "none" {
		return &CheckResult{IsSensitive: false}, nil
	}
	// refer to label https://help.aliyun.com/document_detail/2671445.html#section-3t8-ane-efg
	for _, result := range results {
		if !strings.Contains(*result.Label, "political") {
			continue
		}
		slog.Info("sensitive content detected", slog.String("content", req.Text), slog.String("reason", *result.RiskWords),
			slog.String("label", *result.Label), slog.String("aliyun_request_id", *resp.Body.RequestId))
		return &CheckResult{IsSensitive: true, Reason: fmt.Sprintf("label:%s,reason:%s,requestId:%s", *result.Label, *result.RiskWords, *resp.Body.RequestId)}, nil
	}
	return &CheckResult{IsSensitive: false}, nil
}

func (c *AliyunGreenChecker) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error) {
	serviceParameters, _ := json.Marshal(
		map[string]interface{}{
			"imageURL": imageURL,
			"dataId":   uuid.New(),
		},
	)
	imageModerationRequest := &green20220302.ImageModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	resp, err := c.green2022.ImageModeration(imageModerationRequest)
	if err != nil {
		return nil, err
	}
	if *resp.StatusCode != http.StatusOK || *resp.Body.Code != 200 {
		return nil, errors.New(tea.StringValue(resp.Body.Msg))
	}
	result := resp.Body.Data.Result
	//pass check
	if len(result) == 0 || (len(result) == 1 && tea.StringValue(result[0].Label) == "nonLabel") {
		return &CheckResult{IsSensitive: false}, nil
	}
	//sensitive check
	for _, r := range result {
		if tea.StringValue(r.Label) != "nonLabel" {
			slog.InfoContext(ctx, "sensitive image detected", slog.String("imageURL", imageURL), slog.String("label", tea.StringValue(r.Label)),
				slog.Any("confidence", tea.Float32Value(r.Confidence)), slog.String("aliyun_request_id", *resp.Body.RequestId))
			return &CheckResult{IsSensitive: true, Reason: fmt.Sprintf("label:%s,confidence:%f,requestId:%s", tea.StringValue(r.Label), tea.Float32Value(r.Confidence), *resp.Body.RequestId)}, nil
		}
	}
	return &CheckResult{IsSensitive: false}, nil
}

func (c *AliyunGreenChecker) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
	serviceParameters, _ := json.Marshal(
		map[string]interface{}{
			"ossRegionId": c.green2022.GetRegionId(),
			//for example: my-image-bucket
			"ossBucketName": ossBucketName,
			//for example: image/001.jpg
			"ossObjectName": ossObjectName,
		},
	)
	imageModerationRequest := &green20220302.ImageModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	resp, err := c.green2022.ImageModeration(imageModerationRequest)
	if err != nil {
		return nil, err
	}
	slog.DebugContext(ctx, "aliyun ImageModeration return", slog.String("resp", resp.GoString()))

	if *resp.StatusCode != http.StatusOK || *resp.Body.Code != 200 {
		return nil, errors.New(tea.StringValue(resp.Body.Msg))
	}

	result := resp.Body.Data.Result
	//pass check
	if len(result) == 0 || (len(result) == 1 && tea.StringValue(result[0].Label) == "nonLabel") {
		return &CheckResult{IsSensitive: false}, nil
	}

	labelMap := make(map[string]float32)
	for _, r := range result {
		label, confidence := tea.StringValue(r.Label), tea.Float32Value(r.Confidence)
		if confidence > 80 {
			labelMap[label] = confidence
		}
	}
	//pass check
	if len(labelMap) == 0 {
		return &CheckResult{IsSensitive: false}, nil
	}

	slog.InfoContext(ctx, "sensitive image detected", slog.String("scenario", string(scenario)), slog.String("ossBucketName", ossBucketName),
		slog.String("ossObjectName", ossObjectName), slog.Any("labels", labelMap), slog.String("aliyun_request_id", *resp.Body.RequestId))
	// get all the labels in labelMap and join them with ","
	labels := []string{}
	for label := range labelMap {
		labels = append(labels, label)
	}
	labelStr := strings.Join(labels, ",")
	return &CheckResult{IsSensitive: true, Reason: labelStr}, nil
}

// PassImageStreamCheck uploads an image stream to a temporary Aliyun OSS object,
// then calls PassImageCheck for moderation. Aliyun Green reads the object
// directly from OSS using the bucket name and object key (no presigned URL needed).
// The temporary object is always deleted after the check completes.
func (c *AliyunGreenChecker) PassImageStreamCheck(ctx context.Context, scenario types.SensitiveScenario, reader io.Reader) (*CheckResult, error) {
	if c.s3Client == nil {
		return nil, errors.New("OSS client is not initialized, image stream check is unavailable")
	}

	// Generate a unique object key for the temporary upload
	objectKey := fmt.Sprintf("moderation/temp/%s", uuid.New().String())

	// Upload the image stream to OSS. Set Expires to 30 minutes from now as a
	// safety net — even if the defer delete fails, the object auto-expires.
	_, err := c.s3Client.PutObject(ctx, c.s3BucketName, objectKey, reader, -1, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
		Expires:     time.Now().Add(30 * time.Minute),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload image stream to OSS: %w", err)
	}

	// Always clean up the temporary object
	defer func() {
		if delErr := c.s3Client.RemoveObject(ctx, c.s3BucketName, objectKey, minio.RemoveObjectOptions{}); delErr != nil {
			slog.ErrorContext(ctx, "failed to delete temporary image from OSS", slog.String("objectKey", objectKey), slog.Any("error", delErr))
		}
	}()

	// Aliyun Green reads the object directly from OSS via bucket name + object key
	return c.PassImageCheck(ctx, scenario, c.s3BucketName, objectKey)
}

// Aliyun voice/video moderation service names documented for asynchronous
// media-file moderation.
const (
	aliyunVoiceService = "audio_media_detection"
	aliyunVideoService = "videoDetection"
)

// SubmitMediaModeration submits one audio or video URL to Aliyun for
// asynchronous moderation and returns the provider task handle. The same
// dataId/seed passed by the caller are echoed back so the caller can verify
// the submission identity.
func (c *AliyunGreenChecker) SubmitMediaModeration(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error) {
	if req.URL == "" || req.DataID == "" {
		return nil, errors.New("media moderation submit requires url and data_id")
	}
	serviceParameters, err := json.Marshal(map[string]string{"url": req.URL, "dataId": req.DataID})
	if err != nil {
		return nil, fmt.Errorf("marshal media moderation service parameters: %w", err)
	}
	switch req.Type {
	case types.MediaTypeAudio:
		resp, err := c.green2022.VoiceModeration(&green20220302.VoiceModerationRequest{
			Service:           tea.String(aliyunVoiceService),
			ServiceParameters: tea.String(string(serviceParameters)),
		})
		if err != nil {
			return nil, fmt.Errorf("aliyun voice moderation submit: %w", err)
		}
		return mediaModerationSubmissionFromResp(resp, req)
	case types.MediaTypeVideo:
		resp, err := c.green2022.VideoModeration(&green20220302.VideoModerationRequest{
			Service:           tea.String(aliyunVideoService),
			ServiceParameters: tea.String(string(serviceParameters)),
		})
		if err != nil {
			return nil, fmt.Errorf("aliyun video moderation submit: %w", err)
		}
		return mediaModerationSubmissionFromVideoResp(resp, req)
	default:
		return nil, fmt.Errorf("unsupported media type %q for moderation submit", req.Type)
	}
}

// QueryMediaModerationResult polls the Aliyun task handle and maps the
// provider risk level to a terminal status (pass/reject) or "pending" while
// the task is still being processed. Unknown risk levels map to "error".
func (c *AliyunGreenChecker) QueryMediaModerationResult(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationResult, error) {
	if req.TaskID == "" {
		return nil, errors.New("media moderation query requires task_id")
	}
	serviceParameters, err := json.Marshal(map[string]string{"taskId": req.TaskID})
	if err != nil {
		return nil, fmt.Errorf("marshal media moderation query parameters: %w", err)
	}
	switch req.Type {
	case types.MediaTypeAudio:
		resp, err := c.green2022.VoiceModerationResult(&green20220302.VoiceModerationResultRequest{
			Service:           tea.String(aliyunVoiceService),
			ServiceParameters: tea.String(string(serviceParameters)),
		})
		if err != nil {
			return nil, fmt.Errorf("aliyun voice moderation result: %w", err)
		}
		return mediaModerationResultFromVoice(resp, req.DataID), nil
	case types.MediaTypeVideo:
		resp, err := c.green2022.VideoModerationResult(&green20220302.VideoModerationResultRequest{
			Service:           tea.String(aliyunVideoService),
			ServiceParameters: tea.String(string(serviceParameters)),
		})
		if err != nil {
			return nil, fmt.Errorf("aliyun video moderation result: %w", err)
		}
		return mediaModerationResultFromVideo(resp, req.DataID), nil
	default:
		return nil, fmt.Errorf("unsupported media type %q for moderation query", req.Type)
	}
}

func mediaModerationSubmissionFromResp(resp *green20220302.VoiceModerationResponse, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error) {
	if resp == nil || resp.Body == nil || resp.Body.Code == nil || *resp.Body.Code != 200 || resp.Body.Data == nil {
		return nil, fmt.Errorf("aliyun voice moderation submit failed: %s", mediaModerationAliyunMessage(resp))
	}
	return &types.MediaModerationSubmission{
		DataID: tea.StringValue(resp.Body.Data.DataId),
		Seed:   req.Seed,
		TaskID: tea.StringValue(resp.Body.Data.TaskId),
	}, nil
}

func mediaModerationSubmissionFromVideoResp(resp *green20220302.VideoModerationResponse, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error) {
	if resp == nil || resp.Body == nil || resp.Body.Code == nil || *resp.Body.Code != 200 || resp.Body.Data == nil {
		return nil, fmt.Errorf("aliyun video moderation submit failed: %s", mediaModerationAliyunMessage(resp))
	}
	return &types.MediaModerationSubmission{
		DataID: tea.StringValue(resp.Body.Data.DataId),
		Seed:   req.Seed,
		TaskID: tea.StringValue(resp.Body.Data.TaskId),
	}, nil
}

// mediaModerationResultFromVoice maps the Aliyun voice result to a terminal
// status. A non-200 code or empty risk level means the task is still pending.
func mediaModerationResultFromVoice(resp *green20220302.VoiceModerationResultResponse, dataID string) *types.MediaModerationResult {
	if resp == nil || resp.Body == nil || resp.Body.Code == nil || *resp.Body.Code != 200 || resp.Body.Data == nil {
		return &types.MediaModerationResult{DataID: dataID, Status: "pending"}
	}
	riskLevel := strings.ToLower(strings.TrimSpace(tea.StringValue(resp.Body.Data.RiskLevel)))
	status, reason := mediaModerationStatusFromRisk(riskLevel)
	// Any audio slice with a higher risk level overrides the top-level value.
	for _, slice := range resp.Body.Data.SliceDetails {
		if slice == nil {
			continue
		}
		sliceStatus, sliceReason := mediaModerationStatusFromRisk(strings.ToLower(strings.TrimSpace(tea.StringValue(slice.RiskLevel))))
		if sliceStatus == "reject" {
			return &types.MediaModerationResult{DataID: dataID, Status: "reject", Reason: sliceReason}
		}
		if sliceStatus == "pending" && status != "reject" {
			status, reason = "pending", ""
		}
	}
	return &types.MediaModerationResult{DataID: dataID, Status: status, Reason: reason}
}

// mediaModerationResultFromVideo maps the Aliyun video result, inspecting the
// top-level risk level plus the frame and audio sub-results. Any level that
// maps to reject short-circuits to reject; otherwise the most restrictive
// non-pass level wins (pending beats pass so the workflow keeps polling).
func mediaModerationResultFromVideo(resp *green20220302.VideoModerationResultResponse, dataID string) *types.MediaModerationResult {
	if resp == nil || resp.Body == nil || resp.Body.Code == nil || *resp.Body.Code != 200 || resp.Body.Data == nil {
		return &types.MediaModerationResult{DataID: dataID, Status: "pending"}
	}
	data := resp.Body.Data
	levels := []string{strings.ToLower(strings.TrimSpace(tea.StringValue(data.RiskLevel)))}
	if data.FrameResult != nil {
		levels = append(levels, strings.ToLower(strings.TrimSpace(tea.StringValue(data.FrameResult.RiskLevel))))
	}
	if data.AudioResult != nil {
		levels = append(levels, strings.ToLower(strings.TrimSpace(tea.StringValue(data.AudioResult.RiskLevel))))
	}
	status, reason := "pass", ""
	for _, level := range levels {
		s, r := mediaModerationStatusFromRisk(level)
		if s == "reject" {
			return &types.MediaModerationResult{DataID: dataID, Status: "reject", Reason: r}
		}
		if s == "pending" {
			status, reason = "pending", ""
		}
	}
	return &types.MediaModerationResult{DataID: dataID, Status: status, Reason: reason}
}

// mediaModerationStatusFromRisk maps an Aliyun risk level to a moderation
// status. none/low → pass; medium/high → reject; empty/unknown → pending
// (keep polling). An empty or unrecognized risk level is NOT treated as a
// terminal error, because Aliyun may omit the top-level RiskLevel while the
// task is still finalizing or only populate sub-results; marking it error
// would wrongly delete the comment.
func mediaModerationStatusFromRisk(level string) (string, string) {
	switch level {
	case "none", "low":
		return "pass", ""
	case "medium", "high":
		return "reject", "sensitive media content"
	default:
		// empty or unknown risk level: keep polling, do not persist a terminal result
		return "pending", ""
	}
}

func mediaModerationAliyunMessage(resp any) string {
	// Best-effort message extraction; the response types differ between voice
	// and video but both expose Body.Message via the tea pointer.
	return "aliyun moderation returned a non-200 code"
}
