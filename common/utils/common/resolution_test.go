package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseResolutionPFormat(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		ok    bool
	}{
		{"720P", 720, true},
		{"720p", 720, true},
		{"1080P", 1080, true},
		{"1080p", 1080, true},
		{"480P", 480, true},
		{"720 P", 720, true},
		{"P", 0, false},
		{"abcP", 0, false},
		{"720PX", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := parseResolutionPFormat(tt.input)
		require.Equal(t, tt.ok, ok, "parseResolutionPFormat(%q) ok", tt.input)
		if ok {
			require.Equal(t, tt.want, got, "parseResolutionPFormat(%q) value", tt.input)
		}
	}
}

func TestParseResolutionWxHFormat(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		ok    bool
	}{
		{"720x1280", 1280, true},
		{"1280x720", 1280, true},
		{"1920x1080", 1920, true},
		{"1080X1920", 1920, true},
		{"1920×1080", 1920, true},
		{"720 x 1280", 1280, true},
		{"0x0", 0, true},
		{"720x", 0, false},
		{"x720", 0, false},
		{"720x1280x3", 0, false},
		{"abcx720", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := parseResolutionWxHFormat(tt.input)
		require.Equal(t, tt.ok, ok, "parseResolutionWxHFormat(%q) ok", tt.input)
		if ok {
			require.Equal(t, tt.want, got, "parseResolutionWxHFormat(%q) value", tt.input)
		}
	}
}

func TestParseResolutionPureFormat(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		ok    bool
	}{
		{"720", 720, true},
		{"1080", 1080, true},
		{"0", 0, true},
		{"1", 1, true},
		{"720P", 0, false},
		{"720x1280", 0, false},
		{"abc", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := parseResolutionPureFormat(tt.input)
		require.Equal(t, tt.ok, ok, "parseResolutionPureFormat(%q) ok", tt.input)
		if ok {
			require.Equal(t, tt.want, got, "parseResolutionPureFormat(%q) value", tt.input)
		}
	}
}

func TestExtractEventResolutionMaxSide(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		err   bool
	}{
		{"", 0, false},
		{"  ", 0, false},
		{"720p", 720, false},
		{"1080P", 1080, false},
		{"720", 720, false},
		{"1080", 1080, false},
		{"720x1280", 1280, false},
		{"1920x1080", 1920, false},
		{"1920×1080", 1920, false},
		{"invalid", 0, true},
		{"abcxdef", 0, true},
	}

	for _, tt := range tests {
		got, err := ExtractEventResolutionMaxSide(tt.input)
		if tt.err {
			require.Error(t, err, "extractEventResolutionMaxSide(%q)", tt.input)
		} else {
			require.NoError(t, err, "extractEventResolutionMaxSide(%q)", tt.input)
			require.Equal(t, tt.want, got, "extractEventResolutionMaxSide(%q) value", tt.input)
		}
	}
}

func TestParseResolutionMaxSide(t *testing.T) {
	tests := []struct {
		name     string
		size     string
		wantSide int64
		wantOK   bool
	}{
		{"square", "1024x1024", 1024, true},
		{"landscape", "1280x720", 1280, true},
		{"portrait", "720x1280", 1280, true},
		{"whitespace wxh", "  1024  x  1536  ", 1536, true},
		{"uppercase wxh", "1024X1024", 1024, true},
		{"preset uppercase", "1080P", 1080, true},
		{"preset lowercase", "720p", 720, true},
		{"whitespace preset", "  1080p  ", 1080, true},
		{"pure integer", "1024", 1024, true},
		{"whitespace integer", "  2048  ", 2048, true},
		{"empty", "", 0, false},
		{"degenerate wxh one side zero", "0x1024", 1024, true}, // matches ExtractEventResolutionMaxSide leniency for a single zero dimension
		{"wxh both sides zero", "0x0", 0, false},
		{"negative wxh", "-1024x1024", 0, false},
		{"zero preset", "0p", 0, false},
		{"zero integer", "0", 0, false},
		{"negative integer", "-1024", 0, false},
		{"non numeric", "abcxdef", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			side, ok := ParseResolutionMaxSide(tt.size)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantSide, side)
		})
	}
}
