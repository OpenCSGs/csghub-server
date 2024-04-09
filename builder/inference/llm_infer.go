package inference

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ModelID struct {
	Owner, Name string
	// reserved, keep empty string ""
	Version string
}

func (m ModelID) Hash() uint64 {
	f := fnv.New64()
	f.Write([]byte(m.Owner))
	f.Write([]byte(":"))
	f.Write([]byte(m.Name))
	f.Write([]byte(":"))
	f.Write([]byte(m.Version))
	return f.Sum64()
}

var _ Client = (*llmInferClient)(nil)

type ModelInfo struct {
	Endpoint string
	// deploy,running,failed etc
	Status string
	// ModelID.Hash()
	HashID uint64
}

type llmInferClient struct {
	lastUpdate time.Time
	hc         *http.Client
	modelInfos map[uint64]ModelInfo
	serverAddr string
}

func NewInferClient(addr string) Client {
	hc := http.DefaultClient
	hc.Timeout = time.Minute
	return &llmInferClient{
		hc:         hc,
		modelInfos: make(map[uint64]ModelInfo),
		serverAddr: addr,
	}
}

func (c *llmInferClient) Predict(id ModelID, req *PredictRequest) (*PredictResponse, error) {
	s, err := c.GetModelInfo(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get model info,error:%w", err)
	}

	{
		// for test only, as inference service is not ready
		if id.Owner == "test_user_name" && id.Name == "test_model_name" {
			return &PredictResponse{GeneratedText: "this is a test predict result."}, nil
		}
	}
	return c.CallPredict(s.Endpoint, req)
}

// ListServing call inference service to ge all serving models
func (c *llmInferClient) ListServing() (map[uint64]ModelInfo, error) {
	defer func() {
		// for test only
		testModelID := ModelID{
			Owner:   "test_user_name",
			Name:    "test_model_name",
			Version: "",
		}
		c.modelInfos[testModelID.Hash()] = ModelInfo{
			HashID:   testModelID.Hash(),
			Endpoint: "http://localhost:8080/test_user_name/test_model_name",
			Status:   "running",
		}
	}()

	// use local cache first
	if expire := time.Since(c.lastUpdate).Seconds(); expire < 30 {
		slog.Info("use cached model infos", slog.Float64("expire", expire))
		return c.modelInfos, nil
	}

	api, _ := url.JoinPath(c.serverAddr, "/api/list_serving")
	req, _ := http.NewRequest(http.MethodGet, api, nil)
	req.Header.Set("user-name", "default")
	resp, err := c.hc.Do(req)
	if err != nil {
		slog.Error("fail to call list serving api", slog.Any("err", err))
		return c.modelInfos, fmt.Errorf("fail to call list serving api,%w", err)
	}
	llmInfos := make(map[string]LlmModelInfo)
	err = json.NewDecoder(resp.Body).Decode(&llmInfos)
	if err != nil {
		slog.Error("fail to decode list serving response", slog.Any("err", err))
		return c.modelInfos, fmt.Errorf("fail to decode list serving response,%w", err)
	}

	slog.Debug("llmResp", slog.Any("map", llmInfos))
	if len(llmInfos) > 0 {
		c.updateModelInfos(llmInfos)
	}
	return c.modelInfos, nil
}

func (c *llmInferClient) updateModelInfos(llmInfos map[string]LlmModelInfo) {
	tmp := make(map[uint64]ModelInfo)
	for _, v := range llmInfos {
		for modelName, endpoint := range v.URL {
			// example: THUDM/chatglm3-6b
			owner, name, _ := strings.Cut(modelName, "/")
			mid := ModelID{
				Owner:   owner,
				Name:    name,
				Version: "",
			}
			slog.Debug("get model info", slog.Any("mid", mid), slog.String("endpoint", endpoint))
			// endpoint = strings.Replace(endpoint, "http://0.0.0.0:8000", c.serverAddr, 1)
			parsedUrl, _ := url.Parse(endpoint)
			endpoint, _ = url.JoinPath(c.serverAddr, parsedUrl.RequestURI())
			slog.Debug("replace llm endpoint with new domain", slog.String("new_endpoint", endpoint))
			var status string
			if len(v.Status) > 0 {
				for _, vs := range v.Status {
					status = vs.ApplicationStatus
					break
				}
			}
			mi := ModelInfo{
				Endpoint: endpoint,
				Status:   status,
				HashID:   mid.Hash(),
			}
			tmp[mi.HashID] = mi
			// only one url
			break
		}
	}
	c.modelInfos = tmp
	c.lastUpdate = time.Now()
}

func (c *llmInferClient) GetModelInfo(id ModelID) (ModelInfo, error) {
	list, err := c.ListServing()
	if err != nil {
		return ModelInfo{}, err
	}

	if s, ok := list[id.Hash()]; ok {
		return s, nil
	}

	return ModelInfo{}, errors.New("model info not found by id")
}

func (c *llmInferClient) CallPredict(url string, req *PredictRequest) (*PredictResponse, error) {
	var body bytes.Buffer
	json.NewEncoder(&body).Encode(req)
	resp, err := c.hc.Post(url, "application/json", &body)
	if err != nil {
		return nil, fmt.Errorf("failed to send http request,error: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body,error: %w", err)
	}

	var r PredictResponse
	err = json.Unmarshal(data, &r)
	return &r, err
}
