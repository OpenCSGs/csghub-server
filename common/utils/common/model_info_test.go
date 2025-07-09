package common

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// Mock functions for testing fetchSafetensorsMetadata
func mockFetchSafetensorsMetadataSuccess(url string) (map[string]any, error) {
	return map[string]any{
		"weight1": map[string]any{
			"dtype": "F32",
			"shape": []any{7168.0, 16384.0},
		},
		"weight2": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight3": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight4": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight5": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight6": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight7": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight8": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight9": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},
		"weight10": map[string]any{
			"dtype": "I64",
			"shape": []any{7168.0, 16384.0},
		},

		"__metadata__": map[string]any{},
	}, nil
}

func mockFetchSafetensorsMetadataError(url string) (map[string]any, error) {
	return nil, errors.New("mock error")
}

func mockFetchSafetensorsMetadataLargeHeader(url string) (map[string]any, error) {
	return nil, fmt.Errorf("header size exceeds maximum allowed size: 1024000 bytes,header length: 1025000")
}

func mockFetchSafetensorsMetadataMultipleDtypes(url string) (map[string]any, error) {
	return map[string]any{
		"weight1": map[string]any{
			"dtype": "F32",
			"shape": []any{10.0, 20.0},
		},
		"weight2": map[string]any{
			"dtype": "F16",
			"shape": []any{5.0, 8.0},
		},
		"weight3": map[string]any{
			"dtype": "BF16",
			"shape": []any{3.0, 4.0},
		},
		"__metadata__": map[string]any{},
	}, nil
}

func mockFetchSafetensorsMetadataInvalidShape(url string) (map[string]any, error) {
	return map[string]any{
		"weight1": map[string]any{
			"dtype": "F32",
			"shape": "invalid_shape",
		},
		"__metadata__": map[string]any{},
	}, nil
}

func mockFetchSafetensorsMetadataInvalidTensorData(url string) (map[string]any, error) {
	return map[string]any{
		"weight1":      "invalid_tensor_data",
		"__metadata__": map[string]any{},
	}, nil
}

func mockFetchSafetensorsMetadataEmptyTensors(url string) (map[string]any, error) {
	return map[string]any{
		"__metadata__": map[string]any{},
	}, nil
}

// Helper function to patch fetchSafetensorsMetadata for tests
func patchFetchSafetensorsMetadata(f func(string) (map[string]any, error)) func() {
	orig := fetchSafetensorsMetadata
	fetchSafetensorsMetadata = f
	return func() { fetchSafetensorsMetadata = orig }
}

func TestGetModelInfo_Success(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataSuccess)
	defer restore()

	files := []string{"file1", "file2"}
	minContext := 128

	got, err := GetModelInfo(files, minContext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// weight1: 2*3=6 params, F32=4 bytes
	// weight2: 4*5=20 params, F16=2 bytes
	// For two files: params double
	wantParams := int64(2348810240)
	// ModelWeightsGB calculation: ((6*4)+(20*2))*2 = (24+40)*2 = 128 bytes
	// 128 / (1024*1024*1024) = very small number, but integer division truncates to 0
	wantModelWeightsGB := float32(16)

	if got.TotalParams != wantParams {
		t.Errorf("TotalParams = %v, want %v", got.TotalParams, wantParams)
	}
	if got.ParamsBillions != float32(math.Round(float64(wantParams)/1e9*100)/100) {
		t.Errorf("ParamsBillions = %v, want %v", got.ParamsBillions, float32(math.Round(float64(wantParams)/1e9*100)/100))
	}
	if got.ModelWeightsGB != wantModelWeightsGB {
		t.Errorf("ModelWeightsGB = %v, want %v", got.ModelWeightsGB, wantModelWeightsGB)
	}
	if got.MiniGPUMemoryGB != 1 {
		t.Errorf("MiniGPUMemoryGB = %v, want 1", got.MiniGPUMemoryGB)
	}
	if got.ContextSize != minContext {
		t.Errorf("ContextSize = %v, want %v", got.ContextSize, minContext)
	}
	if got.BatchSize != 1 {
		t.Errorf("BatchSize = %v, want 1", got.BatchSize)
	}
	// BytesPerParam is set to the last processed tensor's bytes per param
	// Since map iteration order is not guaranteed, we just check it's valid
	if got.BytesPerParam != 8 && got.BytesPerParam != 4 {
		t.Errorf("BytesPerParam = %v, want 2 or 4", got.BytesPerParam)
	}
}

func TestGetModelInfo_MetadataError(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataError)
	defer restore()

	files := []string{"file1"}
	_, err := GetModelInfo(files, 64)
	if err == nil || err.Error() != "failed to fetch metadata: mock error, url: file1" {
		t.Errorf("expected fetch error, got %v", err)
	}
}

func TestGetModelInfo_HeaderTooLarge(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataLargeHeader)
	defer restore()

	files := []string{"file1"}
	_, err := GetModelInfo(files, 64)
	if err == nil || !reflect.DeepEqual(err.Error(), "failed to fetch metadata: header size exceeds maximum allowed size: 1024000 bytes,header length: 1025000, url: file1") {
		t.Errorf("expected header size error, got %v", err)
	}
}

