package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseRange verifies the behavior adapted from net/http.parseRange.
func TestParseRange(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		size          int64
		want          []httpRange
		wantErr       bool
		wantNoOverlap bool
	}{
		{name: "no range", size: 100},
		{name: "closed range", header: "bytes=10-19", size: 100, want: []httpRange{{start: 10, length: 10}}},
		{name: "open ended range", header: "bytes=90-", size: 100, want: []httpRange{{start: 90, length: 10}}},
		{name: "suffix range", header: "bytes=-10", size: 100, want: []httpRange{{start: 90, length: 10}}},
		{name: "suffix larger than file", header: "bytes=-200", size: 100, want: []httpRange{{start: 0, length: 100}}},
		{name: "end is clamped", header: "bytes=95-200", size: 100, want: []httpRange{{start: 95, length: 5}}},
		{name: "multiple ranges", header: "bytes=0-1,4-5", size: 100, want: []httpRange{{start: 0, length: 2}, {start: 4, length: 2}}},
		{name: "partially satisfiable ranges", header: "bytes=100-200,2-3", size: 100, want: []httpRange{{start: 2, length: 2}}},
		{name: "missing value", header: "bytes=", size: 100},
		{name: "zero suffix", header: "bytes=-0", size: 100, want: []httpRange{{start: 100, length: 0}}},
		{name: "unsupported unit", header: "items=0-1", size: 100, wantErr: true},
		{name: "case sensitive unit", header: "BYTES=0-0", size: 100, wantErr: true},
		{name: "missing separator", header: "bytes", size: 100, wantErr: true},
		{name: "invalid number", header: "bytes=a-10", size: 100, wantErr: true},
		{name: "negative start", header: "bytes=-1-10", size: 100, wantErr: true},
		{name: "start after end", header: "bytes=20-10", size: 100, wantErr: true},
		{name: "integer overflow", header: "bytes=9223372036854775808-", size: 100, wantErr: true},
		{name: "start at file size", header: "bytes=100-", size: 100, wantErr: true, wantNoOverlap: true},
		{name: "empty file", header: "bytes=0-0", size: 0, wantErr: true, wantNoOverlap: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseRange(test.header, test.size)
			require.Equal(t, test.want, got)
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.wantNoOverlap {
				require.ErrorIs(t, err, errNoOverlap)
			}
		})
	}
}
