package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/common/config"
)

const (
	responsesGatewayIDPrefix = "resp_agw_v1"
	responsesAdapterIDPrefix = "resp_agw_adapter"
)

var (
	errInvalidResponseID = errors.New("invalid response id")
	errResponseIDOwner   = errors.New("response id owner mismatch")
)

type responsesIDClaims struct {
	NamespaceUUID      string `json:"namespace_uuid"`
	UpstreamID         int64  `json:"upstream_id"`
	UpstreamResponseID string `json:"upstream_response_id"`
}

type ResponsesIDMapper struct {
	aead cipher.AEAD
}

func newResponsesIDMapper(secret string) (*ResponsesIDMapper, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("responses id mapper secret is empty")
	}
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &ResponsesIDMapper{aead: aead}, nil
}

func newResponsesIDMapperFromConfig(cfg *config.Config) (*ResponsesIDMapper, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return newResponsesIDMapper(cfg.AIGateway.ResponsesIDSecret)
}

func (m *ResponsesIDMapper) Wrap(claims responsesIDClaims) (string, error) {
	if m == nil || m.aead == nil {
		return "", fmt.Errorf("responses id mapper is not configured")
	}
	if claims.UpstreamResponseID == "" {
		return "", fmt.Errorf("upstream response id is empty")
	}
	if claims.NamespaceUUID == "" {
		return "", fmt.Errorf("namespace uuid is empty")
	}
	if claims.UpstreamID == 0 {
		return "", fmt.Errorf("upstream id is empty")
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// Bind the token prefix as AEAD associated data so ciphertext from another
	// gateway token family cannot be replayed as a Responses ID. Tenant isolation
	// is enforced by the encrypted namespace_uuid claim during Unwrap.
	ciphertext := m.aead.Seal(nil, nonce, payload, []byte(responsesGatewayIDPrefix))
	token := append(nonce, ciphertext...)
	return responsesGatewayIDPrefix + "." + base64.RawURLEncoding.EncodeToString(token), nil
}

func (m *ResponsesIDMapper) Unwrap(id, owner string) (responsesIDClaims, error) {
	var claims responsesIDClaims
	if m == nil || m.aead == nil {
		return claims, fmt.Errorf("responses id mapper is not configured")
	}
	parts := strings.Split(id, ".")
	if len(parts) != 2 || parts[0] != responsesGatewayIDPrefix {
		return claims, errInvalidResponseID
	}
	token, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errInvalidResponseID
	}
	nonceSize := m.aead.NonceSize()
	if len(token) <= nonceSize {
		return claims, errInvalidResponseID
	}
	payload, err := m.aead.Open(nil, token[:nonceSize], token[nonceSize:], []byte(responsesGatewayIDPrefix))
	if err != nil {
		return claims, errInvalidResponseID
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, errInvalidResponseID
	}
	if claims.NamespaceUUID != owner {
		return claims, errResponseIDOwner
	}
	return claims, nil
}

func isAdapterResponseID(id string) bool {
	return strings.HasPrefix(id, responsesAdapterIDPrefix+"_")
}

func newAdapterResponseID() string {
	return responsesAdapterIDPrefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
