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

/*
func (r *ReverseProxy) WithModeration(modSvcClient rpc.ModerationSvcClient) *ReverseProxy {
	r.modSvcClient = modSvcClient
	return r
}*/

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, api string) {
	// do request body content moderation before proxy it to backend service
	// if rp.modSvcClient != nil {
	// 	result, err := rp.modRequest(r)
	// 	if err != nil {
	// 		slog.Error("failed to mod request body", slog.Any("error", err), slog.String("url", r.RequestURI))

	// 		w.WriteHeader(http.StatusInternalServerError)
	// 		_, _ = w.Write([]byte(err.Error()))
	// 		return
	// 	}
	// 	if result.IsSensitive {
	// 		w.WriteHeader(http.StatusBadRequest)
	// 		_, _ = w.Write([]byte("sensitive content detected in request body:" + result.Reason))
	// 		return
	// 	}

	// }
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

		// do response body content moderation before sending it to client
		// if rp.modSvcClient != nil && rp.isTextContent(resp.Header) {
		// 	result, err := rp.modResponse(resp)
		// 	if err != nil {
		// 		//clean up the response body
		// 		bodyContent, _ := io.ReadAll(resp.Body)
		// 		slog.Error("failed to mod response body", slog.Any("error", err), slog.String("body", string(bodyContent)))

		// 		w.WriteHeader(http.StatusInternalServerError)
		// 		_, _ = w.Write([]byte(err.Error()))
		// 		return nil
		// 	}
		// 	if result.IsSensitive {
		// 		//clean up the response body
		// 		bodyContent, _ := io.ReadAll(resp.Body)
		// 		slog.Info("sensitive content detected in response body:", slog.String("body", string(bodyContent)))

		return nil
	}
	proxy.ServeHTTP(w, r)
}

// func (rp *ReverseProxy) modRequest(r *http.Request) (*rpc.CheckResult, error) {
// 	// dump request body
// 	var buf bytes.Buffer
// 	teeReader := io.TeeReader(r.Body, &buf)
// 	bodyContent, err := io.ReadAll(teeReader)
// 	r.Body = io.NopCloser(&buf)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read request body: %w", err)
// 	}

// 	return rp.modText(string(bodyContent))
// }

// // modResponse reads the response body, checks if it contains any sensitive content using the
// // moderation service provided.
// //
// // If the content is sensitive,it will return the sensitive result. Otherwise, it will return nil.
// func (rp *ReverseProxy) modResponse(r *http.Response) (*rpc.CheckResult, error) {
// 	// dump response body
// 	var buf bytes.Buffer
// 	teeReader := io.TeeReader(r.Body, &buf)
// 	bodyContent, err := io.ReadAll(teeReader)
// 	r.Body = io.NopCloser(&buf)

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read response body: %w", err)
// 	}

// 	return rp.modText(string(bodyContent))
// }

// func (rp *ReverseProxy) modText(text string) (*rpc.CheckResult, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
// 	result, err := rp.modSvcClient.PassTextCheck(ctx, string(sensitive.ScenarioCommentDetection), text)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to call moderation service to check content sensitive: %w", err)
// 	}
// 	return result, nil
// }

// func (rp *ReverseProxy) isTextContent(header http.Header) bool {
// 	contentType := header.Get("Content-Type")
// 	if contentType == "" {
// 		return false
// 	}

// 	//TODO: support event stream in the future
// 	if strings.HasPrefix(contentType, "text/event-stream") {
// 		return false
// 	}

// 	contentType = strings.ToLower(contentType)
// 	for _, t := range textContentTypes {
// 		if strings.HasPrefix(contentType, t) {
// 			return true
// 		}
// 	}

// 	return false
// }
/*
func (rp *ReverseProxy) isTextContentSSE(header http.Header) bool {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return false
	}

	//TODO: support event stream in the future
	if strings.HasPrefix(contentType, "text/event-stream") {
		return true
	}

	return false
}
*/
