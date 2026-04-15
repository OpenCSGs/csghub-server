package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/common/utils/trace"
)

type ReverseProxy interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request, api, svcHost string)
}

type reverseProxyImpl struct {
	target         *url.URL
	acceptEncoding *string
}

var DefaultResponseStreamContentType openai.ChatCompletionChunk

type ReverseProxyOption func(*reverseProxyImpl)

func WithAcceptEncoding(encoding string) ReverseProxyOption {
	return func(rp *reverseProxyImpl) {
		rp.acceptEncoding = &encoding
	}
}

func WithoutAcceptEncoding() ReverseProxyOption {
	return WithAcceptEncoding("identity")
}

func NewReverseProxy(target string, opts ...ReverseProxyOption) (ReverseProxy, error) {
	url, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	rp := &reverseProxyImpl{
		target: url,
	}
	for _, opt := range opts {
		opt(rp)
	}
	return rp, nil
}

func (rp *reverseProxyImpl) ServeHTTP(w http.ResponseWriter, r *http.Request, api, svcHost string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("Connection to target server interrupted", slog.Any("error", r))
		}
	}()
	proxy := httputil.NewSingleHostReverseProxy(rp.target)
	proxy.Director = func(req *http.Request) {
		if len(svcHost) > 0 {
			slog.Info("update reverse proxy header host", slog.Any("svc-host", svcHost))
			req.Host = svcHost
		} else {
			req.Host = rp.target.Host
		}
		req.URL.Host = rp.target.Host
		req.URL.Scheme = rp.target.Scheme
		if len(api) > 0 {
			// change url to given api
			req.URL.Path = api
		}

		targetQuery := rp.target.RawQuery
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		// dont support br comporession
		if rp.acceptEncoding == nil {
			req.Header.Set("Accept-Encoding", "gzip")
		} else if *rp.acceptEncoding == "" {
			req.Header.Del("Accept-Encoding")
		} else {
			req.Header.Set("Accept-Encoding", *rp.acceptEncoding)
		}

		// debug only
		// {
		// 	slog.Info("request of redirector", slog.Any("req", *req.URL))
		// 	data, _ := httputil.DumpRequestOut(req, false)
		// 	fmt.Println(string(data))
		// }
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		// data, err := httputil.DumpResponse(r, true)
		// fmt.Println(string(data))
		// remove duplicated header
		resp.Header.Del("Access-Control-Allow-Credentials")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Allow-Origin")
		// remove duplicate X-Request-Id header from downstream response
		// because it is already set by the gateway middleware
		resp.Header.Del(trace.HeaderRequestID)
		// allow upstream pages to be embedded in iframes by the parent app
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")

		return nil
	}
	proxy.ServeHTTP(w, r)
}
