//go:build !ee && !saas

package rpc

import (
	"context"
	"net/url"
	"time"

	"opencsg.com/csghub-server/common/types"
)

type XnetSvcHttpClient struct {
	hc *HttpClient
}

func NewXnetSvcHttpClient(endpoint string, opts ...RequestOption) XnetSvcClient {
	return &XnetSvcHttpClient{
		hc: NewHttpClient(endpoint, opts...),
	}
}

func (c *XnetSvcHttpClient) GenerateWriteToken(ctx context.Context, req *types.XnetTokenReq) (*types.XnetTokenResp, error) {
	return nil, nil
}

func (c *XnetSvcHttpClient) PresignedGetObject(ctx context.Context, objectKey string, expireTime time.Duration, params url.Values) (*url.URL, error) {
	return nil, nil
}

func (c *XnetSvcHttpClient) FileExists(ctx context.Context, objectKey string) (bool, error) {
	return false, nil
}
