package inference

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"time"
)

type ModelID struct {
	Owner, Name, Version string
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

var _ App = (*llmInferClient)(nil)

type ModelInfo struct {
	Endpoint string
	// deploy,running,failed etc
	Status string
	// ModelID.Hash()
	HashID uint64
}

type llmInferClient struct {
	lastUpdate    time.Time
	hc            *http.Client
	modelServices map[uint64]ModelInfo
	serverAddr    string
}

func NewInferClient(addr string) App {
	hc := http.DefaultClient
	hc.Timeout = 5 * time.Second
	return &llmInferClient{
		hc:         hc,
		serverAddr: addr,
	}
}

func (c *llmInferClient) Predict(id ModelID, req *PredictRequest) (*PredictResponse, error) {
	s, err := c.GetModelService(id)
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

func (c *llmInferClient) ServingList() (map[uint64]ModelInfo, error) {
	// use local cache first
	if time.Since(c.lastUpdate).Seconds() < 30 {
		return c.modelServices, nil
	}

	tmp := make(map[uint64]ModelInfo)
	// TODO:call inference service to ge all serving models
	// c.hc.Post()
	testModelID := ModelID{
		Owner:   "test_user_name",
		Name:    "test_model_name",
		Version: "",
	}
	tmp[testModelID.Hash()] = ModelInfo{
		HashID:   testModelID.Hash(),
		Endpoint: "http://localhost:8080/test_user_name/test_model_name",
		Status:   "running",
	}

	c.modelServices = tmp
	c.lastUpdate = time.Now()
	return c.modelServices, nil
}

func (c *llmInferClient) GetModelService(id ModelID) (ModelInfo, error) {
	list, err := c.ServingList()
	if err != nil {
		return ModelInfo{}, err
	}

	if s, ok := list[id.Hash()]; ok {
		return s, nil
	}

	return ModelInfo{}, errors.New("model service not found by id")
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
