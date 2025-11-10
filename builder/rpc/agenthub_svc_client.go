package rpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AgentHubSvcClient interface {
	CreateAgentInstance(ctx context.Context, userUUID string, req *CreateAgentInstanceRequest) (*CreateAgentInstanceResponse, error)
	DeleteAgentInstance(ctx context.Context, userUUID string, contentID string) error
	GetAgentInstances(ctx context.Context, req *GetAgentInstancesRequest) (GetAgentInstancesResponse, error)
	RunAgentInstance(ctx context.Context, userUUID string, contentID string, req *RunAgentInstanceRequest) (*RunAgentInstanceResponse, error)
	RunAgentInstanceStream(ctx context.Context, userUUID string, contentID string, req *RunAgentInstanceRequest) (<-chan types.AgentStreamEvent, error)
}

type CreateAgentInstanceRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
}

type CreateAgentInstanceResponse AgentInstance

type GetAgentInstancesRequest struct {
	IDs      []string `json:"ids"`
	UserUUID string   `json:"user_uuid"`
}

type GetAgentInstancesResponse []*AgentInstance

type DeleteAgentInstanceRequest struct {
	IDs []string `json:"ids"`
}

type DeleteAgentInstanceResponse struct {
	IDs   []string `json:"ids"`
	Total int      `json:"total"`
}

type RunAgentInstanceRequest struct {
	InputValue string          `json:"input_value"`
	InputType  string          `json:"input_type"`
	OutputType string          `json:"output_type"`
	Tweaks     json.RawMessage `json:"tweaks"`
	SessionID  string          `json:"session_id"`
	Stream     bool            `json:"stream"`
}

type RunAgentInstanceResponse struct {
	SessionID string    `json:"session_id"`
	Outputs   []Outputs `json:"outputs"`
}

type Outputs struct {
	Outputs []InnerOutput `json:"outputs,omitempty"` // camada extra para stream=true
	Results *Results      `json:"results,omitempty"` // stream=false
}

type InnerOutput struct {
	Results *Results `json:"results,omitempty"`
}

type Results struct {
	Message Message `json:"message"`
}

type Message struct {
	Timestamp  string `json:"timestamp"`
	Sender     string `json:"sender"`
	SenderName string `json:"sender_name"`
	TextKey    string `json:"text_key"`
	Text       string `json:"text"`
}

// TokenData holds the data for an event of type "token".
type TokenData struct {
	Chunk string `json:"chunk"`
}

// EndData holds the final result from an event of type "end".
// The 'Result' field conveniently matches our target RunAgentInstanceResponse struct.
type EndData struct {
	Result RunAgentInstanceResponse `json:"result"`
}

