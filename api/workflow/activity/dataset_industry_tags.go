package activity

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/types"
)

func (a *Activities) ScanRepoIndustryTags(ctx context.Context, req types.ScanRepoIndustryTagsReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("repo industry scan start", slog.Any("req", req))
	return a.industryTag.RefreshRepoAutoIndustryTags(ctx, types.IdentifyIndustryTagsReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  req.RepoType,
		Branch:    req.Branch,
	})
}
