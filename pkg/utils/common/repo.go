package common

import (
	"fmt"
	"strings"
)

func WithPrefix(name string, prefix string) string {
	return prefix + name
}

func PortalCloneUrl(url string, prefix string) string {
	return strings.Replace(url, prefix, fmt.Sprintf("%s/", prefix[:len(prefix)-1]), 1)
}

func WithoutPrefix(name string, prefix string) string {
	return strings.Replace(name, prefix, "", 1)
}
