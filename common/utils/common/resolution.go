package common

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// This file parses resolution strings in preset ("720p"), "WIDTHxHEIGHT",
// or bare-integer format. ExtractEventResolutionMaxSide is lenient (used for
// historical/billing data), while ParseResolutionMaxSide additionally
// requires a strictly positive value (used for real-time request/SKU
// validation), so gateway and accounting never interpret the same string
// differently.
var (
	resolutionPFormat    = regexp.MustCompile(`^(\d+)\s*[pP]$`)
	resolutionWxHFormat  = regexp.MustCompile(`^(\d+)\s*[xX×]\s*(\d+)$`)
	resolutionPureFormat = regexp.MustCompile(`^(\d+)$`)
)

func longEdge(width, height int64) int64 {
	if width > height {
		return width
	}
	return height
}

func parseResolutionPFormat(s string) (int64, bool) {
	matches := resolutionPFormat.FindStringSubmatch(s)
	if matches == nil {
		return 0, false
	}
	v, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseResolutionWxHPair(s string) (width, height int64, ok bool) {
	matches := resolutionWxHFormat.FindStringSubmatch(s)
	if matches == nil {
		return 0, 0, false
	}
	w, err1 := strconv.ParseInt(matches[1], 10, 64)
	h, err2 := strconv.ParseInt(matches[2], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return w, h, true
}

func parseResolutionWxHFormat(s string) (int64, bool) {
	w, h, ok := parseResolutionWxHPair(s)
	if !ok {
		return 0, false
	}
	return longEdge(w, h), true
}

func parseResolutionPureFormat(s string) (int64, bool) {
	matches := resolutionPureFormat.FindStringSubmatch(s)
	if matches == nil {
		return 0, false
	}
	v, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func ExtractEventResolutionMaxSide(eventResolution string) (int64, error) {
	s := strings.TrimSpace(eventResolution)
	if len(s) < 1 {
		return 0, nil
	}

	if v, ok := parseResolutionPFormat(s); ok {
		return v, nil
	}
	if v, ok := parseResolutionWxHFormat(s); ok {
		return v, nil
	}
	if v, ok := parseResolutionPureFormat(s); ok {
		return v, nil
	}

	return 0, fmt.Errorf("unsupported resolution format: %s, must be 1080P or 720 or 1920x1080", s)
}

// ParseResolutionMaxSide is like ExtractEventResolutionMaxSide but rejects
// empty input and non-positive results.
func ParseResolutionMaxSide(size string) (int64, bool) {
	v, err := ExtractEventResolutionMaxSide(size)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}
