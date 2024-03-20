package imagebuilder

import (
	"context"
)

var _ Builder = (*LocalBuilder)(nil)

type LocalBuilder struct{}

func NewLocalBuilder() *LocalBuilder {
	return &LocalBuilder{}
}

// Build implements Builder.Build
func (*LocalBuilder) Build(ctx context.Context, req *BuildRequest) (*BuildResponse, error) {
	response := &BuildResponse{}

	return response, nil
}

// Logs implements Builder.Logs
func (*LocalBuilder) Logs(ctx context.Context, req *LogsRequest) (*LogsResponse, error) {
	response := &LogsResponse{}
	return response, nil
}

// Status implements Builder.Status
func (*LocalBuilder) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	responses := &StatusResponse{
		// Code:    req.CurrentStatus + 1,
		Code:    3,
		Message: "build completed",
		ImageID: "gradio-test-app:v1.0",
	}
	return responses, nil
}
