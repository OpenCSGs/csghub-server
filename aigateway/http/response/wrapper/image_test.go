package wrapper

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestImageGeneration_Write_Finalize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adapter := text2image.NewOpenAICompatibleAdapter()
	sensitiveDefaultImg := "https://example.com/default.png"

	t.Run("normal response", func(t *testing.T) {
		mockModeration := mock_component.NewMockModeration(t)
		mockModeration.EXPECT().CheckImage(mock.Anything, mock.Anything).Return(&rpc.CheckResult{IsSensitive: false}, nil)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)

		imageGen := NewImageGeneration(ctx.Writer, adapter, mockModeration, sensitiveDefaultImg, nil, "", nil, "")
		imageGen.WriteHeader(http.StatusOK)
		responseData := []byte(`{"created":1625625600,"data":[{"url":"https://example.com/image1.png"}]}`)
		_, err := imageGen.Write(responseData)
		require.NoError(t, err)
		err = imageGen.Finalize()
		require.NoError(t, err)

		require.JSONEq(t, string(responseData), w.Body.String())
	})

	t.Run("sensitive response", func(t *testing.T) {
		mockModeration := mock_component.NewMockModeration(t)
		mockModeration.EXPECT().CheckImage(mock.Anything, mock.Anything).Return(&rpc.CheckResult{IsSensitive: true, Reason: "sensitive content"}, nil)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)

		imageGen := NewImageGeneration(ctx.Writer, adapter, mockModeration, sensitiveDefaultImg, nil, "", nil, "")
		imageGen.WriteHeader(http.StatusOK)
		responseData := []byte(`{"created":1625625600,"data":[{"url":"https://example.com/sensitive.png"}]}`)
		_, err := imageGen.Write(responseData)
		require.NoError(t, err)
		err = imageGen.Finalize()
		require.NoError(t, err)

		expectedResponse := types.ImageGenerationResponse{
			ImagesResponse: openai.ImagesResponse{
				Created: 1625625600,
				Data:    []openai.Image{{URL: sensitiveDefaultImg}},
			},
		}
		expectedData, _ := json.Marshal(expectedResponse)
		require.JSONEq(t, string(expectedData), w.Body.String())
	})

	t.Run("moderation service error", func(t *testing.T) {
		mockModeration := mock_component.NewMockModeration(t)
		mockModeration.EXPECT().CheckImage(mock.Anything, mock.Anything).Return(nil, errors.New("moderation service error"))

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)

		imageGen := NewImageGeneration(ctx.Writer, adapter, mockModeration, sensitiveDefaultImg, nil, "", nil, "")
		imageGen.WriteHeader(http.StatusOK)
		responseData := []byte(`{"created":1625625600,"data":[{"url":"https://example.com/image1.png"}]}`)
		_, err := imageGen.Write(responseData)
		require.NoError(t, err)
		err = imageGen.Finalize()
		require.NoError(t, err)

		require.JSONEq(t, string(responseData), w.Body.String())
	})

	t.Run("feeds image counter when usage present", func(t *testing.T) {
		mockModeration := mock_component.NewMockModeration(t)
		mockModeration.EXPECT().CheckImage(mock.Anything, mock.Anything).Return(&rpc.CheckResult{IsSensitive: false}, nil)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		counter := token.NewImageUsageCounter()

		imageGen := NewImageGeneration(ctx.Writer, adapter, mockModeration, sensitiveDefaultImg, counter, "", nil, "")
		imageGen.WriteHeader(http.StatusOK)
		responseData := []byte(`{"created":1625625600,"data":[{"url":"https://example.com/img.png"}],"usage":{"total_tokens":10,"input_tokens":2,"output_tokens":8}}`)
		_, err := imageGen.Write(responseData)
		require.NoError(t, err)
		err = imageGen.Finalize()
		require.NoError(t, err)

		usage, err := counter.Usage(context.Background())
		require.NoError(t, err)
		require.Equal(t, int64(2), usage.PromptTokens)
		require.Equal(t, int64(8), usage.CompletionTokens)
		require.Equal(t, int64(10), usage.TotalTokens)
	})
}
