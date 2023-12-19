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
	accessKeyID := config.Aliyun.AccessKeyID
	accessKeySecret := config.Aliyun.AccessKeySecret
	region := config.Aliyun.Region
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
		return false, err
	}
	slog.Debug("aliyun TextModeration return", slog.String("resp", resp.GoString()))

	if *resp.StatusCode != http.StatusOK || *resp.Body.Code != 200 {
		slog.Error("fail to call aliyun TextModeration", slog.String("content", text),
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
