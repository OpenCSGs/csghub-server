package common

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

type TensorSummary struct {
	Name      string `json:"name"`
	Shape     []int  `json:"shape"`
	DataType  string `json:"data_type"`
	SizeBytes int64  `json:"size_bytes"`
}

// GetModelInfo fetches and parses metadata from a list of files to extract model information
// file List contains the whole path of the file
// https://hub.opencsg.com/csg/Qwen/Qwen2-1.5B-Instruct/resolve/main/model-00001-of-0002.safetensors
// https://hub.opencsg.com/csg/Qwen/Qwen2-1.5B-Instruct/resolve/main/model-00001-of-0002.safetensors
func GetModelInfo(fileList []string, minContext int) (*types.ModelInfo, error) {

	modelInfo := &types.ModelInfo{}
	var totalParams int64
	var totalMemoryBytes int64
	var modelSize int64
	var bytesPerParam int
	headerTooBig := false
	for _, file := range fileList {
		header, err := fetchSafetensorsMetadata(file)
		// check error if it contains exceeds maximum allowed size
		if err != nil {
			if strings.Contains(err.Error(), "header size exceeds maximum allowed size") {
				// such as deepseek-ai/DeepSeek-R1-0528
				headerTooBig = true
				break
			}
			return nil, fmt.Errorf("failed to fetch metadata: %v", err)
		}
		delete(header, "__metadata__")

		for _, value := range header {
			tensorData, ok := value.(map[string]interface{})
			if !ok {
				continue
			}

			dtype, ok := tensorData["dtype"].(string)
			if ok && !strings.Contains(modelInfo.TensorType, dtype) {
				modelInfo.TensorType += dtype + " "
			}

			shape, err := extractShape(tensorData)
			if err != nil {
				continue
			}

			tensorParams := calculateTensorParams(shape)
			totalParams += tensorParams

			bytesPerParam = GetBytesPerParam(dtype)
			tensorMemoryBytes := tensorParams * int64(bytesPerParam)
			modelSize += tensorMemoryBytes
		}

		modelInfo.TotalParams = totalParams
		modelInfo.ParamsBillions = float32(math.Round(float64(totalParams)/1e9*100) / 100)
	}
	if headerTooBig {
		return GetModelInfoWithoutParameters(fileList, minContext)
	}
	modelInfo.ModelWeightsGB = float32(modelSize / (1024 * 1024 * 1024))
	modelInfo.MiniGPUMemoryGB = max(float32(totalMemoryBytes/(1024*1024*1024)), 1)
	// min context for min gpu memory
	modelInfo.ContextSize = minContext
	modelInfo.BatchSize = 1
	modelInfo.BytesPerParam = bytesPerParam
	modelInfo.TensorType = strings.TrimSpace(modelInfo.TensorType)
	return modelInfo, nil
}

// get model info from model index
func GetModelInfoWithoutParameters(fileList []string, minContext int) (*types.ModelInfo, error) {
	modelInfo := &types.ModelInfo{}
	for _, file := range fileList {
		size, err := GetFileSize(file)
		if err != nil {
			return nil, err
		}
		modelInfo.ModelWeightsGB += size
	}
	modelInfo.ContextSize = minContext
	modelInfo.BatchSize = 1

	return modelInfo, nil
}

func ExtraOverhead(modelSize int64) float32 {
	return float32(modelSize) * 0.05
}

func GetActivationMemory(batchSize, seqLength, numLayers, hiddenSize, numHeads, bytesPerParam int) float32 {
	batchF := float32(batchSize)
	seqF := float32(seqLength)
	headsF := float32(numHeads)
	hiddenF := float32(hiddenSize)
	sizeF := float32(bytesPerParam)
	activationFactor := 34.0 + ((5.0 * seqF * headsF) / hiddenF)
	total := batchF * seqF * hiddenF * activationFactor * sizeF
	return total / (1024 * 1024 * 1024)
}

func GetKvCacheSize(contextSize, batchSize, hiddenSize, numHiddenLayers, bytesPerParam int) float32 {
	activateBytes := 2 * batchSize * contextSize * numHiddenLayers * hiddenSize * bytesPerParam
	return float32(activateBytes / (1024 * 1024 * 1024))
}

