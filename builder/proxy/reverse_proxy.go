package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type ReverseProxy struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
}

func NewReverseProxy(target string) (*ReverseProxy, error) {
	url, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return &ReverseProxy{
		target: url,
	}, nil
}

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, api string) {
	proxy := httputil.NewSingleHostReverseProxy(rp.target)
	proxy.Director = func(req *http.Request) {
		req.Host = rp.target.Host
		req.URL.Host = rp.target.Host
		req.URL.Scheme = rp.target.Scheme
		// change url to space api
		req.URL.Path = api

		// debug only
		// {
		// 	slog.Info("request of redirector", slog.Any("req", *req.URL))
		// 	data, _ := httputil.DumpRequestOut(req, false)
		// 	fmt.Println(string(data))
		// }
	}
	// for debug
	// proxy.ModifyResponse = func(r *http.Response) error {
	// 	data, err := httputil.DumpResponse(r, true)
	// 	fmt.Println(string(data))
	// 	return nil
	// }
	proxy.ServeHTTP(w, r)
}
