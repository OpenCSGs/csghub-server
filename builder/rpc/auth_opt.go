package rpc

import "net/http"

type RequestOption interface {
	Set(req *http.Request)
}

type authWithApiKey struct {
	apiKey string
}

type withJSONHeader struct{}

func AuthWithApiKey(apiKey string) RequestOption {
	return authWithApiKey{apiKey: apiKey}
}

func (a authWithApiKey) Set(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
}

func WithJSONHeader() RequestOption {
	return withJSONHeader{}
}

func (w withJSONHeader) Set(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
}
