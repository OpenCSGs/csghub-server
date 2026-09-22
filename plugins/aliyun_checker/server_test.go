package aliyunchecker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	v1 "opencsg.com/csghub-server/plugins/aliyun_checker/v1"
)

type fakeChecker struct {
	scenario string
	text     string
	content  []byte
	llmReq   *LLMCheckRequest
	mediaReq *MediaModerationRequest

	result *CheckResult
	err    error
}

func (f *fakeChecker) PassTextCheck(ctx context.Context, scenario, text string) (*CheckResult, error) {
	f.scenario, f.text = scenario, text
	return f.result, f.err
}

func (f *fakeChecker) PassImageCheck(ctx context.Context, scenario, bucket, object string) (*CheckResult, error) {
	f.scenario = scenario
	return f.result, f.err
}

func (f *fakeChecker) PassImageURLCheck(ctx context.Context, scenario, url string) (*CheckResult, error) {
	f.scenario, f.text = scenario, url
	return f.result, f.err
}

func (f *fakeChecker) PassImageStreamCheck(ctx context.Context, scenario string, reader io.Reader) (*CheckResult, error) {
	content, err := io.ReadAll(reader)
	f.scenario, f.content, f.err = scenario, content, err
	if err != nil {
		return nil, err
	}
	return f.result, nil
}

func (f *fakeChecker) PassLLMCheck(ctx context.Context, req *LLMCheckRequest) (*CheckResult, error) {
	f.llmReq = req
	return f.result, f.err
}

func (f *fakeChecker) SubmitMediaModeration(ctx context.Context, req *MediaModerationRequest) (*MediaModerationSubmission, error) {
	f.mediaReq = req
	return &MediaModerationSubmission{DataID: "data", Seed: "seed", TaskID: "task"}, f.err
}

func (f *fakeChecker) QueryMediaModerationResult(ctx context.Context, req *MediaModerationRequest) (*MediaModerationResult, error) {
	f.mediaReq = req
	return &MediaModerationResult{DataID: "data", Status: "pass"}, f.err
}

type fakeImageServerStream struct {
	grpc.ServerStream
	frames []*v1.ImageStreamFrame
	index  int
	sent   *v1.CheckResult
}

func (f *fakeImageServerStream) Recv() (*v1.ImageStreamFrame, error) {
	if f.index >= len(f.frames) {
		return nil, io.EOF
	}
	frame := f.frames[f.index]
	f.index++
	return frame, nil
}

func (f *fakeImageServerStream) SendAndClose(result *v1.CheckResult) error {
	f.sent = result
	return nil
}

func (f *fakeImageServerStream) Context() context.Context {
	return context.Background()
}

func TestServerPassTextCheck(t *testing.T) {
	checker := &fakeChecker{result: &CheckResult{IsSensitive: true, Reason: "politics"}}
	server := NewServer(checker)

	result, err := server.PassTextCheck(context.Background(), &v1.PassTextCheckRequest{
		Scenario: "comment_detection", Text: "hello",
	})
	require.NoError(t, err)
	require.True(t, result.GetIsSensitive())
	require.Equal(t, "politics", result.GetReason())
	require.Equal(t, "comment_detection", checker.scenario)
	require.Equal(t, "hello", checker.text)
}

func TestServerPassImageStreamCheck(t *testing.T) {
	checker := &fakeChecker{result: &CheckResult{}}
	server := NewServer(checker)
	stream := &fakeImageServerStream{frames: []*v1.ImageStreamFrame{
		{Frame: &v1.ImageStreamFrame_Header{Header: &v1.ImageStreamHeader{Scenario: "baselineCheck"}}},
		{Frame: &v1.ImageStreamFrame_Chunk{Chunk: []byte("abc")}},
		{Frame: &v1.ImageStreamFrame_Chunk{Chunk: []byte("def")}},
	}}

	err := server.PassImageStreamCheck(stream)
	require.NoError(t, err)
	require.Equal(t, "baselineCheck", checker.scenario)
	require.Equal(t, "abcdef", string(checker.content))
	require.NotNil(t, stream.sent)
}

func TestServerPassImageStreamCheckRequiresHeader(t *testing.T) {
	server := NewServer(&fakeChecker{})
	stream := &fakeImageServerStream{frames: []*v1.ImageStreamFrame{
		{Frame: &v1.ImageStreamFrame_Chunk{Chunk: []byte("abc")}},
	}}

	err := server.PassImageStreamCheck(stream)
	require.EqualError(t, err, "image stream must start with a header")
}

func TestServerPassLLMCheckMapsAllFields(t *testing.T) {
	checker := &fakeChecker{result: &CheckResult{}}
	server := NewServer(checker)
	req := &v1.PassLLMCheckRequest{
		Scenario: "llm_query_moderation", Text: "prompt", SessionId: "session", AccountId: "account",
		MaxTokens: 128, RawJson: "{}", Resumable: true, ModelName: "model", Role: "user", Stream: true,
		AppendSystemPromot: true,
	}

	_, err := server.PassLLMCheck(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, &LLMCheckRequest{
		Scenario: "llm_query_moderation", Text: "prompt", SessionID: "session", AccountID: "account",
		MaxTokens: 128, RawJSON: "{}", Resumable: true, ModelName: "model", Role: "user", Stream: true,
		IsAppendSystemPromot: true,
	}, checker.llmReq)
}

func TestServerMediaModeration(t *testing.T) {
	checker := &fakeChecker{}
	server := NewServer(checker)
	req := &v1.MediaModerationRequest{Url: "https://example.com/a.mp3", Type: "audio", DataId: "data"}

	submission, err := server.SubmitMediaModeration(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "task", submission.GetTaskId())
	require.Equal(t, "audio", checker.mediaReq.Type)

	result, err := server.QueryMediaModerationResult(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "pass", result.GetStatus())
	require.Equal(t, "data", checker.mediaReq.DataID)
}

func TestServerPropagatesCheckerError(t *testing.T) {
	expected := errors.New("aliyun unavailable")
	server := NewServer(&fakeChecker{err: expected})

	_, err := server.PassTextCheck(context.Background(), &v1.PassTextCheckRequest{Text: strings.Repeat("x", 4)})
	require.ErrorIs(t, err, expected)
}
