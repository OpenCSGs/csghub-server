package sensitive

import (
	"context"
	"testing"

	green20220302 "github.com/alibabacloud-go/green-20220302/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func newMediaChecker(g2c Green2022Client) *AliyunGreenChecker {
	return NewAliyunChecker(nil, g2c)
}

func TestAliyunGreenChecker_SubmitVoiceModeration(t *testing.T) {
	g2c := &mockGreen2022{}
	g2c.On("VoiceModeration", mock.MatchedBy(func(req *green20220302.VoiceModerationRequest) bool {
		return tea.StringValue(req.Service) == "audio_media_detection"
	})).Return(&green20220302.VoiceModerationResponse{
		Body: &green20220302.VoiceModerationResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VoiceModerationResponseBodyData{
				DataId: tea.String("data-1"),
				TaskId: tea.String("task-1"),
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	sub, err := c.SubmitMediaModeration(context.Background(), types.MediaModerationRequest{
		URL: "https://bucket.example/audio/a.mp3", Type: types.MediaTypeAudio, DataID: "data-1", Seed: "seed-1",
	})
	require.NoError(t, err)
	require.Equal(t, "data-1", sub.DataID)
	require.Equal(t, "seed-1", sub.Seed)
	require.Equal(t, "task-1", sub.TaskID)
}

func TestAliyunGreenChecker_SubmitVideoModeration(t *testing.T) {
	g2c := &mockGreen2022{}
	g2c.On("VideoModeration", mock.Anything).Return(&green20220302.VideoModerationResponse{
		Body: &green20220302.VideoModerationResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VideoModerationResponseBodyData{
				DataId: tea.String("data-2"),
				TaskId: tea.String("task-2"),
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	sub, err := c.SubmitMediaModeration(context.Background(), types.MediaModerationRequest{
		URL: "https://bucket.example/video/v.mp4", Type: types.MediaTypeVideo, DataID: "data-2", Seed: "seed-2",
	})
	require.NoError(t, err)
	require.Equal(t, "task-2", sub.TaskID)
}

func TestAliyunGreenChecker_SubmitMediaModeration_UnsupportedType(t *testing.T) {
	c := newMediaChecker(&mockGreen2022{})
	_, err := c.SubmitMediaModeration(context.Background(), types.MediaModerationRequest{
		URL: "https://bucket.example/x", Type: types.MediaTypeImage, DataID: "d",
	})
	require.Error(t, err)
}

func TestAliyunGreenChecker_QueryVoiceModerationResult_Pass(t *testing.T) {
	g2c := &mockGreen2022{}
	g2c.On("VoiceModerationResult", mock.MatchedBy(func(req *green20220302.VoiceModerationResultRequest) bool {
		return tea.StringValue(req.Service) == "audio_media_detection"
	})).Return(&green20220302.VoiceModerationResultResponse{
		Body: &green20220302.VoiceModerationResultResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VoiceModerationResultResponseBodyData{
				DataId:    tea.String("data-1"),
				TaskId:    tea.String("task-1"),
				RiskLevel: tea.String("none"),
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	res, err := c.QueryMediaModerationResult(context.Background(), types.MediaModerationRequest{
		Type: types.MediaTypeAudio, DataID: "data-1", TaskID: "task-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "pass", res.Status)
}

func TestAliyunGreenChecker_QueryVoiceModerationResult_PendingSliceOverridesTopLevelPass(t *testing.T) {
	g2c := &mockGreen2022{}
	g2c.On("VoiceModerationResult", mock.Anything).Return(&green20220302.VoiceModerationResultResponse{
		Body: &green20220302.VoiceModerationResultResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VoiceModerationResultResponseBodyData{
				RiskLevel: tea.String("none"),
				SliceDetails: []*green20220302.VoiceModerationResultResponseBodyDataSliceDetails{
					{RiskLevel: tea.String("")},
				},
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	res, err := c.QueryMediaModerationResult(context.Background(), types.MediaModerationRequest{
		Type: types.MediaTypeAudio, DataID: "d", TaskID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", res.Status)
}

func TestAliyunGreenChecker_QueryVoiceModerationResult_TopLevelRejectOverridesPendingSlice(t *testing.T) {
	result := mediaModerationResultFromVoice(&green20220302.VoiceModerationResultResponse{
		Body: &green20220302.VoiceModerationResultResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VoiceModerationResultResponseBodyData{
				RiskLevel: tea.String("high"),
				SliceDetails: []*green20220302.VoiceModerationResultResponseBodyDataSliceDetails{
					{RiskLevel: tea.String("")},
				},
			},
		},
	}, "d")

	assert.Equal(t, "reject", result.Status)
}

func TestAliyunGreenChecker_QueryVoiceModerationResult_Reject(t *testing.T) {
	g2c := &mockGreen2022{}
	g2c.On("VoiceModerationResult", mock.Anything).Return(&green20220302.VoiceModerationResultResponse{
		Body: &green20220302.VoiceModerationResultResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VoiceModerationResultResponseBodyData{
				RiskLevel: tea.String("high"),
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	res, err := c.QueryMediaModerationResult(context.Background(), types.MediaModerationRequest{
		Type: types.MediaTypeAudio, DataID: "d", TaskID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, "reject", res.Status)
}

func TestAliyunGreenChecker_QueryVoiceModerationResult_Pending(t *testing.T) {
	// A non-200 code means the task is still being processed.
	g2c := &mockGreen2022{}
	g2c.On("VoiceModerationResult", mock.Anything).Return(&green20220302.VoiceModerationResultResponse{
		Body: &green20220302.VoiceModerationResultResponseBody{Code: tea.Int32(404)},
	}, nil).Once()
	c := newMediaChecker(g2c)

	res, err := c.QueryMediaModerationResult(context.Background(), types.MediaModerationRequest{
		Type: types.MediaTypeAudio, DataID: "d", TaskID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", res.Status)
}

func TestAliyunGreenChecker_QueryVoiceModerationResult_EmptyRiskLevelIsPending(t *testing.T) {
	// Aliyun may return 200 + Data but an empty top-level RiskLevel while the
	// task is still finalizing. This must NOT be treated as a terminal error
	// (which would delete the comment); it must stay pending so the workflow
	// keeps polling.
	g2c := &mockGreen2022{}
	g2c.On("VoiceModerationResult", mock.Anything).Return(&green20220302.VoiceModerationResultResponse{
		Body: &green20220302.VoiceModerationResultResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VoiceModerationResultResponseBodyData{
				DataId:    tea.String("d"),
				TaskId:    tea.String("t"),
				RiskLevel: tea.String(""),
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	res, err := c.QueryMediaModerationResult(context.Background(), types.MediaModerationRequest{
		Type: types.MediaTypeAudio, DataID: "d", TaskID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", res.Status)
}

func TestAliyunGreenChecker_QueryVideoModerationResult_EmptyLevelsIsPending(t *testing.T) {
	g2c := &mockGreen2022{}
	g2c.On("VideoModerationResult", mock.Anything).Return(&green20220302.VideoModerationResultResponse{
		Body: &green20220302.VideoModerationResultResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VideoModerationResultResponseBodyData{
				RiskLevel: tea.String(""),
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	res, err := c.QueryMediaModerationResult(context.Background(), types.MediaModerationRequest{
		Type: types.MediaTypeVideo, DataID: "d", TaskID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", res.Status)
}

func TestAliyunGreenChecker_QueryVideoModerationResult_RejectFromFrame(t *testing.T) {
	g2c := &mockGreen2022{}
	g2c.On("VideoModerationResult", mock.Anything).Return(&green20220302.VideoModerationResultResponse{
		Body: &green20220302.VideoModerationResultResponseBody{
			Code: tea.Int32(200),
			Data: &green20220302.VideoModerationResultResponseBodyData{
				RiskLevel:   tea.String("low"),
				FrameResult: &green20220302.VideoModerationResultResponseBodyDataFrameResult{RiskLevel: tea.String("high")},
			},
		},
	}, nil).Once()
	c := newMediaChecker(g2c)

	res, err := c.QueryMediaModerationResult(context.Background(), types.MediaModerationRequest{
		Type: types.MediaTypeVideo, DataID: "d", TaskID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, "reject", res.Status)
}
