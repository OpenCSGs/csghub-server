package gitaly

import (
	"context"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
)

func (c *Client) GetRepoTags(ctx context.Context, req gitserver.GetRepoTagsReq) (tags []*types.Tag, err error) {
	return
}
