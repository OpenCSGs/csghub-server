package tagparser

import (
	"path/filepath"
	"strings"
)

// LibraryTag parse file name or extension to match defined tag name
// see: https://git-devops.opencsg.com/product/community/open-portal/-/issues/47
func LibraryTag(filePath string) string {
	filename := filepath.Base(filePath)
	switch {
	case filename == "pytorch_model.bin":
		return "pytorch"
	case filename == "tf_model.h5":
		return "tensorflow"
	case filename == "model.safetensors":
		return "safetensors"
	case filename == "flax_model.msgpack":
		return "jax"
	case strings.HasSuffix(filename, "onnx"):
		return "onnx"
	case strings.HasSuffix(filename, "pdparams"):
		return "paddlepaddle"
	case strings.HasSuffix(filename, "joblib"):
		return "joblib"
	case strings.HasSuffix(filename, "gguf"):
		return "gguf"
	default:
		return ""
	}
}
