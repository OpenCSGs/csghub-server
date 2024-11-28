package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mocktypes "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestSensitiveComponent_CheckText(t *testing.T) {
	mockModeration := mockrpc.NewMockModerationSvcClient(t)
	mockModeration.EXPECT().PassTextCheck(mock.Anything, mock.Anything, mock.Anything).Return(&rpc.CheckResult{
		IsSensitive: false,
	}, nil)
	comp := &sensitiveComponentImpl{
		checker: mockModeration,
	}

	success, err := comp.CheckText(context.TODO(), string(sensitive.ScenarioChatDetection), "test")
	require.Nil(t, err)
	require.True(t, success)
}

func TestSensitiveComponent_CheckImage(t *testing.T) {
	mockModeration := mockrpc.NewMockModerationSvcClient(t)
	mockModeration.EXPECT().PassImageCheck(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&rpc.CheckResult{
		IsSensitive: false,
	}, nil)
	comp := &sensitiveComponentImpl{
		checker: mockModeration,
	}

	success, err := comp.CheckImage(context.TODO(), string(sensitive.ScenarioChatDetection), "ossBucketName", "ossObjectName")
	require.Nil(t, err)
	require.True(t, success)
}

func TestSensitiveComponent_CheckRequestV2(t *testing.T) {
	mockModeration := mockrpc.NewMockModerationSvcClient(t)
	mockModeration.EXPECT().PassTextCheck(mock.Anything, mock.Anything, mock.Anything).Return(&rpc.CheckResult{
		IsSensitive: false,
	}, nil).Twice()
	comp := &sensitiveComponentImpl{
		checker: mockModeration,
	}
	mockRequest := mocktypes.NewMockSensitiveRequestV2(t)
	mockRequest.EXPECT().GetSensitiveFields().Return([]types.SensitiveField{
		{
			Name: "chat",
			Value: func() string {
				return "chat1"
			},
			Scenario: string(sensitive.ScenarioChatDetection),
		},
		{
			Name: "comment",
			Value: func() string {
				return "comment1"
			},
			Scenario: string(sensitive.ScenarioCommentDetection),
		},
	})
	success, err := comp.CheckRequestV2(context.TODO(), mockRequest)
	require.Nil(t, err)
	require.True(t, success)
}

func TestSensitiveComponent_NoOpImpl(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = false
	c, err := NewSensitiveComponent(cfg)
	require.Nil(t, err)

	success, err := c.CheckText(context.Background(), string(sensitive.ScenarioChatDetection), "test")
	require.Nil(t, err)
	require.True(t, success)

	success, err = c.CheckImage(context.Background(), string(sensitive.ScenarioChatDetection), "test", "test")
	require.Nil(t, err)
	require.True(t, success)

	mockRequest := mocktypes.NewMockSensitiveRequestV2(t)

	success, err = c.CheckRequestV2(context.Background(), mockRequest)
	require.Nil(t, err)
	require.True(t, success)
}
