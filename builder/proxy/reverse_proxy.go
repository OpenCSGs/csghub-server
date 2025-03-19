package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/openai/openai-go"
)

type ReverseProxy struct {
	target *url.URL
}

var DefaultResponseStreamContentType openai.ChatCompletionChunk

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
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("Connection to target server interrupted", slog.Any("error", r))
		}
	}()
	proxy := httputil.NewSingleHostReverseProxy(rp.target)
	proxy.Director = func(req *http.Request) {
		req.Host = rp.target.Host
		req.URL.Host = rp.target.Host
		req.URL.Scheme = rp.target.Scheme
		if len(api) > 0 {
			// change url to given api
			req.URL.Path = api
		}
		// dont support br comporession
		req.Header.Set("Accept-Encoding", "gzip")

		// debug only
		// {
		// 	slog.Info("request of redirector", slog.Any("req", *req.URL))
		// 	data, _ := httputil.DumpRequestOut(req, false)
		// 	fmt.Println(string(data))
		// }
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		// data, err := httputil.DumpResponse(r, true)
		// fmt.Println(string(data))
		// remove duplicated header
		resp.Header.Del("Access-Control-Allow-Credentials")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Allow-Origin")

		return nil
	}
	proxy.ServeHTTP(w, r)
}
