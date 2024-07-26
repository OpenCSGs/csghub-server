package rpc

import "net/http"

type RequestOption interface {
	Set(req *http.Request)
}

type authWithApiKey struct {
	apiKey string
}

func AuthWithApiKey(apiKey string) RequestOption {
	return authWithApiKey{apiKey: apiKey}
}

func (a authWithApiKey) Set(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
}
