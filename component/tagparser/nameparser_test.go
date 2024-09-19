package tagparser

import "testing"

func TestLibraryTag(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "case insensitive", args: args{filePath: "Pytorch_model.Bin"}, want: "pytorch"},

		{name: "pytorch", args: args{filePath: "pytorch_model.bin"}, want: "pytorch"},
		{name: "pytorch", args: args{filePath: "model.pt"}, want: "pytorch"},
		{name: "pytorch", args: args{filePath: "pytorch_model_001.bin"}, want: "pytorch"},
		{name: "not pytorch", args: args{filePath: "1-pytorch_model_001.bin"}, want: ""},
		{name: "not pytorch", args: args{filePath: "pytorch_model-bin"}, want: ""},

		{name: "tensorflow", args: args{filePath: "tf_model.h5"}, want: "tensorflow"},
		{name: "tensorflow", args: args{filePath: "tf_model_001.h5"}, want: "tensorflow"},
		{name: "not tensorflow", args: args{filePath: "1-tf_model.h5"}, want: ""},
		{name: "not tensorflow", args: args{filePath: "tf_model-h5"}, want: ""},

		{name: "safetensors", args: args{filePath: "model.safetensors"}, want: "safetensors"},
		{name: "safetensors", args: args{filePath: "model_001.safetensors"}, want: "safetensors"},
		{name: "safetensors", args: args{filePath: "adpter_model.safetensors"}, want: "safetensors"},
		{name: "not safetensors", args: args{filePath: "1-test.safeten"}, want: ""},
		{name: "not safetensors", args: args{filePath: "test-safetensors"}, want: ""},

		{name: "flax_model", args: args{filePath: "flax_model.msgpack"}, want: "jax"},
		{name: "flax_model", args: args{filePath: "flax_model-001.msgpack"}, want: "jax"},
		{name: "not flax_model", args: args{filePath: "1-flax_model.msgpack"}, want: ""},
		{name: "not flax_model", args: args{filePath: "flax_model-msgpack"}, want: ""},

		{name: "onnx", args: args{filePath: "flax_model.onnx"}, want: "onnx"},
		{name: "not onnx", args: args{filePath: "flax_model-onnx"}, want: ""},

		{name: "paddlepaddle", args: args{filePath: "flax_model.pdparams"}, want: "paddlepaddle"},
		{name: "joblib", args: args{filePath: "flax_model.joblib"}, want: "joblib"},
		{name: "gguf", args: args{filePath: "flax_model.gguf"}, want: "gguf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LibraryTag(tt.args.filePath); got != tt.want {
				t.Errorf("LibraryTag() = %v, want %v", got, tt.want)
			}
		})
	}
}
