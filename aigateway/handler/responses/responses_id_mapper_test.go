package responses

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
)

func TestResponsesIDMapperWrapUnwrap(t *testing.T) {
	mapper, err := NewIDMapper("test-secret")
	require.NoError(t, err)

	id, err := mapper.Wrap(IDClaims{
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
	mapper, err := NewIDMapper("test-secret")
	require.NoError(t, err)
	id, err := mapper.Wrap(IDClaims{UpstreamResponseID: "resp_1", UpstreamID: 1, NamespaceUUID: "namespace-1"})
	require.NoError(t, err)

	_, err = mapper.Unwrap(id, "namespace-2")
	require.ErrorIs(t, err, ErrResponseIDOwner)
}

func TestResponsesIDMapperRequiresUpstreamID(t *testing.T) {
	mapper, err := NewIDMapper("test-secret")
	require.NoError(t, err)

	_, err = mapper.Wrap(IDClaims{UpstreamResponseID: "resp_1", NamespaceUUID: "namespace-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "upstream id is empty")
}

func TestResponsesIDMapperFromConfigUsesResponsesSecret(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.ResponsesIDSecret = "responses-secret"
	mapper, err := NewIDMapperFromConfig(cfg)
	require.NoError(t, err)

	id, err := mapper.Wrap(IDClaims{UpstreamResponseID: "resp_1", UpstreamID: 1, NamespaceUUID: "namespace-1"})
	require.NoError(t, err)
	claims, err := mapper.Unwrap(id, "namespace-1")
	require.NoError(t, err)
	require.Equal(t, "resp_1", claims.UpstreamResponseID)
}

func TestResponsesIDMapperFromConfigDoesNotFallbackToSpaceOrJWTSecrets(t *testing.T) {
	cfg := &config.Config{}
	cfg.Space.SessionSecretKey = "space-secret"
	cfg.JWT.SigningKey = "jwt-secret"

	_, err := NewIDMapperFromConfig(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "responses id mapper secret is empty")
}
