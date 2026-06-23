package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"opencsg.com/csghub-server/api/httpbase"
)

type MirrorSvcClient interface {
	CancelMirror(ctx context.Context, taskID int64) error
}

type MirrorSvcClientImpl struct {
	hc *HttpClient
}

func NewMirrorSvcClient(endpoint string, opts ...RequestOption) MirrorSvcClient {
	return &MirrorSvcClientImpl{
		hc: NewHttpClient(endpoint, opts...),
	}
}

func (c *MirrorSvcClientImpl) CancelMirror(ctx context.Context, taskID int64) error {
	type CancelReq struct {
		TaskID int64 `json:"task_id"`
	}
	req := CancelReq{
		TaskID: taskID,
	}

	path := "/api/v1/lfs_sync_internal/cancel"
	resp, err := c.hc.PostResponse(ctx, path, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		var r httpbase.R
		dErr := json.NewDecoder(resp.Body).Decode(&r)
		if dErr != nil {
			return dErr
		}
		return fmt.Errorf("cancel mirror failed, code: %d, msg: %s", resp.StatusCode, r.Msg)
	}

	return nil
}
