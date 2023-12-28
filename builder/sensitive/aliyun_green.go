package sensitive

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	green20220302 "github.com/alibabacloud-go/green-20220302/client"
	"github.com/alibabacloud-go/tea/tea"
	"opencsg.com/starhub-server/common/config"
)

/*
AliyunGreenChecker implements SensitiveChecker by calling Aliyun green sdk
*/
type AliyunGreenChecker struct {
	*green20220302.Client
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
	c, err := green20220302.NewClient(aliyunConfig)
	if err != nil {
		log.Fatalf("NewAliyunGreenChecker failed: %v", err)
	}

	return &AliyunGreenChecker{
		c,
	}
}

func (c *AliyunGreenChecker) PassTextCheck(ctx context.Context, scenario Scenario, text string) (bool, error) {
	serviceParameters, _ := json.Marshal(
		map[string]interface{}{
			"content": text,
		},
	)

	textModerationRequest := &green20220302.TextModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	resp, err := c.TextModeration(textModerationRequest)
	if err != nil {
		slog.Error("fail to call aliyun TextModeration", slog.String("content", text), slog.Any("error", err))
		return false, err
	}
	slog.Debug("aliyun TextModeration return", slog.String("resp", resp.GoString()))

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

	return true, nil
}

func (c *AliyunGreenChecker) PassImageCheck(ctx context.Context, scenario Scenario, ossBucketName, ossObjectName string) (bool, error) {
	serviceParameters, _ := json.Marshal(
		map[string]interface{}{
			"ossRegionId": tea.StringValue(c.RegionId),
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
	resp, err := c.ImageModeration(imageModerationRequest)
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
