package tagparser

import (
	"path/filepath"
	"strings"
)

// LibraryTag parse file name or extension to match defined tag name
// see: https://git-devops.opencsg.com/product/community/open-portal/-/issues/47
func LibraryTag(filePath string) string {
	if len(filePath) == 0 {
		return ""
	}
	filename := filepath.Base(filePath)
	filename = strings.ToLower(filename)
	switch {
	case isPytorch(filename):
		return "pytorch"
	case isTensorflow(filename):
		return "tensorflow"
	case isSafetensors(filename):
		return "safetensors"
	case isJAX(filename):
		return "jax"
	case strings.HasSuffix(filename, ".onnx"):
		return "onnx"
	case strings.HasSuffix(filename, ".pdparams"):
		return "paddlepaddle"
	case strings.HasSuffix(filename, ".joblib"):
		return "joblib"
	case strings.HasSuffix(filename, ".gguf"):
		return "gguf"
	default:
		return ""
	}
}

func isPytorch(filename string) bool {
	return strings.HasPrefix(filename, "pytorch_model") && strings.HasSuffix(filename, ".bin")
}

func isTensorflow(filename string) bool {
	return strings.HasPrefix(filename, "tf_model") && strings.HasSuffix(filename, ".h5")
}
func isSafetensors(filename string) bool {
	return strings.HasPrefix(filename, "model") && strings.HasSuffix(filename, ".safetensors")
}
func isJAX(filename string) bool {
	return strings.HasPrefix(filename, "flax_model") && strings.HasSuffix(filename, ".msgpack")
}
