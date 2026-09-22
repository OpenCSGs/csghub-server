package aliyunchecker

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	v1 "opencsg.com/csghub-server/plugins/aliyun_checker/v1"
)

type fakeAliyunCheckerClient struct {
	v1.AliyunCheckerClient
	stream *fakeImageClientStream
	llmReq *v1.PassLLMCheckRequest
}

func (f *fakeAliyunCheckerClient) PassImageStreamCheck(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[v1.ImageStreamFrame, v1.CheckResult], error) {
	return f.stream, nil
}

func (f *fakeAliyunCheckerClient) PassLLMCheck(ctx context.Context, req *v1.PassLLMCheckRequest, opts ...grpc.CallOption) (*v1.CheckResult, error) {
	f.llmReq = req
	return &v1.CheckResult{IsSensitive: true, Reason: "politics"}, nil
}

type fakeImageClientStream struct {
	grpc.ClientStream
	sent   []*v1.ImageStreamFrame
	closed bool
}

func (f *fakeImageClientStream) Send(frame *v1.ImageStreamFrame) error {
	f.sent = append(f.sent, frame)
	return nil
}

func (f *fakeImageClientStream) CloseAndRecv() (*v1.CheckResult, error) {
	f.closed = true
	return &v1.CheckResult{IsSensitive: false}, nil
}

func (f *fakeImageClientStream) Context() context.Context {
	return context.Background()
}

func TestClientPassImageStreamCheck(t *testing.T) {
	stream := &fakeImageClientStream{}
	client := NewClient(&fakeAliyunCheckerClient{stream: stream})

	result, err := client.PassImageStreamCheck(context.Background(), "baselineCheck", strings.NewReader("abcdef"))
	require.NoError(t, err)
	require.False(t, result.IsSensitive)
	require.True(t, stream.closed)
	require.Len(t, stream.sent, 2)
	require.Equal(t, "baselineCheck", stream.sent[0].GetHeader().GetScenario())
	require.Equal(t, "abcdef", string(stream.sent[1].GetChunk()))
}

func TestClientPassLLMCheck(t *testing.T) {
	pbClient := &fakeAliyunCheckerClient{}
	client := NewClient(pbClient)

	result, err := client.PassLLMCheck(context.Background(), &LLMCheckRequest{
		Scenario: "llm_query_moderation", Text: "prompt", SessionID: "session", AccountID: "account",
		MaxTokens: 128, RawJSON: "{}", Resumable: true, ModelName: "model", Role: "user", Stream: true,
		IsAppendSystemPromot: true,
	})
	require.NoError(t, err)
	require.True(t, result.IsSensitive)
	require.Equal(t, "account", pbClient.llmReq.GetAccountId())
	require.Equal(t, "model", pbClient.llmReq.GetModelName())
	require.True(t, pbClient.llmReq.GetAppendSystemPromot())
}
