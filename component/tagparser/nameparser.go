package tagparser

import "strings"

// LibraryTag parse file name or extension to match defined tag name
// see: https://git-devops.opencsg.com/product/community/open-portal/-/issues/47
func LibraryTag(filename string) string {
	switch {
	case filename == "pytorch_model.bin":
		return "PyTorch"
	case filename == "tf_model.h5":
		return "TensorFlow"
	case filename == "model.safetensors":
		return "Safetensors"
	case filename == "flax_model.msgpack":
		return "JAX"
	case strings.HasSuffix(filename, "onnx"):
		return "ONNX"
	case strings.HasSuffix(filename, "pdparams"):
		return "PaddlePaddle"
	case strings.HasSuffix(filename, "joblib"):
		return "Joblib"
	case strings.HasSuffix(filename, "gguf"):
		return "GGUF"
	default:
		return ""
	}
}
