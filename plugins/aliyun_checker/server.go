package aliyunchecker

import (
	"context"
	"errors"
	"io"

	v1 "opencsg.com/csghub-server/plugins/aliyun_checker/v1"
)

type server struct {
	v1.UnimplementedAliyunCheckerServer
	checker Checker
}

func NewServer(checker Checker) v1.AliyunCheckerServer {
	return &server{checker: checker}
}

func (s *server) PassTextCheck(ctx context.Context, req *v1.PassTextCheckRequest) (*v1.CheckResult, error) {
	result, err := s.checker.PassTextCheck(ctx, req.GetScenario(), req.GetText())
	return checkResultToProto(result), err
}

func (s *server) PassImageCheck(ctx context.Context, req *v1.PassImageCheckRequest) (*v1.CheckResult, error) {
	result, err := s.checker.PassImageCheck(ctx, req.GetScenario(), req.GetOssBucketName(), req.GetOssObjectName())
	return checkResultToProto(result), err
}

func (s *server) PassImageURLCheck(ctx context.Context, req *v1.PassImageURLCheckRequest) (*v1.CheckResult, error) {
	result, err := s.checker.PassImageURLCheck(ctx, req.GetScenario(), req.GetImageUrl())
	return checkResultToProto(result), err
}

func (s *server) PassImageStreamCheck(stream v1.AliyunChecker_PassImageStreamCheckServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	header, ok := first.GetFrame().(*v1.ImageStreamFrame_Header)
	if !ok || header.Header == nil {
		return errors.New("image stream must start with a header")
	}

	result, err := s.checker.PassImageStreamCheck(
		stream.Context(), header.Header.GetScenario(), &streamReader{stream: stream})
	return sendCheckResult(stream, result, err)
}

func (s *server) PassLLMCheck(ctx context.Context, req *v1.PassLLMCheckRequest) (*v1.CheckResult, error) {
	result, err := s.checker.PassLLMCheck(ctx, lLMCheckRequestFromProto(req))
	return checkResultToProto(result), err
}

func (s *server) SubmitMediaModeration(ctx context.Context, req *v1.MediaModerationRequest) (*v1.MediaModerationSubmission, error) {
	result, err := s.checker.SubmitMediaModeration(ctx, mediaRequestFromProto(req))
	if err != nil {
		return nil, err
	}
	return &v1.MediaModerationSubmission{
		DataId: result.DataID,
		Seed:   result.Seed,
		TaskId: result.TaskID,
	}, nil
}

func (s *server) QueryMediaModerationResult(ctx context.Context, req *v1.MediaModerationRequest) (*v1.MediaModerationResult, error) {
	result, err := s.checker.QueryMediaModerationResult(ctx, mediaRequestFromProto(req))
	if err != nil {
		return nil, err
	}
	return &v1.MediaModerationResult{
		DataId: result.DataID,
		Status: result.Status,
		Reason: result.Reason,
	}, nil
}

type streamReader struct {
	stream v1.AliyunChecker_PassImageStreamCheckServer
	buf    []byte
}

func (r *streamReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 {
		frame, err := r.stream.Recv()
		if err != nil {
			return 0, err
		}
		chunk, ok := frame.GetFrame().(*v1.ImageStreamFrame_Chunk)
		if !ok {
			return 0, errors.New("unexpected frame in image stream")
		}
		r.buf = chunk.Chunk
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func sendCheckResult(stream v1.AliyunChecker_PassImageStreamCheckServer, result *CheckResult, err error) error {
	if err != nil {
		return err
	}
	return stream.SendAndClose(checkResultToProto(result))
}

func checkResultToProto(result *CheckResult) *v1.CheckResult {
	if result == nil {
		return &v1.CheckResult{}
	}
	return &v1.CheckResult{IsSensitive: result.IsSensitive, Reason: result.Reason}
}

func lLMCheckRequestFromProto(req *v1.PassLLMCheckRequest) *LLMCheckRequest {
	if req == nil {
		return nil
	}
	return &LLMCheckRequest{
		Scenario:             req.GetScenario(),
		Text:                 req.GetText(),
		SessionID:            req.GetSessionId(),
		AccountID:            req.GetAccountId(),
		MaxTokens:            req.GetMaxTokens(),
		RawJSON:              req.GetRawJson(),
		Resumable:            req.GetResumable(),
		ModelName:            req.GetModelName(),
		Role:                 req.GetRole(),
		Stream:               req.GetStream(),
		IsAppendSystemPromot: req.GetAppendSystemPromot(),
	}
}

func mediaRequestFromProto(req *v1.MediaModerationRequest) *MediaModerationRequest {
	if req == nil {
		return nil
	}
	return &MediaModerationRequest{
		URL:    req.GetUrl(),
		Type:   req.GetType(),
		DataID: req.GetDataId(),
		Seed:   req.GetSeed(),
		TaskID: req.GetTaskId(),
	}
}

var _ io.Reader = (*streamReader)(nil)
