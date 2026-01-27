//go:build !ee && !saas

package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestXnetSvcHttpClient_GetMigrationStats_CE(t *testing.T) {
	endpoint := "http://xnet-service"
	client := NewXnetSvcHttpClient(endpoint)

	resp, err := client.GetMigrationStats(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func TestXnetSvcHttpClient_OtherMethods_CE(t *testing.T) {
	endpoint := "http://xnet-service"
	client := NewXnetSvcHttpClient(endpoint)

	respToken, errToken := client.GenerateWriteToken(context.Background(), nil)
	assert.NoError(t, errToken)
	assert.Nil(t, respToken)

	respURL, errURL := client.PresignedGetObject(context.Background(), "", 0, nil)
	assert.NoError(t, errURL)
	assert.Nil(t, respURL)

	exists, errExists := client.FileExists(context.Background(), nil)
	assert.NoError(t, errExists)
	assert.False(t, exists)
}
