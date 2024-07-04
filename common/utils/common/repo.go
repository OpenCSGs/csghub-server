package common

import (
	"strings"
)

func WithPrefix(name string, prefix string) string {
	return prefix + name
}

func WithoutPrefix(name string, prefix string) string {
	return strings.Replace(name, prefix, "", 1)
}

func ConvertDotToSlash(d string) string {
	if d == "." {
		return "/"
	} else {
		return d
	}
}
