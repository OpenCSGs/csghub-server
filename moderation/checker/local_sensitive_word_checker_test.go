package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/config"
)

func TestContainsSensitiveWord(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.SensitiveCheck.AccessKeyID = "accessKeyID"
	cfg.SensitiveCheck.AccessKeySecret = "accessKeySecret"
	cfg.SensitiveCheck.Endpoint = "endpoint"
	cfg.SensitiveCheck.Region = "region"
	Init(cfg)

	d := NewDFA()
	d.BuildDFA(getSensitiveWordList(`5pWP5oSf6K+NLHNlbnNpdGl2ZXdvcmQ=`))
	assert.True(t, d.ContainsSensitiveWord("敏感词"))
	assert.True(t, d.ContainsSensitiveWord("敏*感词"))
	assert.True(t, d.ContainsSensitiveWord("敏 感词"))
	assert.True(t, d.ContainsSensitiveWord("敏\u3000感词"))
	assert.True(t, d.ContainsSensitiveWord("敏感词123"))
	assert.False(t, d.ContainsSensitiveWord("敏感1234词"))

	assert.True(t, d.ContainsSensitiveWord("sensitive word"))
	assert.True(t, d.ContainsSensitiveWord("sensitive word123"))
	assert.False(t, d.ContainsSensitiveWord("sensitive ord"))
}
