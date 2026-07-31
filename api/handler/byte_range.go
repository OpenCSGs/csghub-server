package handler

import (
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
)

// errNoOverlap indicates that none of the requested ranges overlap the representation.
var errNoOverlap = errors.New("invalid range: failed to overlap")

// httpRange specifies a byte range to be sent to the client.
type httpRange struct {
	start  int64
	length int64
}

// contentRange formats the range for an HTTP Content-Range response header.
func (r httpRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size)
}

// parseRange parses a Range header as defined by RFC 9110.
// It is adapted from net/http.parseRange in the Go standard library (BSD-3-Clause).
// Copyright 2009 The Go Authors.
func parseRange(header string, size int64) ([]httpRange, error) {
	if header == "" {
		return nil, nil
	}
	const byteRangePrefix = "bytes="
	if !strings.HasPrefix(header, byteRangePrefix) {
		return nil, errors.New("invalid range")
	}

	var ranges []httpRange
	noOverlap := false
	for _, rangeSpec := range strings.Split(header[len(byteRangePrefix):], ",") {
		rangeSpec = textproto.TrimString(rangeSpec)
		if rangeSpec == "" {
			continue
		}

		startText, endText, found := strings.Cut(rangeSpec, "-")
		if !found {
			return nil, errors.New("invalid range")
		}
		startText = textproto.TrimString(startText)
		endText = textproto.TrimString(endText)

		var requestedRange httpRange
		if startText == "" {
			if endText == "" || endText[0] == '-' {
				return nil, errors.New("invalid range")
			}
			suffixLength, err := strconv.ParseInt(endText, 10, 64)
			if suffixLength < 0 || err != nil {
				return nil, errors.New("invalid range")
			}
			if suffixLength > size {
				suffixLength = size
			}
			requestedRange.start = size - suffixLength
			requestedRange.length = size - requestedRange.start
		} else {
			start, err := strconv.ParseInt(startText, 10, 64)
			if err != nil || start < 0 {
				return nil, errors.New("invalid range")
			}
			if start >= size {
				noOverlap = true
				continue
			}
			requestedRange.start = start
			if endText == "" {
				requestedRange.length = size - requestedRange.start
			} else {
				end, err := strconv.ParseInt(endText, 10, 64)
				if err != nil || requestedRange.start > end {
					return nil, errors.New("invalid range")
				}
				if end >= size {
					end = size - 1
				}
				requestedRange.length = end - requestedRange.start + 1
			}
		}
		ranges = append(ranges, requestedRange)
	}
	if noOverlap && len(ranges) == 0 {
		return nil, errNoOverlap
	}
	return ranges, nil
}
