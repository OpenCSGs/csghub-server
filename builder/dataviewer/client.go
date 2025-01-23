package dataviewer

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type DataviewerClient interface {
	TriggerWorkflow(ctx context.Context, req types.UpdateViewerReq) (*types.WorkFlowInfo, error)
}

type dataviewerClientImpl struct {
	hc *rpc.HttpClient
}

func NewDataviewerClient(config *config.Config, opts ...rpc.RequestOption) DataviewerClient {
	remoteURL := fmt.Sprintf("%s:%d", config.DataViewer.Host, config.DataViewer.Port)
	return &dataviewerClientImpl{
		hc: rpc.NewHttpClient(remoteURL, opts...),
	}
}

func (c *dataviewerClientImpl) TriggerWorkflow(ctx context.Context, req types.UpdateViewerReq) (*types.WorkFlowInfo, error) {
	url := fmt.Sprintf("/api/v1/%ss/%s/%s/callback/%s", req.RepoType, req.Namespace, req.Name, req.Branch)
	var r httpbase.R
	r.Data = &types.WorkFlowInfo{}
	err := c.hc.Post(ctx, url, nil, &r)
	if err != nil {
		return nil, fmt.Errorf("fail trigger workflow repo %s/%s branch %s", req.Namespace, req.Name, req.Branch)
	}

	return r.Data.(*types.WorkFlowInfo), nil
}
