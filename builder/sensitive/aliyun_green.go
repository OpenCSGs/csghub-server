package sensitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	green20220302 "github.com/alibabacloud-go/green-20220302/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/green"
	"opencsg.com/csghub-server/common/config"
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

/*
AliyunGreenChecker implements SensitiveChecker by calling Aliyun green sdk
*/
type AliyunGreenChecker struct {
	//improved client
	green2022 Green2022Client
	//normal client
	green GreenClient
}

func NewAliyunChecker(green GreenClient, green2022 Green2022Client) *AliyunGreenChecker {
	return &AliyunGreenChecker{
		green:     green,
		green2022: green2022,
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
		slog.String("accessKeySecret", accessKeySecret),
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
	}
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
			if result.Label == "ad" || result.Label == "flood" {
				slog.Info("allow ad and flood in text", slog.String("taskId", data.TaskId), slog.String("aliyun_request_id", resp.RequestID))
				continue
			}

			if result.Rate < 0.8 {
				continue
			}

			if result.Suggestion == "block" {
				slog.Info("block content", slog.String("content", truncString(data.Content, 128)), slog.String("taskId", data.TaskId),
					slog.String("aliyun_request_id", resp.RequestID))

				return &CheckResult{IsSensitive: true, Reason: result.Label}, nil
			}
		}
	}

	return &CheckResult{IsSensitive: false}, nil
}

func (c *AliyunGreenChecker) PassTextCheck(ctx context.Context, scenario Scenario, text string) (*CheckResult, error) {
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
		if label == "ad" || label == "flood" {
			continue
		}

		slog.Info("sensitive content detected", slog.String("content", text),
			slog.String("label", label), slog.String("aliyun_request_id", *resp.Body.RequestId))
		return &CheckResult{IsSensitive: true, Reason: *resp.Body.Data.Reason}, nil
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

func (c *AliyunGreenChecker) PassImageCheck(ctx context.Context, scenario Scenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
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
		slog.Error("fail to call aliyun ImageModeration", slog.String("ossBucketName", ossBucketName),
			slog.String("ossObjectName", ossObjectName), slog.Any("error", err))
		return nil, err
	}
	slog.Debug("aliyun ImageModeration return", slog.String("resp", resp.GoString()))

	if *resp.StatusCode != http.StatusOK || *resp.Body.Code != 200 {
		slog.Error("aliyun ImageModeration return code not 200", slog.String("ossBucketName", ossBucketName),
			slog.String("ossObjectName", ossObjectName),
			slog.String("resp", resp.GoString()))
		return nil, errors.New(tea.StringValue(resp.Body.Msg))
	}

	result := resp.Body.Data.Result
	//pass check
	if len(result) == 0 && tea.StringValue(result[0].Label) == "nonLabel" {
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

	slog.Info("sensitive image detected", slog.String("scenario", string(scenario)), slog.String("ossBucketName", ossBucketName),
		slog.String("ossObjectName", ossObjectName), slog.Any("labels", labelMap), slog.String("aliyun_request_id", *resp.Body.RequestId))
	// get all the labels in labelMap and join them with ","
	labels := []string{}
	for label := range labelMap {
		labels = append(labels, label)
	}
	labelStr := strings.Join(labels, ",")
	return &CheckResult{IsSensitive: true, Reason: labelStr}, nil
}
