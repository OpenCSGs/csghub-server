package component

import (
	"context"

	dcommon "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *modelComponentImpl) CreateInferenceVersion(ctx context.Context, req types.CreateInferenceVersionReq) error {
	verReq := types.DeployActReq{
		CurrentUser: req.CurrentUser,
		DeployID:    req.DeployId,
	}
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, verReq)
	if err != nil {
		return err
	}

	if deploy.Status != dcommon.Running {
		return errorx.ErrDeployStatusNotMatchErr
	}

	if req.TrafficPercent > 100 || req.TrafficPercent < 0 {
		return errorx.ErrTrafficInvalid
	}

	if req.CommitID == "" {
		return errorx.ErrCommitIDEmpty
	}
	commitID, err := common.ShortenCommitID7(req.CommitID)
	if err != nil {
		return errorx.ErrInvalidCommitID
	}

	req.CommitID = commitID

	return c.imageRunner.CreateRevisions(ctx, &types.CreateRevisionReq{
		ClusterID:      deploy.ClusterID,
		SvcName:        deploy.SvcName,
		Commit:         req.CommitID,
		InitialTraffic: req.TrafficPercent,
	})
}

func (c *modelComponentImpl) ListInferenceVersions(ctx context.Context, verReq types.DeployActReq) ([]types.ListInferenceVersionsResp, error) {
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, verReq)
	if err != nil {
		return nil, err
	}

	var resp = []types.ListInferenceVersionsResp{}

	versions, err := c.imageRunner.ListKsvcVersions(ctx, deploy.ClusterID, deploy.SvcName)
	if err != nil {
		return nil, err
	}

	for _, version := range versions {
		resp = append(resp, types.ListInferenceVersionsResp{
			Commit:         version.Commit,
			CreateTime:     version.CreateTime,
			IsReady:        version.IsReady,
			TrafficPercent: version.TrafficPercent,
			RevisionName:   version.RevisionName,
			Message:        version.Message,
			Reason:         version.Reason,
		})
	}

	return resp, nil
}

func (c *modelComponentImpl) UpdateInferenceVersionTraffic(ctx context.Context, verReq types.DeployActReq, req []types.UpdateInferenceVersionTrafficReq) error {
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, verReq)
	if err != nil {
		return err
	}

	if deploy.Status != dcommon.Running {
		return errorx.ErrDeployStatusNotMatchErr
	}

	params := []types.TrafficReq{}
	for _, item := range req {
		params = append(params, types.TrafficReq{
			Commit:         item.CommitID,
			TrafficPercent: item.TrafficPercent,
		})
	}
	err = c.imageRunner.SetVersionsTraffic(ctx, deploy.ClusterID, deploy.SvcName, params)
	if err != nil {
		return err
	}

	return nil
}

func (c *modelComponentImpl) DeleteInferenceVersion(ctx context.Context, verReq types.DeployActReq, commitID string) error {
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, verReq)
	if err != nil {
		return err
	}

	if deploy.Status != dcommon.Running {
		return errorx.ErrDeployStatusNotMatchErr
	}

	shortCommitId, err := common.ShortenCommitID7(commitID)
	if err != nil {
		return errorx.ErrInvalidCommitID
	}

	return c.imageRunner.DeleteKsvcVersion(ctx, deploy.ClusterID, deploy.SvcName, shortCommitId)
}