func TestGetModelInfo_EmptyFileList(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataSuccess)
	defer restore()

	files := []string{}
	got, err := GetModelInfo(files, 32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalParams != 0 {
		t.Errorf("TotalParams = %v, want 0", got.TotalParams)
	}
	if got.ContextSize != 32 {
		t.Errorf("ContextSize = %v, want 32", got.ContextSize)
	}
	if got.BatchSize != 1 {
		t.Errorf("BatchSize = %v, want 1", got.BatchSize)
	}
	if got.MiniGPUMemoryGB != 1 {
		t.Errorf("MiniGPUMemoryGB = %v, want 1", got.MiniGPUMemoryGB)
	}
}

func TestGetModelInfo_MultipleDtypes(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataMultipleDtypes)
	defer restore()

	files := []string{"file1"}
	got, err := GetModelInfo(files, 256)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// weight1: 10*20=200 params, F32=4 bytes
	// weight2: 5*8=40 params, F16=2 bytes
	// weight3: 3*4=12 params, BF16=2 bytes
	wantParams := int64(200 + 40 + 12)
	wantBytesPerParam := 2 // last dtype is BF16

	if got.TotalParams != wantParams {
		t.Errorf("TotalParams = %v, want %v", got.TotalParams, wantParams)
	}
	if got.BytesPerParam != wantBytesPerParam {
		t.Errorf("BytesPerParam = %v, want %v", got.BytesPerParam, wantBytesPerParam)
	}
}

func TestGetModelInfo_InvalidShape(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataInvalidShape)
	defer restore()

	files := []string{"file1"}
	got, err := GetModelInfo(files, 64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip invalid tensors and return zero params
	if got.TotalParams != 0 {
		t.Errorf("TotalParams = %v, want 0", got.TotalParams)
	}
}

func TestGetModelInfo_InvalidTensorData(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataInvalidTensorData)
	defer restore()

	files := []string{"file1"}
	got, err := GetModelInfo(files, 64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip invalid tensor data and return zero params
	if got.TotalParams != 0 {
		t.Errorf("TotalParams = %v, want 0", got.TotalParams)
	}
}

func TestGetModelInfo_EmptyTensors(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataEmptyTensors)
	defer restore()

	files := []string{"file1"}
	got, err := GetModelInfo(files, 64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle empty tensors gracefully
	if got.TotalParams != 0 {
		t.Errorf("TotalParams = %v, want 0", got.TotalParams)
	}
	if got.TensorType != "" {
		t.Errorf("TensorType = %q, want empty string", got.TensorType)
	}
}

func TestGetModelInfo_SingleFile(t *testing.T) {
	restore := patchFetchSafetensorsMetadata(mockFetchSafetensorsMetadataSuccess)
	defer restore()

	files := []string{"file1"}
	minContext := 512

	got, err := GetModelInfo(files, minContext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// weight1: 2*3=6 params, F32=4 bytes
	// weight2: 4*5=20 params, F16=2 bytes
	// For single file: no doubling
	wantParams := int64(1174405120)

	if got.TotalParams != wantParams {
		t.Errorf("TotalParams = %v, want %v", got.TotalParams, wantParams)
	}
	// BytesPerParam is set to the last processed tensor's bytes per param
	// Since map iteration order is not guaranteed, we just check it's valid
	if got.BytesPerParam != 8 && got.BytesPerParam != 4 {
		t.Errorf("BytesPerParam = %v, want 2 or 4", got.BytesPerParam)
	}
	if got.ContextSize != minContext {
		t.Errorf("ContextSize = %v, want %v", got.ContextSize, minContext)
	}
}

func TestGetModelInfo_LargeModel(t *testing.T) {
	// Mock for a large model with billions of parameters
	mockLargeModel := func(url string) (map[string]any, error) {
		return map[string]any{
			"weight1": map[string]any{
				"dtype": "F16",
				"shape": []any{4096.0, 4096.0}, // 16M params
			},
			"weight2": map[string]any{
				"dtype": "F16",
				"shape": []any{4096.0, 11008.0}, // 45M params
			},
			"__metadata__": map[string]any{},
		}, nil
	}

	restore := patchFetchSafetensorsMetadata(mockLargeModel)
	defer restore()

	files := []string{"file1"}
	got, err := GetModelInfo(files, 2048)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// weight1: 4096*4096 = 16,777,216 params
	// weight2: 4096*11008 = 45,088,768 params
	// Total: 61,865,984 params ≈ 0.06 billion
	wantParams := int64(4096*4096 + 4096*11008)
	expectedBillions := float32(math.Round(float64(wantParams)/1e9*100) / 100)

	if got.TotalParams != wantParams {
		t.Errorf("TotalParams = %v, want %v", got.TotalParams, wantParams)
	}
	if got.ParamsBillions != expectedBillions {
		t.Errorf("ParamsBillions = %v, want %v", got.ParamsBillions, expectedBillions)
	}
}
