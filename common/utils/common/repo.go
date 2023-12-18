package common

import (
	"fmt"
	"strings"

	"opencsg.com/starhub-server/common/config"
)

func WithPrefix(name string, prefix string) string {
	return prefix + name
}

func PortalCloneUrl(url string, prefix string, config *config.Config) string {
	url = strings.Replace(url, prefix, fmt.Sprintf("%s/", prefix[:len(prefix)-1]), 1)
	url = strings.Replace(url, config.GitServer.URL, config.Frontend.URL, 1)
	return url
}

func WithoutPrefix(name string, prefix string) string {
	return strings.Replace(name, prefix, "", 1)
}
