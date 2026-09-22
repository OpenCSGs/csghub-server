package aliyun

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
)

func TestPluginAliyunCheckerSensitiveCheckEnv(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.AccessKeyID = "access-key-id"
	cfg.SensitiveCheck.AccessKeySecret = "access-key-secret"
	cfg.SensitiveCheck.Region = "cn-beijing"
	cfg.SensitiveCheck.Endpoint = "oss-cn-beijing.aliyuncs.com"
	cfg.SensitiveCheck.OSSBucket = "sensitive-check"
	cfg.SensitiveCheck.EnableSSL = true

	checker := &pluginAliyunChecker{config: cfg}

	require.Equal(t, []string{
		"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID=access-key-id",
		"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET=access-key-secret",
		"STARHUB_SERVER_SENSITIVE_CHECK_REGION=cn-beijing",
		"STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT=oss-cn-beijing.aliyuncs.com",
		"STARHUB_SERVER_SENSITIVE_CHECK_OSS_BUCKET=sensitive-check",
		"STARHUB_SERVER_SENSITIVE_CHECK_ENABLE_SSL=true",
	}, checker.sensitiveCheckEnv())
}
