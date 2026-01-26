package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocktypes "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestSensitiveComponent_CheckText(t *testing.T) {
	ctx := context.TODO()
	comp := initializeTestSensitiveComponent(ctx, t)

	comp.mocks.moderationClient.EXPECT().PassTextCheck(mock.Anything, mock.Anything, mock.Anything).Return(&rpc.CheckResult{
		IsSensitive: false,
	}, nil)

	success, err := comp.CheckText(context.TODO(), types.ScenarioChatDetection, "test")
	require.Nil(t, err)
	require.True(t, success)
}

func TestSensitiveComponent_CheckImage(t *testing.T) {
	ctx := context.TODO()
	comp := initializeTestSensitiveComponent(ctx, t)

	comp.mocks.moderationClient.EXPECT().PassImageCheck(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&rpc.CheckResult{
		IsSensitive: false,
	}, nil)

	success, err := comp.CheckImage(context.TODO(), types.ScenarioChatDetection, "ossBucketName", "ossObjectName")
	require.Nil(t, err)
	require.True(t, success)
}

func TestSensitiveComponent_CheckRequestV2(t *testing.T) {
	ctx := context.TODO()
	comp := initializeTestSensitiveComponent(ctx, t)

	comp.mocks.moderationClient.EXPECT().PassTextCheck(mock.Anything, mock.Anything, mock.Anything).Return(&rpc.CheckResult{
		IsSensitive: false,
	}, nil).Twice()

	mockRequest := mocktypes.NewMockSensitiveRequestV2(t)
	mockRequest.EXPECT().GetSensitiveFields().Return([]types.SensitiveField{
		{
			Name: "chat",
			Value: func() string {
				return "chat1"
			},
			Scenario: types.ScenarioChatDetection,
		},
		{
			Name: "comment",
			Value: func() string {
				return "comment1"
			},
			Scenario: types.ScenarioCommentDetection,
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

	success, err := c.CheckText(context.Background(), types.ScenarioChatDetection, "test")
	require.Nil(t, err)
	require.True(t, success)

	success, err = c.CheckImage(context.Background(), types.ScenarioChatDetection, "test", "test")
	require.Nil(t, err)
	require.True(t, success)

	mockRequest := mocktypes.NewMockSensitiveRequestV2(t)

	success, err = c.CheckRequestV2(context.Background(), mockRequest)
	require.Nil(t, err)
	require.True(t, success)
}
