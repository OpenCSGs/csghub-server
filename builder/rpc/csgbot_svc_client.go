package rpc

import (
	"context"
	"fmt"
	"net/http"

	"opencsg.com/csghub-server/common/errorx"
)

type CsgbotSvcClient interface {
	DeleteWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error
}

type CsgbotSvcHttpClientImpl struct {
	hc *HttpClient
}

func NewCsgbotSvcHttpClient(endpoint string, opts ...RequestOption) CsgbotSvcClient {
	return &CsgbotSvcHttpClientImpl{
		hc: NewHttpClient(endpoint, opts...),
	}
}

// Delete workspace files for a code agent
// DELETE /api/v1/csgbot/codeAgent/{agent_name}
func (c *CsgbotSvcHttpClientImpl) DeleteWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error {
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "csgbot",
		"api":       "DELETE /api/v1/csgbot/codeAgent/{agent_name}",
	}

	path := c.hc.endpoint + "/api/v1/csgbot/codeAgent/" + agentName
	hreq, err := http.NewRequestWithContext(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)
	hreq.Header.Set("user_name", username)
	hreq.Header.Set("user_token", token)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return errorx.RemoteSvcFail(fmt.Errorf("failed to delete workspace files for code agent: %w", err), rpcErrorCtx)
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		return errorx.RemoteSvcFail(fmt.Errorf("failed to delete workspace files for code agent: %s", hresp.Status), rpcErrorCtx)
	}

	return nil
}
