package i18n

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitLocalizersFromEmbedFile_ImportErrors(t *testing.T) {
	InitLocalizersFromEmbedFile()

	cases := []struct {
		lang string
		code string
		want string
	}{
		{"en-US", "IMPORT-ERR-0", "The provided address is not a valid GitLab instance"},
		{"en-US", "IMPORT-ERR-1", "The GitLab instance is unreachable"},
		{"en-US", "IMPORT-ERR-2", "Unauthorized to access the GitLab instance, please check the access token"},
		{"zh-CN", "IMPORT-ERR-0", "提供的地址不是有效的 GitLab 实例"},
		{"zh-CN", "IMPORT-ERR-1", "无法访问 GitLab 实例"},
		{"zh-CN", "IMPORT-ERR-2", "无权访问 GitLab 实例，请检查访问令牌"},
		{"zh-HK", "IMPORT-ERR-0", "提供的地址不是有效的 GitLab 實例"},
		{"zh-HK", "IMPORT-ERR-1", "無法訪問 GitLab 實例"},
		{"zh-HK", "IMPORT-ERR-2", "無權訪問 GitLab 實例，請檢查訪問令牌"},
	}
	for _, c := range cases {
		t.Run(c.lang+"/"+c.code, func(t *testing.T) {
			got, ok := TranslateText(c.lang, "error."+c.code, c.code)
			require.True(t, ok, "translation missing for %s", c.code)
			require.Equal(t, c.want, got)
		})
	}
}
