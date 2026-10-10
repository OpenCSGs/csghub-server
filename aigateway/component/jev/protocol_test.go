package jev

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestValidateSuccessResponse(t *testing.T) {
	valid := &types.JevResponse{
		Answers: map[string]json.RawMessage{"refund": json.RawMessage(`{"type":"noul"}`)},
		Usage:   &types.JevUsage{InputTokens: 1, OutputTokens: 2},
	}
	assert.NoError(t, ValidateSuccessResponse(valid))

	// An empty answers map satisfies the protocol; nil/absent answers and a
	// missing or null usage block do not.
	emptyAnswers := &types.JevResponse{Answers: map[string]json.RawMessage{}, Usage: &types.JevUsage{}}
	assert.NoError(t, ValidateSuccessResponse(emptyAnswers))

	nilAnswers := &types.JevResponse{Usage: &types.JevUsage{}}
	assert.ErrorIs(t, ValidateSuccessResponse(nilAnswers), ErrMissingRequiredFields)
	missingUsage := &types.JevResponse{Answers: map[string]json.RawMessage{}}
	assert.ErrorIs(t, ValidateSuccessResponse(missingUsage), ErrMissingRequiredFields)
	assert.ErrorIs(t, ValidateSuccessResponse(nil), ErrMissingRequiredFields)
}

func TestExtractUpstreamErrorMessage(t *testing.T) {
	assert.Equal(t, "invalid questions", ExtractUpstreamErrorMessage([]byte(`{"error":{"message":"invalid questions"}}`)))
	assert.Equal(t, "model not found", ExtractUpstreamErrorMessage([]byte(`{"error":"model not found"}`)))
	assert.Equal(t, "rate limited", ExtractUpstreamErrorMessage([]byte(`{"message":"rate limited"}`)))
	assert.Equal(t, "oops", ExtractUpstreamErrorMessage([]byte(`oops`)))
	assert.Equal(t, "upstream returned an error with no response body", ExtractUpstreamErrorMessage(nil))

	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	assert.Len(t, ExtractUpstreamErrorMessage(long), 512)
}
