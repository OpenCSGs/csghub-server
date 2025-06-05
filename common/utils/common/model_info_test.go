package common

import (
	"testing"
)

func TestTorchDtypeToSafetensors(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "float16 convert",
			input:    "float16",
			expected: "F16",
		},
		{
			name:     "float32 convert",
			input:    "float32",
			expected: "F32",
		},
		{
			name:     "float64 convert",
			input:    "float64",
			expected: "F64",
		},
		{
			name:     "bfloat16 convert",
			input:    "bfloat16",
			expected: "BF16",
		},

		{
			name:     "int8 convert",
			input:    "int8",
			expected: "I8",
		},
		{
			name:     "int16 convert",
			input:    "int16",
			expected: "I16",
		},
		{
			name:     "int32 convert",
			input:    "int32",
			expected: "I32",
		},
		{
			name:     "int64 convert",
			input:    "int64",
			expected: "I64",
		},
		{
			name:     "uint8 convert",
			input:    "uint8",
			expected: "U8",
		},
		{
			name:     "uint16 convert",
			input:    "uint16",
			expected: "U16",
		},
		{
			name:     "uint32 convert",
			input:    "uint32",
			expected: "U32",
		},
		{
			name:     "uint64 convert",
			input:    "uint64",
			expected: "U64",
		},

		{
			name:     "byte alias convert",
			input:    "byte",
			expected: "U8",
		},
		{
			name:     "short alias convert",
			input:    "short",
			expected: "I16",
		},
		{
			name:     "int alias convert",
			input:    "int",
			expected: "I32",
		},
		{
			name:     "long alias convert",
			input:    "long",
			expected: "I64",
		},

		{
			name:     "bool convert",
			input:    "bool",
			expected: "BOOL",
		},

		{
			name:     "complex64 convert",
			input:    "complex64",
			expected: "C64",
		},
		{
			name:     "complex128 convert",
			input:    "complex128",
			expected: "C128",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TorchDtypeToSafetensors(tc.input)
			if result != tc.expected {
				t.Errorf("TorchDtypeToSafetensors(%q) = %q, expect %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestTorchDtypeToSafetensors_CaseSensitivity(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "float16",
			input:    "float16",
			expected: "F16",
		},
		{
			name:     "FLOAT16",
			input:    "FLOAT16",
			expected: "FLOAT16",
		},
		{
			name:     "Float16",
			input:    "Float16",
			expected: "FLOAT16",
		},
		{
			name:     "bfloat16",
			input:    "bfloat16",
			expected: "BF16",
		},
		{
			name:     "BFLOAT16",
			input:    "BFLOAT16",
			expected: "BFLOAT16",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TorchDtypeToSafetensors(tc.input)
			if result != tc.expected {
				t.Errorf("TorchDtypeToSafetensors(%q) = %q, expect %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestTorchDtypeToSafetensors_WithSpaces(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "leading and trailing spaces",
			input:    "  float16  ",
			expected: "  FLOAT16  ",
		},
		{
			name:     "has spaces in middle",
			input:    "float 16",
			expected: "FLOAT 16",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TorchDtypeToSafetensors(tc.input)
			if result != tc.expected {
				t.Errorf("TorchDtypeToSafetensors(%q) = %q, expect %q", tc.input, result, tc.expected)
			}
		})
	}
}

func BenchmarkTorchDtypeToSafetensors(b *testing.B) {
	testInputs := []string{
		"float16", "float32", "float64", "bfloat16",
		"int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64",
		"bool", "complex64", "complex128",
		"half", "float", "double", "byte", "short", "int", "long",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range testInputs {
			TorchDtypeToSafetensors(input)
		}
	}
}

func TestGetBytesPerParam(t *testing.T) {
	testCases := []struct {
		name     string
		dtype    string
		expected int
	}{
		{
			name:     "F16 Type",
			dtype:    "F16",
			expected: 2,
		},
		{
			name:     "BF16 Type",
			dtype:    "BF16",
			expected: 2,
		},
		{
			name:     "F32 Type",
			dtype:    "F32",
			expected: 4,
		},
		{
			name:     "F64 Type",
			dtype:    "F64",
			expected: 8,
		},
		{
			name:     "I8 Type",
			dtype:    "I8",
			expected: 1,
		},
		{
			name:     "U8 Type",
			dtype:    "U8",
			expected: 1,
		},
		{
			name:     "I32 Type",
			dtype:    "I32",
			expected: 4,
		},
		{
			name:     "I64 Type",
			dtype:    "I64",
			expected: 8,
		},
		{
			name:     "U64 Type",
			dtype:    "U64",
			expected: 8,
		},

		{
			name:     "小写f16",
			dtype:    "f16",
			expected: 2,
		},
		{
			name:     "bf16",
			dtype:    "bf16",
			expected: 2,
		},

		{
			name:     "Type",
			dtype:    "unknown",
			expected: 4,
		},
		{
			name:     "",
			dtype:    "",
			expected: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetBytesPerParam(tc.dtype)
			if result != tc.expected {
				t.Errorf("GetBytesPerParam(%q) = %d, expect %d", tc.dtype, result, tc.expected)
			}
		})
	}
}

func TestTorchDtypeToSafetensors_Integration(t *testing.T) {
	testCases := []struct {
		torchDtype       string
		expectedSafetype string
		expectedBytes    int
	}{
		{
			torchDtype:       "float16",
			expectedSafetype: "F16",
			expectedBytes:    2,
		},
		{
			torchDtype:       "bfloat16",
			expectedSafetype: "BF16",
			expectedBytes:    2,
		},
		{
			torchDtype:       "float32",
			expectedSafetype: "F32",
			expectedBytes:    4,
		},
		{
			torchDtype:       "int8",
			expectedSafetype: "I8",
			expectedBytes:    1,
		},
		{
			torchDtype:       "int64",
			expectedSafetype: "I64",
			expectedBytes:    8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.torchDtype, func(t *testing.T) {
			safetensorsType := TorchDtypeToSafetensors(tc.torchDtype)
			if safetensorsType != tc.expectedSafetype {
				t.Errorf("TorchDtypeToSafetensors(%q) = %q, 期望 %q",
					tc.torchDtype, safetensorsType, tc.expectedSafetype)
			}

			bytes := GetBytesPerParam(safetensorsType)
			if bytes != tc.expectedBytes {
				t.Errorf("GetBytesPerParam(%q) = %d, 期望 %d",
					safetensorsType, bytes, tc.expectedBytes)
			}
		})
	}
}
