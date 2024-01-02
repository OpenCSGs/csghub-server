package sensitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	green20220302 "github.com/alibabacloud-go/green-20220302/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/green"
	"opencsg.com/starhub-server/common/config"
)

/*
AliyunGreenChecker implements SensitiveChecker by calling Aliyun green sdk
*/
type AliyunGreenChecker struct {
	//improved client
	cip *green20220302.Client
	//normal client
	c *green.Client
}

var _ SensitiveChecker = (*AliyunGreenChecker)(nil)

// NewAliyunGreenChecker creates a new AliyunGreenChecker
func NewAliyunGreenChecker(config *config.Config) *AliyunGreenChecker {
	accessKeyID := config.S3.AccessKeyID
	accessKeySecret := config.S3.AccessKeySecret
	region := config.S3.Region
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
		cip,
		c,
	}
}

// passLargeTextCheck splits large text into smaller 10000 bytes chunks and check them in batch
func (c *AliyunGreenChecker) passLargeTextCheck(ctx context.Context, text string) (bool, error) {
	if len(text) > 100*10000 {
		return false, errors.New("text length can't be greater than 100*10000")
	}
	//指定检测对象，JSON数组中的每个元素是一个检测任务结构体。最多支持100个元素，即每次提交100条内容进行检测。如果您的业务需要更大的并发量，请联系客户经理申请并发扩容
	tasks := c.splitTasks(text)
	// scenes：检测场景，唯一取值：antispam。
	content, _ := json.Marshal(
		map[string]interface{}{
			"scenes": [...]string{"antispam"},
			"tasks":  tasks,
		},
	)

	textScanRequest := green.CreateTextScanRequest()
	textScanRequest.SetContent(content)
	textScanResponse, err := c.c.TextScan(textScanRequest)
	if err != nil {
		slog.Error("Failed to call TextScan", slog.Any("error", err))
		return false, err
	}
	data := textScanResponse.GetHttpContentBytes()
	resp := new(TextScanResponse)
	err = json.Unmarshal(data, resp)
	if err != nil {
		return false, fmt.Errorf("error unmarshalling scan response: %w", err)
	}
	for _, data := range resp.Data {
		for _, result := range data.Results {
			if result.Label == "ad" || result.Label == "flood" {
				slog.Info("allow ad and flood in text")
				continue
			}

			if result.Suggestion == "block" {
				return false, nil
			}
		}
	}
	return true, nil
}

func (c *AliyunGreenChecker) PassTextCheck(ctx context.Context, scenario Scenario, text string) (bool, error) {
	if len(text) > 600 {
		return c.passLargeTextCheck(ctx, text)
	}
	tasks := c.splitTasks(text)
	for _, task := range tasks {
		serviceParameters, _ := json.Marshal(task)
		slog.Debug("task", slog.String("task", string(serviceParameters)))
		textModerationRequest := &green20220302.TextModerationRequest{
			Service:           tea.String(string(scenario)),
			ServiceParameters: tea.String(string(serviceParameters)),
		}
		resp, err := c.cip.TextModeration(textModerationRequest)
		if err != nil {
			slog.Error("fail to call aliyun TextModeration", slog.String("content", text), slog.Any("error", err))
			return false, err
		}

		if *resp.StatusCode != http.StatusOK || *resp.Body.Code != 200 {
			slog.Error("aliyun TextModeration return code not 200", slog.String("content", text),
				slog.String("resp", resp.GoString()))
			return false, errors.New(*resp.Body.Message)
		}

		if len(*resp.Body.Data.Labels) > 0 {
			slog.Info("sensitive content detected", slog.String("content", text),
				slog.String("labels", *resp.Body.Data.Labels),
				slog.String("reason", *resp.Body.Data.Reason))
			return false, nil
		}
	}

	return true, nil
}

func (*AliyunGreenChecker) splitTasks(text string) []map[string]string {
	var tasks []map[string]string
	var i int
	for i+1000 < len(text) {
		tasks = append(tasks, map[string]string{"content": text[i : i+1000]})
		i += 1000
	}
	if i <= len(text) {
		tasks = append(tasks, map[string]string{"content": text[i:]})
	}
	return tasks
}

func (c *AliyunGreenChecker) PassImageCheck(ctx context.Context, scenario Scenario, ossBucketName, ossObjectName string) (bool, error) {
	serviceParameters, _ := json.Marshal(
		map[string]interface{}{
			"ossRegionId": tea.StringValue(c.cip.RegionId),
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
	resp, err := c.cip.ImageModeration(imageModerationRequest)
	if err != nil {
		slog.Error("fail to call aliyun ImageModeration", slog.String("ossBucketName", ossBucketName),
			slog.String("ossObjectName", ossObjectName), slog.Any("error", err))
		return false, err
	}
	slog.Debug("aliyun ImageModeration return", slog.String("resp", resp.GoString()))

	if *resp.StatusCode != http.StatusOK || *resp.Body.Code != 200 {
		slog.Error("aliyun ImageModeration return code not 200", slog.String("ossBucketName", ossBucketName),
			slog.String("ossObjectName", ossObjectName),
			slog.String("resp", resp.GoString()))
		return false, errors.New(tea.StringValue(resp.Body.Msg))
	}

	result := resp.Body.Data.Result
	//pass check
	if len(result) == 0 && tea.StringValue(result[0].Label) == "nonLabel" {
		return true, nil
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
		return true, nil
	}

	slog.Info("sensitive image detected", slog.String("scenario", string(scenario)), slog.String("ossBucketName", ossBucketName),
		slog.String("ossObjectName", ossObjectName), slog.Any("labels", labelMap))
	//TODO:return the labels if need in future
	return false, nil
}