// GetLoRAFinetuneMemory estimates the memory required for fine-tuning a model with LoRA
func GetLoRAFinetuneMemory(modelWeightsGB, totalParams float32, batchSize, contextSize, hiddenSize, numHiddenLayers, numAttentionHeads, bytesPerParam int, loraRank int) float32 {
	modelWeights := modelWeightsGB
	loraRatio := float32(loraRank) / 1000.0
	if loraRatio > 0.05 {
		loraRatio = 0.05
	}
	loraParamsGB := totalParams * loraRatio * float32(bytesPerParam) / (1024 * 1024 * 1024)
	gradientsGB := loraParamsGB
	optimizerStateGB := loraParamsGB * 2
	activationMemoryGB := GetActivationMemory(batchSize, contextSize, numHiddenLayers, hiddenSize, numAttentionHeads, bytesPerParam)
	overHead := float32(1.0)
	totalMemoryGB := modelWeights + loraParamsGB + gradientsGB + optimizerStateGB + activationMemoryGB + overHead
	return float32(math.Round(float64(totalMemoryGB)*100)) / 100
}

func extractShape(tensorData map[string]any) ([]int, error) {
	shapeRaw, ok := tensorData["shape"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid shape format")
	}

	shape := make([]int, len(shapeRaw))
	for i, dim := range shapeRaw {
		dimFloat, ok := dim.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid shape dimension")
		}
		shape[i] = int(dimFloat)
	}
	return shape, nil
}

func calculateTensorParams(shape []int) int64 {
	var tensorParams int64 = 1
	for _, dim := range shape {
		tensorParams *= int64(dim)
	}
	return tensorParams
}

// fetchSafetensorsMetadata fetches the metadata header from a safetensors file
func fetchSafetensorsMetadata(url string) (map[string]any, error) {
	// Create a custom http.Transport that skips TLS verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	if strings.HasPrefix(url, "http://") {
		client = &http.Client{}
	}

	// Fetch the first 8 bytes of the file
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Range", "bytes=0-7")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("failed to fetch metadata: %v", resp.Status)
	}
	defer resp.Body.Close()

	// Read the first 8 bytes
	lengthBytes := make([]byte, 8)
	_, err = io.ReadFull(resp.Body, lengthBytes)
	if err != nil {
		return nil, err
	}

	// Interpret the bytes as a little-endian unsigned 64-bit integer
	lengthOfHeader := binary.LittleEndian.Uint64(lengthBytes)

	maxSize := uint64(1000 * 1024)
	if lengthOfHeader > maxSize {
		return nil, fmt.Errorf("header size exceeds maximum allowed size: %d bytes,header length: %d", maxSize, lengthOfHeader)
	}

	// Fetch length_of_header bytes starting from the 9th byte
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=8-%d", 7+lengthOfHeader))

	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	headerBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Interpret the response as a JSON object
	var header map[string]any
	err = json.Unmarshal(headerBytes, &header)
	if err != nil {
		return nil, err
	}

	return header, nil
}

// getBytesPerParam returns the number of bytes per parameter for a given data type
func GetBytesPerParam(dtype string) int {
	dtype = strings.ToUpper(dtype)
	switch {
	case strings.Contains(dtype, "F16") || strings.Contains(dtype, "BF16"):
		return 2
	case strings.Contains(dtype, "F64"):
		return 8
	case strings.Contains(dtype, "F8"):
		return 1
	case strings.Contains(dtype, "I8") || strings.Contains(dtype, "U8"):
		return 1
	case strings.Contains(dtype, "I32") || strings.Contains(dtype, "U32"):
		return 4
	case strings.Contains(dtype, "I64") || strings.Contains(dtype, "U64"):
		return 8
	default:
		// Default to float32 (4 bytes)
		return 4
	}
}

func GetFileSize(fileURL string) (float32, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	if strings.HasPrefix(fileURL, "http://") {
		client = &http.Client{}
	}

	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Range", "bytes=0-0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("remote server does not support range request ,status code: %d", resp.StatusCode)
	}

	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		return 0, fmt.Errorf("empty Content-Range")
	}

	var start, end, total int64
	_, err = fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &total)
	if err != nil || total <= 0 {
		return 0, fmt.Errorf("can not parse Content-Range: %s", contentRange)
	}

	sizeInGB := float32(total) / (1024 * 1024 * 1024)
	return sizeInGB, nil
}

func TorchDtypeToSafetensors(dtype string) string {

	dtypeMap := map[string]string{
		"float16":  "F16",
		"float32":  "F32",
		"float64":  "F64",
		"bfloat16": "BF16",
		"half":     "F16",
		"float":    "F32",
		"double":   "F64",

		"int8":   "I8",
		"int16":  "I16",
		"int32":  "I32",
		"int64":  "I64",
		"uint8":  "U8",
		"uint16": "U16",
		"uint32": "U32",
		"uint64": "U64",
		"byte":   "U8",
		"short":  "I16",
		"int":    "I32",
		"long":   "I64",

		"bool": "BOOL",

		"complex64":  "C64",
		"complex128": "C128",
	}

	if safetensorsName, exists := dtypeMap[dtype]; exists {
		return safetensorsName
	}

	return strings.ToUpper(dtype)
}
