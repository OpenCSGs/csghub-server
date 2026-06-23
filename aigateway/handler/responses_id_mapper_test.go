package handler

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
)

func TestResponsesIDMapperWrapUnwrap(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	id, err := mapper.Wrap(responsesIDClaims{
		UpstreamResponseID: "resp_openai_123",
		UpstreamID:         42,
		NamespaceUUID:      "namespace-1",
	})
	require.NoError(t, err)
	require.Contains(t, id, "resp_agw_v1.")

	claims, err := mapper.Unwrap(id, "namespace-1")
	require.NoError(t, err)
	require.Equal(t, "resp_openai_123", claims.UpstreamResponseID)
	require.Equal(t, int64(42), claims.UpstreamID)
	require.Equal(t, "namespace-1", claims.NamespaceUUID)
}

func TestResponsesIDMapperRejectsOtherOwner(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	id, err := mapper.Wrap(responsesIDClaims{UpstreamResponseID: "resp_1", UpstreamID: 1, NamespaceUUID: "namespace-1"})
	require.NoError(t, err)

	_, err = mapper.Unwrap(id, "namespace-2")
	require.ErrorIs(t, err, errResponseIDOwner)
}

func TestResponsesIDMapperRequiresUpstreamID(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	_, err = mapper.Wrap(responsesIDClaims{UpstreamResponseID: "resp_1", NamespaceUUID: "namespace-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "upstream id is empty")
}

func TestResponsesIDMapperFromConfigUsesResponsesSecret(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.ResponsesIDSecret = "responses-secret"
	mapper, err := newResponsesIDMapperFromConfig(cfg)
	require.NoError(t, err)

	id, err := mapper.Wrap(responsesIDClaims{UpstreamResponseID: "resp_1", UpstreamID: 1, NamespaceUUID: "namespace-1"})
	require.NoError(t, err)
	claims, err := mapper.Unwrap(id, "namespace-1")
	require.NoError(t, err)
	require.Equal(t, "resp_1", claims.UpstreamResponseID)
}

func TestResponsesIDMapperFromConfigDoesNotFallbackToSpaceOrJWTSecrets(t *testing.T) {
	cfg := &config.Config{}
	cfg.Space.SessionSecretKey = "space-secret"
	cfg.JWT.SigningKey = "jwt-secret"

	_, err := newResponsesIDMapperFromConfig(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "responses id mapper secret is empty")
}

func TestGetResponsesIDMapperConcurrentInit(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.ResponsesIDSecret = "responses-secret"
	h := &OpenAIHandlerImpl{config: cfg}

	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	mappers := make(chan *ResponsesIDMapper, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mapper, err := h.getResponsesIDMapper()
			errs <- err
			mappers <- mapper
		}()
	}
	wg.Wait()
	close(errs)
	close(mappers)

	for err := range errs {
		require.NoError(t, err)
	}
	var first *ResponsesIDMapper
	for mapper := range mappers {
		require.NotNil(t, mapper)
		if first == nil {
			first = mapper
			continue
		}
		require.Same(t, first, mapper)
	}
}