/*
	{
	  "id": "new-flow-id",
	  "name": "New Flow Name",
	  "description": "Flow description",
	  "data": {
	    // Flow graph data
	  },
	  "hasIO": true,
	  "created_at": "2024-01-01T00:00:00Z",
	  "updated_at": "2024-01-01T00:00:00Z",
	  "user_id": "user-uuid",
	  "folder_id": "folder-uuid"
	}
*/
type AgentInstance struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
	HasIO       bool            `json:"hasIO"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	UserUUID    string          `json:"user_id"` // user uuid
	FolderID    string          `json:"folder_id"`
}

type AgentHubSvcClientImpl struct {
	hc    *HttpClient
	token string
}

func NewAgentHubSvcClientImpl(endpoint string, token string, opts ...RequestOption) AgentHubSvcClient {
	return &AgentHubSvcClientImpl{
		hc:    NewHttpClient(strings.TrimSuffix(endpoint, "/"), opts...),
		token: token,
	}
}

// POST /api/v1/opencsg/flows/
func (c *AgentHubSvcClientImpl) CreateAgentInstance(ctx context.Context, userUUID string, req *CreateAgentInstanceRequest) (*CreateAgentInstanceResponse, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("create agent instance request is nil"), nil)
	}
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "agenthub",
		"api":       "/api/v1/opencsg/flows/",
	}
	var resp CreateAgentInstanceResponse
	var buf io.Reader

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf = bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/opencsg/flows/?token=" + c.token
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return nil, errorx.RemoteSvcFail(errors.New("failed to create agent instance in agenthub"), rpcErrorCtx)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		return nil, errorx.RemoteSvcFail(errors.New("failed to create agent instance in agenthub"), rpcErrorCtx)
	}

	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	return &resp, nil
}

// POST /api/v1/opencsg/flows/query
func (c *AgentHubSvcClientImpl) GetAgentInstances(ctx context.Context, req *GetAgentInstancesRequest) (GetAgentInstancesResponse, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("get agent instances request is nil"), nil)
	}
	rpcErrorCtx := map[string]any{
		"user_uuid": req.UserUUID,
		"service":   "agenthub",
		"api":       "/api/v1/opencsg/flows/query",
	}

	var resp GetAgentInstancesResponse
	var buf io.Reader
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf = bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/opencsg/flows/query?token=" + c.token
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", req.UserUUID)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return nil, errorx.RemoteSvcFail(errors.New("failed to get agent instance from agenthub"), rpcErrorCtx)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		return nil, errorx.RemoteSvcFail(errors.New("failed to get agent instance from agenthub"), rpcErrorCtx)
	}
	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	return resp, nil
}

// POST /api/v1/opencsg/flows/delete
func (c *AgentHubSvcClientImpl) DeleteAgentInstance(ctx context.Context, userUUID string, contentID string) error {
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "agenthub",
		"api":       "/api/v1/opencsg/flows/delete",
	}
	var resp DeleteAgentInstanceResponse

	req := DeleteAgentInstanceRequest{
		IDs: []string{contentID},
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf := bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/opencsg/flows/delete?token=" + c.token
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}

	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return errorx.RemoteSvcFail(errors.New("failed to delete agent instance in agenthub"), rpcErrorCtx)
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		return errorx.RemoteSvcFail(errors.New("failed to delete agent instance in agenthub, status code: "+strconv.Itoa(hresp.StatusCode)), rpcErrorCtx)
	}

	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return errorx.RemoteSvcFail(errors.New("failed to delete agent instance in agenthub, unmarshal response error: "+err.Error()), rpcErrorCtx)
	}

	if resp.Total != 1 {
		return errorx.RemoteSvcFail(errors.New("failed to delete agent instance in agenthub, total: "+strconv.Itoa(resp.Total)), rpcErrorCtx)
	}

	if len(resp.IDs) == 0 {
		return errorx.RemoteSvcFail(errors.New("failed to delete agent instance in agenthub, response IDs is empty"), rpcErrorCtx)
	}

	if resp.IDs[0] != contentID {
		return errorx.RemoteSvcFail(errors.New("failed to delete agent instance in agenthub, content ID mismatch: "+contentID+" != "+resp.IDs[0]), rpcErrorCtx)
	}
	return nil
}

// POST /api/v1/opencsg/run/{id}?stream=false
func (c *AgentHubSvcClientImpl) RunAgentInstance(ctx context.Context, userUUID string, instanceID string, req *RunAgentInstanceRequest) (*RunAgentInstanceResponse, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("run agent instance request is nil"), nil)
	}
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "agenthub",
		"api":       "/api/v1/opencsg/run/" + instanceID + "?stream=false",
	}
	var buf io.Reader
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf = bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/opencsg/run/" + instanceID + "?stream=false&token=" + c.token
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return nil, errorx.RemoteSvcFail(errors.New("failed to run agent instance in agenthub"), rpcErrorCtx)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		return nil, errorx.RemoteSvcFail(errors.New("failed to run agent instance in agenthub"), rpcErrorCtx)
	}

	// handle non-stream response
	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	var resp RunAgentInstanceResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	return &resp, nil
}

// RunAgentInstanceStream runs an agent instance and returns a streaming channel
// POST /api/v1/opencsg/run/{id}?stream=true
func (c *AgentHubSvcClientImpl) RunAgentInstanceStream(ctx context.Context, userUUID string, contentID string, req *RunAgentInstanceRequest) (<-chan types.AgentStreamEvent, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("run agent instance request is nil"), nil)
	}

	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "agenthub",
		"api":       "/api/v1/opencsg/run/" + contentID + "?stream=true",
	}

	var buf io.Reader
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf = bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/opencsg/run/" + contentID + "?stream=true&token=" + c.token
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return nil, errorx.RemoteSvcFail(errors.New("failed to run agent instance in agenthub"), rpcErrorCtx)
	}

	if hresp.StatusCode != http.StatusOK {
		defer hresp.Body.Close()
		return nil, errorx.RemoteSvcFail(errors.New("failed to run agent instance in agenthub"), rpcErrorCtx)
	}

	// Create a channel for streaming responses
	streamChan := make(chan types.AgentStreamEvent, 100)

	// Start a goroutine to handle the streaming response
	go func(body io.ReadCloser) {
		defer close(streamChan)
		defer body.Close()

		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // allow up to 1MB buffer
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				slog.Debug("stream cancelled", slog.String("session_id", contentID))
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var streamEvent types.AgentStreamEvent
			if err := json.Unmarshal([]byte(line), &streamEvent); err != nil {
				slog.Warn("could not unmarshal stream line into event, skipping. session_id: %s, error: %v, line: %s", slog.String("session_id", contentID), slog.Any("error", err), slog.String("line", line))
				continue
			}

			// Process the event based on its type
			switch streamEvent.Event {
			case "token":
				sendEvent(ctx, streamChan, streamEvent)
			case "add_message":
				sendEvent(ctx, streamChan, streamEvent)
			case "end":
				var endData EndData
				if err := json.Unmarshal(streamEvent.Data, &endData); err != nil {
					slog.Error("Error: could not unmarshal 'end' event data: %v", slog.String("session_id", contentID), slog.Any("error", err))
					continue
				}
				sendEvent(ctx, streamChan, streamEvent)

				// extract the message text from the event data, and send it as a separate event "output-message"
				if len(endData.Result.Outputs) > 0 && len(endData.Result.Outputs[0].Outputs) > 0 && endData.Result.Outputs[0].Outputs[0].Results != nil && endData.Result.Outputs[0].Outputs[0].Results.Message.Text != "" {
					sendEvent(ctx, streamChan, types.AgentStreamEvent{
						Event: "output-message",
						Data:  []byte(endData.Result.Outputs[0].Outputs[0].Results.Message.Text),
					})
				}
			default:
				slog.Warn("unknown event type", slog.String("session_id", contentID), slog.String("event", streamEvent.Event))
			}
		}

		if err := scanner.Err(); err != nil {
			slog.Error("scanner error", slog.String("session_id", contentID), slog.Any("error", err))
		}
	}(hresp.Body)

	return streamChan, nil
}

func sendEvent(ctx context.Context, ch chan<- types.AgentStreamEvent, msg types.AgentStreamEvent) {
	select {
	case ch <- msg:
	case <-ctx.Done():
		slog.Debug("stream channel closed", slog.Any("error", ctx.Err()))
	}
}
